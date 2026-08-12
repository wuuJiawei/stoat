package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/action"
	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/diagnostics"
	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/exporter"
	"github.com/wuuJiawei/stoat/internal/model"
	"github.com/wuuJiawei/stoat/internal/monitor"
	"github.com/wuuJiawei/stoat/internal/risk"
	snapshotfile "github.com/wuuJiawei/stoat/internal/snapshot"
	"github.com/wuuJiawei/stoat/internal/tui"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "stoat:", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) > 0 {
		switch arguments[0] {
		case "help", "--help", "-h":
			printUsage(stdout)
			return nil
		case "version", "--version":
			fmt.Fprintln(stdout, version)
			return nil
		}
	}
	command := "tui"
	if len(arguments) > 0 {
		command = arguments[0]
		arguments = arguments[1:]
	}
	if !knownCommand(command) {
		return fmt.Errorf("unknown command %q", command)
	}
	options, err := parseOptions(command, arguments, stderr)
	if err != nil {
		return err
	}
	if command == "diff" {
		return runDiff(options, stdout)
	}
	if runtime.GOOS != "darwin" {
		return errors.New("requires macOS 13 or later")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	if options.dataDir == "" {
		options.dataDir = filepath.Join(home, "Library", "Application Support", "Stoat")
	}
	policy := risk.Policy{}
	if options.rulesPath != "" {
		policy, err = risk.LoadPolicy(options.rulesPath, time.Now())
		if err != nil {
			return fmt.Errorf("load risk policy: %w", err)
		}
		if policy.ExpiredExceptions > 0 {
			fmt.Fprintf(stderr, "warning: ignored %d expired risk policy exception(s)\n", policy.ExpiredExceptions)
		}
	}
	if command == "restore" || command == "audit" {
		return runStoredAction(command, home, options, stdout)
	}
	if command == "changes" {
		return runChanges(options, stdout)
	}
	if command == "watch" {
		return runWatch(home, options, policy, stdout, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	report := app.NewDefaultScannerWithPolicy(home, options.includeSystem, policy).Scan(ctx)
	report.ToolVersion = version
	report.Items = filterItems(report.Items, options.category, options.risk, options.minimumRisk)

	switch command {
	case "tui":
		program := tea.NewProgram(tui.New(report.Items, report.Warnings), tea.WithAltScreen())
		_, err = program.Run()
		return err
	case "scan", "startup", "scheduled", "background", "suspicious":
		if options.jsonOutput {
			encoder := json.NewEncoder(stdout)
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		}
		printItems(stdout, report.Items)
		for _, warning := range report.Warnings {
			fmt.Fprintln(stderr, "warning:", warning.Error())
		}
		return nil
	case "export":
		format, formatErr := exporter.ParseFormat(options.format)
		if formatErr != nil {
			return formatErr
		}
		write := func(writer io.Writer) error { return exporter.Write(writer, format, report) }
		if options.output == "-" {
			return write(stdout)
		}
		return exporter.WriteFile(options.output, options.force, write)
	case "snapshot":
		if options.output == "" || options.output == "-" {
			return errors.New("snapshot requires --output <file>")
		}
		return snapshotfile.WriteFile(options.output, options.force, snapshotfile.New(report))
	case "inspect":
		if options.query == "" {
			return errors.New("inspect requires an ID or label")
		}
		for _, item := range report.Items {
			if item.ID == options.query || item.Label == options.query {
				encoder := json.NewEncoder(stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(item)
			}
		}
		return fmt.Errorf("item not found: %s", options.query)
	case "disable", "quarantine":
		return runMutation(ctx, command, home, options, report.Items, stdout)
	case "diagnose":
		return runDiagnose(ctx, options, report.Items, stdout)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

type cliOptions struct {
	includeSystem bool
	jsonOutput    bool
	risk          model.RiskLevel
	minimumRisk   bool
	category      model.Category
	query         string
	format        string
	output        string
	force         bool
	rulesPath     string
	beforePath    string
	afterPath     string
	confirm       string
	dataDir       string
	statePath     string
	interval      time.Duration
	once          bool
	logPeriod     time.Duration
	logLimit      int
	eventLimit    int
}

func parseOptions(command string, arguments []string, stderr io.Writer) (cliOptions, error) {
	options := cliOptions{}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	if acceptsIdentifier(command) && len(arguments) > 1 && !strings.HasPrefix(arguments[0], "-") {
		arguments = append(append([]string(nil), arguments[1:]...), arguments[0])
	}
	flags.BoolVar(&options.jsonOutput, "json", false, "write JSON")
	var riskValue *string
	if command != "diff" && command != "audit" && command != "restore" && command != "changes" {
		flags.BoolVar(&options.includeSystem, "system", false, "include Apple system launchd jobs")
		flags.StringVar(&options.rulesPath, "rules", "", "risk exception policy JSON")
		riskValue = flags.String("risk", "", "filter by risk: trusted, normal, attention, high")
	}
	if command == "export" || command == "snapshot" {
		defaultOutput := "-"
		if command == "snapshot" {
			defaultOutput = ""
		}
		flags.StringVar(&options.output, "output", defaultOutput, "output file or - for stdout")
		flags.BoolVar(&options.force, "force", false, "replace an existing regular file")
	}
	if command == "export" {
		flags.StringVar(&options.format, "format", "json", "export format: json or csv")
	}
	if command == "disable" || command == "quarantine" || command == "restore" {
		flags.StringVar(&options.confirm, "confirm", "", "confirmation token from the current plan")
	}
	if command == "disable" || command == "quarantine" || command == "restore" || command == "audit" || command == "watch" || command == "changes" {
		flags.StringVar(&options.dataDir, "data-dir", "", "Stoat private state directory")
	}
	if command == "watch" || command == "changes" {
		flags.StringVar(&options.statePath, "state", "", "monitor snapshot path")
	}
	if command == "watch" {
		flags.DurationVar(&options.interval, "interval", 30*time.Second, "polling interval (5s to 24h)")
		flags.BoolVar(&options.once, "once", false, "scan once and exit")
	}
	if command == "changes" {
		flags.IntVar(&options.eventLimit, "limit", 50, "maximum stored change events (1 to 1000)")
	}
	if command == "diagnose" {
		flags.DurationVar(&options.logPeriod, "last", time.Hour, "unified log lookback (1m to 24h)")
		flags.IntVar(&options.logLimit, "limit", 100, "maximum recent log entries (1 to 500)")
	}
	if err := flags.Parse(arguments); err != nil {
		return options, err
	}
	if command == "diff" {
		if flags.NArg() != 2 {
			return options, errors.New("diff requires <before> and <after> snapshot files")
		}
		options.beforePath, options.afterPath = flags.Arg(0), flags.Arg(1)
		return options, nil
	}
	positional := acceptsIdentifier(command)
	if !positional && flags.NArg() != 0 {
		return options, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if positional && flags.NArg() > 1 {
		return options, fmt.Errorf("%s accepts at most one identifier", command)
	}
	if riskValue != nil && *riskValue != "" {
		options.risk = model.RiskLevel(*riskValue)
		if !validRisk(options.risk) {
			return options, fmt.Errorf("invalid risk level %q", *riskValue)
		}
	}
	switch command {
	case "startup":
		options.category = model.CategoryStartup
	case "scheduled":
		options.category = model.CategoryScheduled
	case "background":
		options.category = model.CategoryBackground
	case "suspicious":
		options.risk = model.RiskAttention
		options.minimumRisk = true
	case "inspect", "disable", "quarantine", "restore", "audit", "diagnose":
		if flags.NArg() > 0 {
			options.query = flags.Arg(0)
		}
	}
	if (command == "inspect" || command == "disable" || command == "quarantine" || command == "restore" || command == "diagnose") && options.query == "" {
		return options, fmt.Errorf("%s requires an identifier", command)
	}
	if command == "watch" && (options.interval < 5*time.Second || options.interval > 24*time.Hour) {
		return options, errors.New("watch interval must be between 5s and 24h")
	}
	if command == "diagnose" && (options.logPeriod < time.Minute || options.logPeriod > 24*time.Hour || options.logLimit < 1 || options.logLimit > 500) {
		return options, errors.New("diagnose requires --last 1m–24h and --limit 1–500")
	}
	if command == "changes" && (options.eventLimit < 1 || options.eventLimit > 1000) {
		return options, errors.New("changes limit must be between 1 and 1000")
	}
	return options, nil
}

func acceptsIdentifier(command string) bool {
	switch command {
	case "inspect", "disable", "quarantine", "restore", "audit", "diagnose":
		return true
	default:
		return false
	}
}

func runMutation(ctx context.Context, command, home string, options cliOptions, items []model.PersistenceItem, stdout io.Writer) error {
	item, err := findItem(items, options.query)
	if err != nil {
		return err
	}
	runner := executil.NewExecRunner(10*time.Second, 8<<20)
	manager := action.NewManager(home, options.dataDir, runner, strconv.Itoa(os.Getuid()))
	plan, err := manager.Plan(action.Kind(command), item)
	if err != nil {
		return err
	}
	if options.confirm == "" {
		return writeJSON(stdout, plan)
	}
	operation, actionErr := manager.Apply(ctx, plan, options.confirm)
	if err := writeJSON(stdout, operation); err != nil && actionErr == nil {
		return err
	}
	return actionErr
}

func runStoredAction(command, home string, options cliOptions, stdout io.Writer) error {
	runner := executil.NewExecRunner(10*time.Second, 8<<20)
	manager := action.NewManager(home, options.dataDir, runner, strconv.Itoa(os.Getuid()))
	if command == "audit" {
		if options.query != "" {
			operation, err := manager.Read(options.query)
			if err != nil {
				return err
			}
			return writeJSON(stdout, operation)
		}
		operations, err := manager.List()
		if err != nil {
			return err
		}
		table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(table, "ID\tSTATUS\tACTION\tLABEL\tCREATED")
		for _, operation := range operations {
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", operation.ID, operation.Status, operation.Plan.Action, terminalText(operation.Plan.Label), operation.CreatedAt.Format(time.RFC3339))
		}
		return table.Flush()
	}
	plan, err := manager.PlanRestore(options.query)
	if err != nil {
		return err
	}
	if options.confirm == "" {
		return writeJSON(stdout, plan)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	operation, restoreErr := manager.Restore(ctx, plan, options.confirm)
	if err := writeJSON(stdout, operation); err != nil && restoreErr == nil {
		return err
	}
	return restoreErr
}

func runWatch(home string, options cliOptions, policy risk.Policy, stdout, stderr io.Writer) error {
	if options.statePath == "" {
		options.statePath = filepath.Join(options.dataDir, "monitor", "snapshot.json")
	}
	if !filepath.IsAbs(options.statePath) {
		return errors.New("watch state path must be absolute")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	for {
		scanCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		report := app.NewDefaultScannerWithPolicy(home, options.includeSystem, policy).Scan(scanCtx)
		cancel()
		report.ToolVersion = version
		if ctx.Err() != nil {
			return nil
		}
		if len(report.Warnings) > 0 {
			for _, warning := range report.Warnings {
				fmt.Fprintln(stderr, "warning:", warning.Error())
			}
			if options.once {
				return errors.New("scan was incomplete; monitor state was not changed")
			}
			if waitForNextScan(ctx, options.interval) {
				return nil
			}
			continue
		}
		observation, err := monitor.Observe(options.statePath, report)
		if err != nil {
			return err
		}
		if observation.Initialized || len(observation.Diff.Changes) > 0 {
			if options.jsonOutput {
				if err := writeJSON(stdout, observation); err != nil {
					return err
				}
			} else if observation.Initialized {
				fmt.Fprintln(stdout, "monitor baseline initialized")
			} else {
				fmt.Fprintf(stdout, "%s: %d persistence change(s)\n", observation.ObservedAt.Format(time.RFC3339), len(observation.Diff.Changes))
				if err := printDiff(stdout, observation.Diff); err != nil {
					return err
				}
			}
		}
		if options.once {
			return nil
		}
		if waitForNextScan(ctx, options.interval) {
			return nil
		}
	}
}

func runChanges(options cliOptions, stdout io.Writer) error {
	if options.statePath == "" {
		options.statePath = filepath.Join(options.dataDir, "monitor", "snapshot.json")
	}
	events, err := monitor.List(options.statePath, options.eventLimit)
	if err != nil {
		return err
	}
	if options.jsonOutput {
		return writeJSON(stdout, events)
	}
	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "TIME\tCHANGE\tTYPE\tLABEL\tEVENT")
	for _, event := range events {
		for _, change := range event.Diff.Changes {
			itemType := ""
			if change.After != nil {
				itemType = string(change.After.Type)
			} else if change.Before != nil {
				itemType = string(change.Before.Type)
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", event.ObservedAt.Format(time.RFC3339), change.Type, itemType, terminalText(change.Label), event.ID)
		}
	}
	return table.Flush()
}

func waitForNextScan(ctx context.Context, interval time.Duration) bool {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

func runDiagnose(_ context.Context, options cliOptions, items []model.PersistenceItem, stdout io.Writer) error {
	item, err := findItem(items, options.query)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	report := diagnostics.NewInspector(executil.NewExecRunner(20*time.Second, 8<<20)).Inspect(ctx, item, options.logPeriod, options.logLimit)
	if options.jsonOutput {
		return writeJSON(stdout, report)
	}
	fmt.Fprintf(stdout, "Label: %s\nExecutable: %s\nRisk: %s (%d)\n", terminalText(report.Label), terminalText(report.Program), report.RiskLevel, report.RiskScore)
	for _, issue := range report.Issues {
		fmt.Fprintln(stdout, "Issue:", terminalText(issue))
	}
	if report.LogWarning != "" {
		fmt.Fprintln(stdout, "Warning:", terminalText(report.LogWarning))
	}
	for _, entry := range report.RecentLogs {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", terminalText(entry.Timestamp), terminalText(entry.MessageType), terminalText(entry.Message))
	}
	return nil
}

func findItem(items []model.PersistenceItem, query string) (model.PersistenceItem, error) {
	for _, item := range items {
		if item.ID == query {
			return item, nil
		}
	}
	var match *model.PersistenceItem
	for index := range items {
		if items[index].Label != query {
			continue
		}
		if match != nil {
			return model.PersistenceItem{}, fmt.Errorf("label %q is ambiguous; use the item id", query)
		}
		match = &items[index]
	}
	if match == nil {
		return model.PersistenceItem{}, fmt.Errorf("item not found: %s", query)
	}
	return *match, nil
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func runDiff(options cliOptions, stdout io.Writer) error {
	before, err := snapshotfile.ReadFile(options.beforePath)
	if err != nil {
		return fmt.Errorf("read before snapshot: %w", err)
	}
	after, err := snapshotfile.ReadFile(options.afterPath)
	if err != nil {
		return fmt.Errorf("read after snapshot: %w", err)
	}
	diff := snapshotfile.Compare(before, after)
	if options.jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(diff)
	}
	return printDiff(stdout, diff)
}

func printDiff(stdout io.Writer, diff snapshotfile.Diff) error {
	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "CHANGE\tTYPE\tLABEL\tFIELDS")
	for _, change := range diff.Changes {
		itemType := ""
		if change.After != nil {
			itemType = string(change.After.Type)
		} else if change.Before != nil {
			itemType = string(change.Before.Type)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", strings.ToUpper(string(change.Type)), itemType, terminalText(change.Label), strings.Join(change.Fields, ", "))
	}
	return table.Flush()
}

func validRisk(level model.RiskLevel) bool {
	return level == model.RiskTrusted || level == model.RiskNormal || level == model.RiskAttention || level == model.RiskHigh
}

func knownCommand(command string) bool {
	switch command {
	case "tui", "scan", "startup", "scheduled", "background", "suspicious", "inspect", "export", "snapshot", "diff", "disable", "quarantine", "restore", "audit", "watch", "changes", "diagnose":
		return true
	default:
		return false
	}
}

func filterItems(items []model.PersistenceItem, category model.Category, level model.RiskLevel, minimumRisk bool) []model.PersistenceItem {
	filtered := make([]model.PersistenceItem, 0, len(items))
	for _, item := range items {
		if category != "" && !item.HasCategory(category) {
			continue
		}
		if level != "" {
			if minimumRisk && riskRank(item.RiskLevel) < riskRank(level) {
				continue
			}
			if !minimumRisk && item.RiskLevel != level {
				continue
			}
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func riskRank(level model.RiskLevel) int {
	switch level {
	case model.RiskHigh:
		return 4
	case model.RiskAttention:
		return 3
	case model.RiskNormal:
		return 2
	case model.RiskTrusted:
		return 1
	default:
		return 0
	}
}

func printItems(writer io.Writer, items []model.PersistenceItem) {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "RISK\tSCORE\tTYPE\tLABEL\tEXECUTABLE")
	for _, item := range items {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\n", strings.ToUpper(string(item.RiskLevel)), item.RiskScore, item.Type, terminalText(item.Label), terminalText(item.Program))
	}
	_ = table.Flush()
}

func terminalText(value string) string {
	return strings.Map(func(character rune) rune {
		if character == '\n' || character == '\r' || character == '\t' || character == 0x1b || character < 0x20 || character == 0x7f {
			return ' '
		}
		return character
	}, value)
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, `Stoat inspects macOS login, scheduled, and background persistence.

Usage:
  stoat                         open the read-only TUI
  stoat scan [--json] [--risk LEVEL] [--rules policy.json] [--system]
  stoat startup|scheduled|background|suspicious [--json]
  stoat inspect <id-or-label>
  stoat export --format json|csv --output <file> [--force]
  stoat snapshot --output <file> [--rules policy.json]
  stoat diff [--json] <before> <after>
  stoat disable|quarantine <id-or-label> [--confirm TOKEN]
  stoat restore <operation-id> [--confirm TOKEN]
  stoat audit [operation-id]
  stoat watch [--once] [--interval 30s] [--json]
  stoat changes [--limit 50] [--json]
  stoat diagnose <id-or-label> [--last 1h] [--limit 100] [--json]
  stoat version

State-changing commands only support launchd jobs. Running without --confirm prints a
plan and confirmation token; the token is valid only for the unchanged configuration.

--system includes Apple-owned /System/Library launchd jobs.`)
}
