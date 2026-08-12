package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/exporter"
	"github.com/wuuJiawei/stoat/internal/model"
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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
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
}

func parseOptions(command string, arguments []string, stderr io.Writer) (cliOptions, error) {
	options := cliOptions{}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&options.jsonOutput, "json", false, "write JSON")
	var riskValue *string
	if command != "diff" {
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
	if command != "inspect" && flags.NArg() != 0 {
		return options, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if command == "inspect" && flags.NArg() > 1 {
		return options, errors.New("inspect accepts exactly one ID or label")
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
	case "inspect":
		if flags.NArg() > 0 {
			options.query = flags.Arg(0)
		}
	}
	return options, nil
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
	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "CHANGE\tTYPE\tLABEL\tFIELDS")
	for _, change := range diff.Changes {
		itemType := ""
		if change.After != nil {
			itemType = string(change.After.Type)
		} else if change.Before != nil {
			itemType = string(change.Before.Type)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", strings.ToUpper(string(change.Type)), itemType, change.Label, strings.Join(change.Fields, ", "))
	}
	return table.Flush()
}

func validRisk(level model.RiskLevel) bool {
	return level == model.RiskTrusted || level == model.RiskNormal || level == model.RiskAttention || level == model.RiskHigh
}

func knownCommand(command string) bool {
	switch command {
	case "tui", "scan", "startup", "scheduled", "background", "suspicious", "inspect", "export", "snapshot", "diff":
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
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\n", strings.ToUpper(string(item.RiskLevel)), item.RiskScore, item.Type, item.Label, item.Program)
	}
	_ = table.Flush()
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
  stoat version

--system includes Apple-owned /System/Library launchd jobs.`)
}
