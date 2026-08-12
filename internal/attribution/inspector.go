package attribution

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wuuJiawei/stoat/internal/executil"
	"github.com/wuuJiawei/stoat/internal/model"
)

const maxInfoPlistSize = 2 << 20

type Inspector struct {
	runner executil.Runner
}

type bundleDocument struct {
	Identifier  string `json:"CFBundleIdentifier"`
	DisplayName string `json:"CFBundleDisplayName"`
	Name        string `json:"CFBundleName"`
}

func NewInspector(runner executil.Runner) *Inspector { return &Inspector{runner: runner} }

func (i *Inspector) Name() string { return "app-attribution" }

func (i *Inspector) Enrich(ctx context.Context, item *model.PersistenceItem) error {
	item.Attribution.Checked = true
	if item.BundleID != "" {
		item.Attribution.BundleID = item.BundleID
		item.Attribution.Evidence = append(item.Attribution.Evidence, "source supplied bundle identifier")
	}

	appPath := item.AppPath
	if appPath == "" {
		appPath = AppBundleForPath(item.Program)
	}
	if appPath == "" {
		if item.Attribution.BundleID != "" {
			item.Attribution.Confidence = "medium"
		}
		return nil
	}
	item.AppPath = appPath
	item.Attribution.AppPath = appPath
	item.Attribution.Evidence = append(item.Attribution.Evidence, "executable path is inside app bundle")

	infoPath := filepath.Join(appPath, "Contents", "Info.plist")
	info, err := os.Lstat(infoPath)
	if err != nil {
		if os.IsNotExist(err) && item.Attribution.BundleID != "" {
			item.Attribution.Confidence = "medium"
			return nil
		}
		return fmt.Errorf("inspect app bundle metadata: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing non-regular Info.plist")
	}
	if info.Size() > maxInfoPlistSize {
		return fmt.Errorf("Info.plist exceeds %d bytes", maxInfoPlistSize)
	}

	converted, err := i.runner.Run(ctx, "plutil", "-convert", "json", "-o", "-", "--", infoPath)
	if err != nil {
		return fmt.Errorf("read app bundle metadata: %w", err)
	}
	var document bundleDocument
	if err := json.Unmarshal(converted.Output, &document); err != nil {
		return fmt.Errorf("decode app bundle metadata: %w", err)
	}
	item.Attribution.Name = firstNonEmpty(document.DisplayName, document.Name)
	if document.Identifier != "" {
		if item.Attribution.BundleID != "" && item.Attribution.BundleID != document.Identifier {
			item.Attribution.Evidence = append(item.Attribution.Evidence, "source and Info.plist bundle identifiers differ")
			item.Attribution.Confidence = "low"
			return nil
		}
		item.BundleID = document.Identifier
		item.Attribution.BundleID = document.Identifier
		item.Attribution.Evidence = append(item.Attribution.Evidence, "bundle identifier verified from Info.plist")
		item.Attribution.Confidence = "high"
		return nil
	}
	item.Attribution.Confidence = "medium"
	return nil
}

func AppBundleForPath(path string) string {
	if !filepath.IsAbs(path) {
		return ""
	}
	clean := filepath.Clean(path)
	parts := strings.Split(strings.TrimPrefix(clean, string(filepath.Separator)), string(filepath.Separator))
	lastApp := -1
	for index, part := range parts {
		if strings.HasSuffix(strings.ToLower(part), ".app") {
			lastApp = index
		}
	}
	if lastApp < 0 {
		return ""
	}
	return string(filepath.Separator) + filepath.Join(parts[:lastApp+1]...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
