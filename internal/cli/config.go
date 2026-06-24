package cli

import (
	"bufio"
	"os"
	"strings"
)

func loadAreaPathMappings() map[string][]string {
	file, err := os.Open(configPath)
	if err != nil {
		return nil
	}
	defer file.Close()
	mappings := map[string][]string{}
	inAreas := false
	currentArea := ""
	inPaths := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if indent == 0 {
			inAreas = trimmed == "areas:"
			currentArea, inPaths = "", false
			continue
		}
		if !inAreas || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if indent == 2 && strings.HasSuffix(trimmed, ":") {
			currentArea = strings.TrimSuffix(trimmed, ":")
			inPaths = false
			continue
		}
		if currentArea != "" && indent == 4 && trimmed == "paths:" {
			inPaths = true
			continue
		}
		if currentArea != "" && inPaths && indent >= 6 && strings.HasPrefix(trimmed, "-") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			value = strings.Trim(value, `"'`)
			if value != "" {
				mappings[currentArea] = append(mappings[currentArea], value)
			}
		}
	}
	return mappings
}

func applyAreaPathMappings(events []DirectionEvent) {
	for i := range events {
		events[i].Scope.Paths = expandAreaPaths(events[i].Scope.Areas, events[i].Scope.Paths)
	}
}

func expandAreaPaths(areas, paths []string) []string {
	expanded := append([]string(nil), paths...)
	seen := map[string]bool{}
	for _, value := range expanded {
		seen[value] = true
	}
	mappings := loadAreaPathMappings()
	for _, area := range areas {
		for _, pattern := range mappings[area] {
			if !seen[pattern] {
				expanded = append(expanded, pattern)
				seen[pattern] = true
			}
		}
	}
	return expanded
}
