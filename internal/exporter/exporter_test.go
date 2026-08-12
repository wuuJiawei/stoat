package exporter

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wuuJiawei/stoat/internal/app"
	"github.com/wuuJiawei/stoat/internal/model"
)

func TestCSVPreventsFormulaExecution(t *testing.T) {
	report := app.Report{Items: []model.PersistenceItem{{ID: "1", Label: "=CMD()", Program: "/bin/test"}}}
	var output bytes.Buffer
	if err := Write(&output, FormatCSV, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "'=CMD()") {
		t.Fatalf("formula-like cell was not escaped: %q", output.String())
	}
}

func TestWriteFileRefusesOverwriteWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteFile(path, false, func(writer io.Writer) error {
		_, writeErr := writer.Write([]byte("replacement"))
		return writeErr
	})
	if err == nil {
		t.Fatal("expected overwrite refusal")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "original" {
		t.Fatalf("existing file changed: %q, %v", data, readErr)
	}
}
