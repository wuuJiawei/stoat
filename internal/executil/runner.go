package executil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

var ErrCommandNotAllowed = errors.New("command is not allowlisted")

type Result struct {
	Output    []byte
	Truncated bool
}

type Runner interface {
	Run(ctx context.Context, command string, args ...string) (Result, error)
}

type ExecRunner struct {
	commands  map[string]string
	timeout   time.Duration
	maxOutput int
}

func NewExecRunner(timeout time.Duration, maxOutput int) *ExecRunner {
	return &ExecRunner{
		commands: map[string]string{
			"codesign":  "/usr/bin/codesign",
			"crontab":   "/usr/bin/crontab",
			"launchctl": "/bin/launchctl",
			"plutil":    "/usr/bin/plutil",
			"sfltool":   "/usr/bin/sfltool",
			"spctl":     "/usr/sbin/spctl",
		},
		timeout:   timeout,
		maxOutput: maxOutput,
	}
}

func (r *ExecRunner) Run(ctx context.Context, command string, args ...string) (Result, error) {
	path, ok := r.commands[command]
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrCommandNotAllowed, command)
	}

	commandCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	buffer := newLimitedBuffer(r.maxOutput)
	cmd := exec.CommandContext(commandCtx, path, args...)
	cmd.Stdout = buffer
	cmd.Stderr = buffer
	err := cmd.Run()
	result := Result{Output: buffer.Bytes(), Truncated: buffer.Truncated()}
	if commandCtx.Err() != nil {
		return result, fmt.Errorf("%s timed out: %w", command, commandCtx.Err())
	}
	if err != nil {
		return result, fmt.Errorf("%s failed: %w", command, err)
	}
	return result, nil
}

type limitedBuffer struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	remaining int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	if limit <= 0 {
		limit = 1 << 20
	}
	return &limitedBuffer{remaining: limit}
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	originalLength := len(data)
	if len(data) > b.remaining {
		data = data[:b.remaining]
		b.truncated = true
	}
	if len(data) > 0 {
		_, _ = b.buffer.Write(data)
		b.remaining -= len(data)
	}
	return originalLength, nil
}

func (b *limitedBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return bytes.Clone(b.buffer.Bytes())
}

func (b *limitedBuffer) Truncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.truncated
}
