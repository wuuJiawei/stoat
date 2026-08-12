package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

type ItemType string

const (
	TypeLoginItem    ItemType = "login_item"
	TypeLaunchAgent  ItemType = "launch_agent"
	TypeLaunchDaemon ItemType = "launch_daemon"
	TypeCron         ItemType = "cron"
)

type Scope string

const (
	ScopeUser   Scope = "user"
	ScopeSystem Scope = "system"
)

type SourceType string

const (
	SourceLaunchd SourceType = "launchd"
	SourceBTM     SourceType = "background_task_management"
	SourceCron    SourceType = "cron"
)

type Category string

const (
	CategoryStartup    Category = "startup"
	CategoryScheduled  Category = "scheduled"
	CategoryBackground Category = "background"
)

type RiskLevel string

const (
	RiskTrusted   RiskLevel = "trusted"
	RiskNormal    RiskLevel = "normal"
	RiskAttention RiskLevel = "attention"
	RiskHigh      RiskLevel = "high"
)

type CalendarRule struct {
	Minute  *int `json:"minute,omitempty"`
	Hour    *int `json:"hour,omitempty"`
	Day     *int `json:"day,omitempty"`
	Month   *int `json:"month,omitempty"`
	Weekday *int `json:"weekday,omitempty"`
}

type Schedule struct {
	Kind        string `json:"kind"`
	Expression  string `json:"expression,omitempty"`
	Description string `json:"description"`
}

type SignatureInfo struct {
	Checked     bool   `json:"checked"`
	Signed      bool   `json:"signed"`
	AppleSigned bool   `json:"apple_signed"`
	Identifier  string `json:"identifier,omitempty"`
	TeamID      string `json:"team_id,omitempty"`
	Signer      string `json:"signer,omitempty"`
}

type RuntimeInfo struct {
	Checked      bool   `json:"checked"`
	Domain       string `json:"domain,omitempty"`
	Loaded       bool   `json:"loaded"`
	Running      bool   `json:"running"`
	State        string `json:"state,omitempty"`
	PID          int    `json:"pid,omitempty"`
	LastExitCode *int   `json:"last_exit_code,omitempty"`
	Disabled     bool   `json:"disabled"`
}

type AttributionInfo struct {
	Checked    bool     `json:"checked"`
	AppPath    string   `json:"app_path,omitempty"`
	BundleID   string   `json:"bundle_id,omitempty"`
	Name       string   `json:"name,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Evidence   []string `json:"evidence,omitempty"`
}

type PersistenceItem struct {
	ID               string          `json:"id"`
	Label            string          `json:"label"`
	Type             ItemType        `json:"type"`
	Scope            Scope           `json:"scope"`
	Source           SourceType      `json:"source"`
	Categories       []Category      `json:"categories"`
	Program          string          `json:"program,omitempty"`
	Arguments        []string        `json:"arguments,omitempty"`
	Command          string          `json:"command,omitempty"`
	WorkingDir       string          `json:"working_dir,omitempty"`
	RunAtLoad        bool            `json:"run_at_load"`
	KeepAlive        bool            `json:"keep_alive"`
	Schedules        []Schedule      `json:"schedules,omitempty"`
	WatchPaths       []string        `json:"watch_paths,omitempty"`
	QueueDirs        []string        `json:"queue_directories,omitempty"`
	ConfigPath       string          `json:"config_path,omitempty"`
	AppPath          string          `json:"app_path,omitempty"`
	BundleID         string          `json:"bundle_id,omitempty"`
	User             string          `json:"user,omitempty"`
	Exists           bool            `json:"exists"`
	Owner            string          `json:"owner,omitempty"`
	Mode             string          `json:"mode,omitempty"`
	WritableByOthers bool            `json:"writable_by_others"`
	Signature        SignatureInfo   `json:"signature"`
	Runtime          RuntimeInfo     `json:"runtime"`
	Attribution      AttributionInfo `json:"attribution"`
	RiskScore        int             `json:"risk_score"`
	RiskLevel        RiskLevel       `json:"risk_level"`
	RiskReasons      []string        `json:"risk_reasons,omitempty"`
}

func StableID(source SourceType, configPath, label string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{string(source), configPath, label}, "\x00")))
	return fmt.Sprintf("%x", sum[:10])
}

func (i PersistenceItem) HasCategory(category Category) bool {
	for _, current := range i.Categories {
		if current == category {
			return true
		}
	}
	return false
}

func AddCategory(categories []Category, category Category) []Category {
	for _, current := range categories {
		if current == category {
			return categories
		}
	}
	return append(categories, category)
}
