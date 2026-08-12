package exporter

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wuuJiawei/stoat/internal/app"
)

type Format string

const (
	FormatJSON Format = "json"
	FormatCSV  Format = "csv"
)

var ErrPathExists = errors.New("export path already exists; use --force to replace it")

func ParseFormat(value string) (Format, error) {
	format := Format(strings.ToLower(value))
	if format != FormatJSON && format != FormatCSV {
		return "", fmt.Errorf("unsupported export format %q", value)
	}
	return format, nil
}

func Write(writer io.Writer, format Format, report app.Report) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	case FormatCSV:
		return writeCSV(writer, report)
	default:
		return fmt.Errorf("unsupported export format %q", format)
	}
}

func WriteFile(path string, force bool, write func(io.Writer) error) error {
	if path == "" || path == "-" {
		return errors.New("export path must be a file")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("refusing to replace non-regular export path")
		}
		if !force {
			return ErrPathExists
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect export path: %w", err)
	}

	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".stoat-export-*")
	if err != nil {
		return fmt.Errorf("create temporary export: %w", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("protect temporary export: %w", err)
	}
	if err := write(temporary); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync export: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close export: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit export: %w", err)
	}
	committed = true
	return nil
}

func writeCSV(writer io.Writer, report app.Report) error {
	output := csv.NewWriter(writer)
	header := []string{"id", "risk_level", "risk_score", "type", "scope", "label", "executable", "app", "bundle_id", "loaded", "running", "pid", "source"}
	if err := output.Write(header); err != nil {
		return err
	}
	for _, item := range report.Items {
		row := []string{
			item.ID,
			string(item.RiskLevel),
			strconv.Itoa(item.RiskScore),
			string(item.Type),
			string(item.Scope),
			safeCell(item.Label),
			safeCell(item.Program),
			safeCell(item.Attribution.Name),
			safeCell(item.BundleID),
			strconv.FormatBool(item.Runtime.Loaded),
			strconv.FormatBool(item.Runtime.Running),
			strconv.Itoa(item.Runtime.PID),
			safeCell(item.ConfigPath),
		}
		if err := output.Write(row); err != nil {
			return err
		}
	}
	output.Flush()
	return output.Error()
}

func safeCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
