package signing

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

type Inspector struct {
	runner executil.Runner
}

func NewInspector(runner executil.Runner) *Inspector { return &Inspector{runner: runner} }

func (i *Inspector) Name() string { return "file-signature" }

func (i *Inspector) Enrich(ctx context.Context, item *model.PersistenceItem) error {
	if item.Program == "" || !strings.HasPrefix(item.Program, "/") {
		return nil
	}
	info, err := os.Stat(item.Program)
	if err != nil {
		if os.IsNotExist(err) {
			item.Exists = false
			return nil
		}
		return fmt.Errorf("stat executable: %w", err)
	}
	item.Exists = true
	item.Mode = info.Mode().Perm().String()
	item.WritableByOthers = info.Mode().Perm()&0o002 != 0
	item.Owner = fileOwner(info)

	if runtime.GOOS != "darwin" {
		return nil
	}
	item.Signature.Checked = true
	result, signErr := i.runner.Run(ctx, "codesign", "-dv", "--verbose=4", "--", item.Program)
	parseCodeSign(result.Output, &item.Signature)
	if signErr != nil {
		item.Signature.Signed = false
		return nil
	}
	item.Signature.Signed = true
	return nil
}

func fileOwner(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	uid := strconv.FormatUint(uint64(stat.Uid), 10)
	owner, err := user.LookupId(uid)
	if err != nil {
		return uid
	}
	return owner.Username
}

func parseCodeSign(output []byte, signature *model.SignatureInfo) {
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "Identifier":
			signature.Identifier = value
		case "TeamIdentifier":
			signature.TeamID = value
		case "Authority":
			if signature.Signer == "" {
				signature.Signer = value
			}
			if strings.Contains(value, "Apple") {
				signature.AppleSigned = true
			}
		}
	}
}
