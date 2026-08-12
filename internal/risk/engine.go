package risk

import (
	"path/filepath"
	"strings"

	"github.com/wuuJiawei/stoat/internal/model"
)

type Finding struct {
	Score    int
	Reason   string
	Evidence []string
}

type Rule interface {
	ID() string
	Evaluate(item model.PersistenceItem) []Finding
}

type ruleFunc struct {
	id       string
	evaluate func(model.PersistenceItem) []Finding
}

func (r ruleFunc) ID() string { return r.id }

func (r ruleFunc) Evaluate(item model.PersistenceItem) []Finding { return r.evaluate(item) }

type Engine struct {
	rules  []Rule
	policy Policy
}

func NewEngine() *Engine {
	return &Engine{rules: defaultRules()}
}

func NewEngineWithRules(rules ...Rule) *Engine {
	return &Engine{rules: append([]Rule(nil), rules...)}
}

func (e *Engine) WithPolicy(policy Policy) *Engine {
	return &Engine{rules: append([]Rule(nil), e.rules...), policy: policy}
}

func (e *Engine) Evaluate(item *model.PersistenceItem) {
	score := 20
	reasons := make([]string, 0, 8)
	findings := make([]model.RiskFinding, 0, 8)
	seen := make(map[string]struct{})
	for _, rule := range e.rules {
		for _, finding := range rule.Evaluate(*item) {
			riskFinding := model.RiskFinding{
				RuleID:   rule.ID(),
				Score:    finding.Score,
				Reason:   finding.Reason,
				Evidence: append([]string(nil), finding.Evidence...),
			}
			if reason, suppressed := e.policy.Suppression(item.ID, rule.ID()); suppressed && finding.Score > 0 {
				riskFinding.Suppressed = true
				riskFinding.SuppressionReason = reason
				findings = append(findings, riskFinding)
				continue
			}
			score += finding.Score
			findings = append(findings, riskFinding)
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
	item.RiskFindings = findings
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

func KnownRuleIDs() map[string]bool {
	result := make(map[string]bool)
	for _, rule := range defaultRules() {
		result[rule.ID()] = true
	}
	return result
}

func defaultRules() []Rule {
	return []Rule{
		newRule("apple-system-location", func(item model.PersistenceItem) []Finding {
			if strings.HasPrefix(item.ConfigPath, "/System/Library/") || strings.HasPrefix(item.Program, "/System/Library/") {
				return []Finding{{Score: -40, Reason: "Apple system location", Evidence: []string{firstEvidence(item.ConfigPath, item.Program)}}}
			}
			return nil
		}),
		newRule("apple-code-signature", func(item model.PersistenceItem) []Finding {
			if item.Signature.AppleSigned {
				return []Finding{{Score: -40, Reason: "Apple code signature", Evidence: compactEvidence(item.Signature.Signer, item.Signature.TeamID)}}
			}
			return nil
		}),
		newRule("missing-executable", func(item model.PersistenceItem) []Finding {
			if item.Program != "" && strings.HasPrefix(item.Program, "/") && !item.Exists {
				return []Finding{{Score: 25, Reason: "Executable does not exist", Evidence: []string{item.Program}}}
			}
			return nil
		}),
		newRule("script-execution", func(item model.PersistenceItem) []Finding {
			lower := strings.ToLower(item.Program)
			extension := strings.ToLower(filepath.Ext(lower))
			if extension == ".sh" || extension == ".py" || extension == ".js" || strings.Contains(lower, "/osascript") {
				return []Finding{{Score: 15, Reason: "Executes a script", Evidence: []string{item.Program}}}
			}
			return nil
		}),
		newRule("every-minute", func(item model.PersistenceItem) []Finding {
			for _, schedule := range item.Schedules {
				if schedule.Expression == "* * * * *" || schedule.Expression == "60s" {
					return []Finding{{Score: 10, Reason: "Runs every minute", Evidence: []string{schedule.Expression}}}
				}
			}
			return nil
		}),
		newRule("persistent-at-load", func(item model.PersistenceItem) []Finding {
			if item.RunAtLoad && item.KeepAlive {
				return []Finding{{Score: 15, Reason: "Runs at load and stays alive"}}
			}
			return nil
		}),
		newRule("downloads-location", func(item model.PersistenceItem) []Finding {
			if strings.Contains(strings.ToLower(item.Program), "/downloads/") {
				return []Finding{{Score: 15, Reason: "Executable is in Downloads", Evidence: []string{item.Program}}}
			}
			return nil
		}),
		newRule("temporary-location", func(item model.PersistenceItem) []Finding {
			if isTemporaryPath(strings.ToLower(item.Program)) {
				return []Finding{{Score: 40, Reason: "Executable is in a temporary directory", Evidence: []string{item.Program}}}
			}
			return nil
		}),
		newRule("world-writable", func(item model.PersistenceItem) []Finding {
			if item.WritableByOthers {
				return []Finding{{Score: 50, Reason: "Executable is writable by other users", Evidence: compactEvidence(item.Program, item.Mode)}}
			}
			return nil
		}),
		newRule("unsigned-root-daemon", func(item model.PersistenceItem) []Finding {
			if item.Type == model.TypeLaunchDaemon && item.User == "root" && item.Signature.Checked && !item.Signature.Signed {
				return []Finding{{Score: 30, Reason: "Unsigned root LaunchDaemon", Evidence: []string{item.Program}}}
			}
			return nil
		}),
		newRule("download-pipe-shell", func(item model.PersistenceItem) []Finding {
			command := strings.ToLower(item.Command + " " + strings.Join(item.Arguments, " "))
			if (strings.Contains(command, "curl ") || strings.Contains(command, "wget ")) &&
				(strings.Contains(command, "| sh") || strings.Contains(command, "| bash")) {
				return []Finding{{Score: 70, Reason: "Downloads and pipes code to a shell", Evidence: []string{"network download piped to shell interpreter"}}}
			}
			return nil
		}),
		newRule("attribution-mismatch", func(item model.PersistenceItem) []Finding {
			if item.Attribution.Confidence == "low" {
				return []Finding{{Score: 20, Reason: "Application attribution evidence conflicts", Evidence: append([]string(nil), item.Attribution.Evidence...)}}
			}
			return nil
		}),
	}
}

func newRule(id string, evaluate func(model.PersistenceItem) []Finding) Rule {
	return ruleFunc{id: id, evaluate: evaluate}
}

func compactEvidence(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func firstEvidence(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "system path"
}

func isTemporaryPath(path string) bool {
	return strings.HasPrefix(path, "/tmp/") || strings.HasPrefix(path, "/private/tmp/") || strings.HasPrefix(path, "/var/tmp/")
}
