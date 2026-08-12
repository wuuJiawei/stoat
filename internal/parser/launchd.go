package parser

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wuuJiawei/stoat/internal/model"
)

type launchdDocument struct {
	Label                 string          `json:"Label"`
	Program               string          `json:"Program"`
	ProgramArguments      []string        `json:"ProgramArguments"`
	WorkingDirectory      string          `json:"WorkingDirectory"`
	RunAtLoad             bool            `json:"RunAtLoad"`
	KeepAlive             json.RawMessage `json:"KeepAlive"`
	StartInterval         int             `json:"StartInterval"`
	StartCalendarInterval json.RawMessage `json:"StartCalendarInterval"`
	WatchPaths            []string        `json:"WatchPaths"`
	QueueDirectories      []string        `json:"QueueDirectories"`
	Sockets               json.RawMessage `json:"Sockets"`
	MachServices          json.RawMessage `json:"MachServices"`
	UserName              string          `json:"UserName"`
}

func ParseLaunchdJSON(data []byte, configPath string, itemType model.ItemType, scope model.Scope) (model.PersistenceItem, error) {
	var document launchdDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return model.PersistenceItem{}, fmt.Errorf("decode launchd JSON: %w", err)
	}
	if document.Label == "" {
		document.Label = strings.TrimSuffix(filepath.Base(configPath), filepath.Ext(configPath))
	}
	program := document.Program
	if program == "" && len(document.ProgramArguments) > 0 {
		program = document.ProgramArguments[0]
	}
	item := model.PersistenceItem{
		ID:         model.StableID(model.SourceLaunchd, configPath, document.Label),
		Label:      document.Label,
		Type:       itemType,
		Scope:      scope,
		Source:     model.SourceLaunchd,
		Program:    program,
		Arguments:  append([]string(nil), document.ProgramArguments...),
		WorkingDir: document.WorkingDirectory,
		RunAtLoad:  document.RunAtLoad,
		KeepAlive:  enabledValue(document.KeepAlive),
		WatchPaths: append([]string(nil), document.WatchPaths...),
		QueueDirs:  append([]string(nil), document.QueueDirectories...),
		ConfigPath: configPath,
		User:       document.UserName,
	}
	if item.User == "" && scope == model.ScopeSystem {
		item.User = "root"
	}
	if item.RunAtLoad {
		item.Categories = model.AddCategory(item.Categories, model.CategoryStartup)
	}
	if document.StartInterval > 0 {
		item.Categories = model.AddCategory(item.Categories, model.CategoryScheduled)
		item.Schedules = append(item.Schedules, model.Schedule{
			Kind: "interval", Expression: fmt.Sprintf("%ds", document.StartInterval),
			Description: describeInterval(document.StartInterval),
		})
	}
	calendarRules, err := parseCalendarRules(document.StartCalendarInterval)
	if err != nil {
		return model.PersistenceItem{}, fmt.Errorf("parse StartCalendarInterval: %w", err)
	}
	for _, rule := range calendarRules {
		item.Categories = model.AddCategory(item.Categories, model.CategoryScheduled)
		item.Schedules = append(item.Schedules, model.Schedule{
			Kind: "calendar", Description: DescribeCalendarRule(rule),
		})
	}
	if item.KeepAlive || len(item.WatchPaths) > 0 || len(item.QueueDirs) > 0 || enabledValue(document.Sockets) || enabledValue(document.MachServices) {
		item.Categories = model.AddCategory(item.Categories, model.CategoryBackground)
	}
	return item, nil
}

func enabledValue(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "false" || string(raw) == "{}" || string(raw) == "[]" {
		return false
	}
	var value bool
	if json.Unmarshal(raw, &value) == nil {
		return value
	}
	return true
}

func parseCalendarRules(raw json.RawMessage) ([]model.CalendarRule, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var rules []model.CalendarRule
	if err := json.Unmarshal(raw, &rules); err == nil {
		return rules, nil
	}
	var rule model.CalendarRule
	if err := json.Unmarshal(raw, &rule); err != nil {
		return nil, err
	}
	return []model.CalendarRule{rule}, nil
}

func describeInterval(seconds int) string {
	if seconds%3600 == 0 {
		return fmt.Sprintf("Every %d hour(s)", seconds/3600)
	}
	if seconds%60 == 0 {
		return fmt.Sprintf("Every %d minute(s)", seconds/60)
	}
	return fmt.Sprintf("Every %d second(s)", seconds)
}

func DescribeCalendarRule(rule model.CalendarRule) string {
	parts := make([]string, 0, 5)
	if rule.Month != nil {
		parts = append(parts, fmt.Sprintf("month=%d", *rule.Month))
	}
	if rule.Day != nil {
		parts = append(parts, fmt.Sprintf("day=%d", *rule.Day))
	}
	if rule.Weekday != nil {
		parts = append(parts, fmt.Sprintf("weekday=%d", *rule.Weekday))
	}
	if rule.Hour != nil {
		parts = append(parts, fmt.Sprintf("hour=%02d", *rule.Hour))
	}
	if rule.Minute != nil {
		parts = append(parts, fmt.Sprintf("minute=%02d", *rule.Minute))
	}
	if len(parts) == 0 {
		return "Calendar trigger"
	}
	return strings.Join(parts, " ")
}
