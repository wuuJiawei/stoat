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
	if runtime.GOOS != "darwin" {
		return errors.New("requires macOS 13 or later")
	}

	command := "tui"
	if len(arguments) > 0 {
		command = arguments[0]
		arguments = arguments[1:]
	}
	options, err := parseOptions(command, arguments, stderr)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	report := app.NewDefaultScanner(home, options.includeSystem).Scan(ctx)
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
}

func parseOptions(command string, arguments []string, stderr io.Writer) (cliOptions, error) {
	options := cliOptions{}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&options.includeSystem, "system", false, "include Apple system launchd jobs")
	flags.BoolVar(&options.jsonOutput, "json", false, "write JSON")
	if command == "export" {
		flags.StringVar(&options.format, "format", "json", "export format: json or csv")
		flags.StringVar(&options.output, "output", "-", "output file or - for stdout")
		flags.BoolVar(&options.force, "force", false, "replace an existing regular file")
	}
	riskValue := flags.String("risk", "", "filter by risk: trusted, normal, attention, high")
	if err := flags.Parse(arguments); err != nil {
		return options, err
	}
	if command != "inspect" && flags.NArg() != 0 {
		return options, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	if command == "inspect" && flags.NArg() > 1 {
		return options, errors.New("inspect accepts exactly one ID or label")
	}
	if *riskValue != "" {
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

func validRisk(level model.RiskLevel) bool {
	return level == model.RiskTrusted || level == model.RiskNormal || level == model.RiskAttention || level == model.RiskHigh
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
  stoat scan [--json] [--risk LEVEL] [--system]
  stoat startup|scheduled|background|suspicious [--json]
  stoat inspect <id-or-label>
	stoat export --format json|csv --output <file> [--force]
  stoat version

--system includes Apple-owned /System/Library launchd jobs.`)
}
