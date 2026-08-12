package risk

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

const (
	maxPolicySize       = 1 << 20
	maxPolicyExceptions = 500
)

type Policy struct {
	SchemaVersion     int         `json:"schema_version"`
	Exceptions        []Exception `json:"exceptions"`
	ExpiredExceptions int         `json:"-"`
}

type Exception struct {
	ItemID    string     `json:"item_id"`
	RuleIDs   []string   `json:"rule_ids"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func LoadPolicy(path string, now time.Time) (Policy, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return Policy{}, err
	}
	if !info.Mode().IsRegular() {
		return Policy{}, errors.New("refusing non-regular risk policy")
	}
	if info.Size() > maxPolicySize {
		return Policy{}, fmt.Errorf("risk policy exceeds %d bytes", maxPolicySize)
	}
	file, err := os.Open(path)
	if err != nil {
		return Policy{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, maxPolicySize+1))
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return Policy{}, fmt.Errorf("decode risk policy: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Policy{}, errors.New("risk policy contains multiple JSON values")
		}
		return Policy{}, fmt.Errorf("decode trailing risk policy data: %w", err)
	}
	return validatePolicy(policy, now.UTC())
}

func (p Policy) Suppression(itemID, ruleID string) (string, bool) {
	for _, exception := range p.Exceptions {
		if exception.ItemID != itemID {
			continue
		}
		for _, candidate := range exception.RuleIDs {
			if candidate == ruleID {
				return exception.Reason, true
			}
		}
	}
	return "", false
}

func validatePolicy(policy Policy, now time.Time) (Policy, error) {
	if policy.SchemaVersion != 1 {
		return Policy{}, fmt.Errorf("unsupported risk policy schema version %d", policy.SchemaVersion)
	}
	if len(policy.Exceptions) > maxPolicyExceptions {
		return Policy{}, fmt.Errorf("risk policy has more than %d exceptions", maxPolicyExceptions)
	}
	seen := make(map[string]struct{})
	active := make([]Exception, 0, len(policy.Exceptions))
	for index, exception := range policy.Exceptions {
		if !validItemID(exception.ItemID) {
			return Policy{}, fmt.Errorf("exception %d has invalid item_id", index)
		}
		exception.Reason = strings.TrimSpace(exception.Reason)
		if exception.Reason == "" || len(exception.Reason) > 200 {
			return Policy{}, fmt.Errorf("exception %d reason must contain 1-200 bytes", index)
		}
		if len(exception.RuleIDs) == 0 {
			return Policy{}, fmt.Errorf("exception %d must name at least one rule", index)
		}
		for _, ruleID := range exception.RuleIDs {
			if !SuppressibleRuleIDs()[ruleID] {
				return Policy{}, fmt.Errorf("exception %d cannot suppress rule %q", index, ruleID)
			}
			key := exception.ItemID + "\x00" + ruleID
			if _, exists := seen[key]; exists {
				return Policy{}, fmt.Errorf("duplicate suppression for item %s and rule %s", exception.ItemID, ruleID)
			}
			seen[key] = struct{}{}
		}
		if exception.ExpiresAt != nil && !exception.ExpiresAt.After(now) {
			policy.ExpiredExceptions++
			continue
		}
		active = append(active, exception)
	}
	policy.Exceptions = active
	return policy, nil
}

func SuppressibleRuleIDs() map[string]bool {
	return map[string]bool{
		"missing-executable":   true,
		"script-execution":     true,
		"every-minute":         true,
		"persistent-at-load":   true,
		"downloads-location":   true,
		"temporary-location":   true,
		"world-writable":       true,
		"unsigned-root-daemon": true,
		"download-pipe-shell":  true,
		"attribution-mismatch": true,
	}
}

func validItemID(value string) bool {
	if len(value) != 20 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
