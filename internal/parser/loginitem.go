package parser

import (
	"bufio"
	"net/url"
	"strings"

	"github.com/wuuJiawei/stoat/internal/model"
)

// ParseBTMDump parses the stable key/value portion of `sfltool dumpbtm` output.
// Unknown fields are intentionally ignored so new macOS fields remain forward-compatible.
func ParseBTMDump(data []byte) []model.PersistenceItem {
	var items []model.PersistenceItem
	current := make(map[string]string)
	flush := func() {
		identifier := firstNonEmpty(current["identifier"], current["bundle identifier"])
		name := current["name"]
		rawURL := firstNonEmpty(current["url"], current["path"])
		if identifier == "" && name == "" && rawURL == "" {
			current = make(map[string]string)
			return
		}
		path := fileURLPath(rawURL)
		label := firstNonEmpty(name, identifier, path)
		typeText := strings.ToLower(current["type"])
		itemType := model.TypeLoginItem
		categories := []model.Category{model.CategoryStartup}
		if strings.Contains(typeText, "background") || strings.Contains(typeText, "agent") || strings.Contains(typeText, "daemon") {
			categories = model.AddCategory(categories, model.CategoryBackground)
		}
		item := model.PersistenceItem{
			ID:         model.StableID(model.SourceBTM, rawURL, identifier+label),
			Label:      label,
			Type:       itemType,
			Scope:      model.ScopeUser,
			Source:     model.SourceBTM,
			Categories: categories,
			Program:    path,
			AppPath:    path,
			BundleID:   identifier,
			ConfigPath: rawURL,
		}
		items = append(items, item)
		current = make(map[string]string)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			flush()
			continue
		}
		separator := strings.IndexByte(line, ':')
		if separator <= 0 {
			if strings.HasPrefix(line, "#") && len(current) > 0 {
				flush()
			}
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:separator]))
		value := strings.TrimSpace(line[separator+1:])
		if key == "uuid" && len(current) > 0 && current["uuid"] != "" {
			flush()
		}
		current[key] = value
	}
	flush()
	return items
}

func fileURLPath(value string) string {
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme == "file" {
		return parsed.Path
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
