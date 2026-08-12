package risk

import (
	"path/filepath"
	"strings"

	"github.com/wuuJiawei/stoat/internal/model"
)

type Finding struct {
	Score  int
	Reason string
}

type Rule interface {
	Evaluate(item model.PersistenceItem) []Finding
}

type RuleFunc func(item model.PersistenceItem) []Finding

func (f RuleFunc) Evaluate(item model.PersistenceItem) []Finding { return f(item) }

type Engine struct {
	rules []Rule
}

func NewEngine() *Engine {
	return &Engine{rules: defaultRules()}
}

func NewEngineWithRules(rules ...Rule) *Engine {
	return &Engine{rules: append([]Rule(nil), rules...)}
}

func (e *Engine) Evaluate(item *model.PersistenceItem) {
	score := 20
	reasons := make([]string, 0, 8)
	seen := make(map[string]struct{})
	for _, rule := range e.rules {
		for _, finding := range rule.Evaluate(*item) {
			score += finding.Score
			if finding.Reason != "" {
				if _, exists := seen[finding.Reason]; !exists {
					seen[finding.Reason] = struct{}{}
					reasons = append(reasons, finding.Reason)
				}
			}
		}
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	item.RiskScore = score
	item.RiskLevel = Level(score)
	item.RiskReasons = reasons
}

func Level(score int) model.RiskLevel {
	switch {
	case score >= 70:
		return model.RiskHigh
	case score >= 40:
		return model.RiskAttention
	case score >= 20:
		return model.RiskNormal
	default:
		return model.RiskTrusted
	}
}

func defaultRules() []Rule {
	return []Rule{
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if strings.HasPrefix(item.ConfigPath, "/System/Library/") || strings.HasPrefix(item.Program, "/System/Library/") {
				return []Finding{{Score: -40, Reason: "Apple system location"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if item.Signature.AppleSigned {
				return []Finding{{Score: -40, Reason: "Apple code signature"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if item.Program != "" && strings.HasPrefix(item.Program, "/") && !item.Exists {
				return []Finding{{Score: 25, Reason: "Executable does not exist"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			lower := strings.ToLower(item.Program)
			extension := strings.ToLower(filepath.Ext(lower))
			if extension == ".sh" || extension == ".py" || extension == ".js" || strings.Contains(lower, "/osascript") {
				return []Finding{{Score: 15, Reason: "Executes a script"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			for _, schedule := range item.Schedules {
				if schedule.Expression == "* * * * *" || schedule.Expression == "60s" {
					return []Finding{{Score: 10, Reason: "Runs every minute"}}
				}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if item.RunAtLoad && item.KeepAlive {
				return []Finding{{Score: 15, Reason: "Runs at load and stays alive"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			lower := strings.ToLower(item.Program)
			if strings.Contains(lower, "/downloads/") {
				return []Finding{{Score: 15, Reason: "Executable is in Downloads"}}
			}
			if isTemporaryPath(lower) {
				return []Finding{{Score: 40, Reason: "Executable is in a temporary directory"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if item.WritableByOthers {
				return []Finding{{Score: 50, Reason: "Executable is writable by other users"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			if item.Type == model.TypeLaunchDaemon && item.User == "root" && item.Signature.Checked && !item.Signature.Signed {
				return []Finding{{Score: 30, Reason: "Unsigned root LaunchDaemon"}}
			}
			return nil
		}),
		RuleFunc(func(item model.PersistenceItem) []Finding {
			command := strings.ToLower(item.Command + " " + strings.Join(item.Arguments, " "))
			if (strings.Contains(command, "curl ") || strings.Contains(command, "wget ")) &&
				(strings.Contains(command, "| sh") || strings.Contains(command, "| bash")) {
				return []Finding{{Score: 70, Reason: "Downloads and pipes code to a shell"}}
			}
			return nil
		}),
	}
}

func isTemporaryPath(path string) bool {
	return strings.HasPrefix(path, "/tmp/") || strings.HasPrefix(path, "/private/tmp/") || strings.HasPrefix(path, "/var/tmp/")
}
