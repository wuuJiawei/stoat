package executil

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRejectsCommandsOutsideAllowlist(t *testing.T) {
	runner := NewExecRunner(time.Second, 1024)
	_, err := runner.Run(context.Background(), "sh", "-c", "echo unsafe")
	if !errors.Is(err, ErrCommandNotAllowed) {
		t.Fatalf("expected allowlist error, got %v", err)
	}
}

func TestLimitedBuffer(t *testing.T) {
	buffer := newLimitedBuffer(4)
	written, err := buffer.Write([]byte("123456"))
	if err != nil || written != 6 {
		t.Fatalf("unexpected write: %d, %v", written, err)
	}
	if string(buffer.Bytes()) != "1234" || !buffer.Truncated() {
		t.Fatalf("unexpected buffer: %q truncated=%v", buffer.Bytes(), buffer.Truncated())
	}
}
