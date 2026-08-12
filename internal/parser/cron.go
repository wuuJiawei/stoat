package parser

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/wuuJiawei/stoat/internal/model"
)

func ParseCron(data []byte, scope model.Scope, configPath string, systemFormat bool) ([]model.PersistenceItem, []error) {
	var items []model.PersistenceItem
	var parseErrors []error
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || isCronEnvironment(line) {
			continue
		}
		item, err := parseCronLine(line, scope, configPath, systemFormat, lineNumber)
		if err != nil {
			parseErrors = append(parseErrors, err)
			continue
		}
		items = append(items, item)
	}
	if err := scanner.Err(); err != nil {
		parseErrors = append(parseErrors, fmt.Errorf("scan cron: %w", err))
	}
	return items, parseErrors
}

func parseCronLine(line string, scope model.Scope, configPath string, systemFormat bool, lineNumber int) (model.PersistenceItem, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return model.PersistenceItem{}, fmt.Errorf("cron line %d is empty", lineNumber)
	}
	var expression, command, user string
	categories := []model.Category{model.CategoryScheduled}
	if strings.HasPrefix(fields[0], "@") {
		minimum := 2
		commandIndex := 1
		if systemFormat {
			minimum = 3
			user = fields[1]
			commandIndex = 2
		}
		if len(fields) < minimum {
			return model.PersistenceItem{}, fmt.Errorf("cron line %d: incomplete macro entry", lineNumber)
		}
		expression = fields[0]
		command = strings.Join(fields[commandIndex:], " ")
		if expression == "@reboot" {
			categories = []model.Category{model.CategoryStartup}
		}
	} else {
		minimum := 6
		commandIndex := 5
		if systemFormat {
			minimum = 7
			user = fields[5]
			commandIndex = 6
		}
		if len(fields) < minimum {
			return model.PersistenceItem{}, fmt.Errorf("cron line %d: expected at least %d fields", lineNumber, minimum)
		}
		expression = strings.Join(fields[:5], " ")
		command = strings.Join(fields[commandIndex:], " ")
	}
	program := ""
	commandFields := strings.Fields(command)
	if len(commandFields) > 0 {
		program = commandFields[0]
	}
	label := fmt.Sprintf("cron:%d", lineNumber)
	return model.PersistenceItem{
		ID:         model.StableID(model.SourceCron, configPath, line),
		Label:      label,
		Type:       model.TypeCron,
		Scope:      scope,
		Source:     model.SourceCron,
		Categories: categories,
		Program:    program,
		Command:    command,
		User:       user,
		ConfigPath: configPath,
		Schedules: []model.Schedule{{
			Kind: "cron", Expression: expression, Description: DescribeCron(expression),
		}},
	}, nil
}

func isCronEnvironment(line string) bool {
	separator := strings.IndexByte(line, '=')
	space := strings.IndexAny(line, " \t")
	return separator > 0 && (space == -1 || separator < space)
}

func DescribeCron(expression string) string {
	switch expression {
	case "@reboot":
		return "At login or boot"
	case "* * * * *":
		return "Every minute"
	case "@hourly":
		return "Every hour"
	case "@daily", "@midnight":
		return "Every day"
	case "@weekly":
		return "Every week"
	case "@monthly":
		return "Every month"
	case "@yearly", "@annually":
		return "Every year"
	}
	fields := strings.Fields(expression)
	if len(fields) == 5 && strings.HasPrefix(fields[0], "*/") && fields[1] == "*" && fields[2] == "*" && fields[3] == "*" && fields[4] == "*" {
		return "Every " + strings.TrimPrefix(fields[0], "*/") + " minutes"
	}
	return expression
}
