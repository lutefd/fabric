package cli

import (
	"sort"
	"strings"

	"github.com/lutefd/fabric/internal/core"
)

func relevantEvents(events []DirectionEvent, issue string, areas []string) []DirectionEvent {
	return relevantEventsForScope(events, issue, "", areas)
}

func relevantEventsForScope(events []DirectionEvent, issue, pr string, areas []string, pathSets ...[]string) []DirectionEvent {
	var matches []DirectionEvent
	var paths []string
	if len(pathSets) > 0 {
		paths = pathSets[0]
	}
	for _, event := range events {
		if !isActiveEvent(event) {
			continue
		}
		if reasonForScope(event, issue, pr, areas, paths).matched() {
			matches = append(matches, event)
		}
	}
	return matches
}

func staleThreads(event DirectionEvent, threads map[string]ThreadRecord) ([]string, error) {
	_, stale, err := seenAndStaleFromReceipts(event, threads)
	return stale, err
}

func reasonForScope(event DirectionEvent, issue, pr string, areas []string, pathSets ...[]string) matchReason {
	reason := matchReason{Global: event.Scope.Global}
	if event.Scope.Issue != "" && issue != "" && event.Scope.Issue == issue {
		reason.Issue = true
	}
	if event.Scope.PR != "" && pr != "" && event.Scope.PR == pr {
		reason.PR = true
	}
	for _, eventArea := range event.Scope.Areas {
		for _, area := range areas {
			if eventArea != "" && eventArea == area {
				reason.Areas = append(reason.Areas, area)
			}
		}
	}
	paths := expandAreaPaths(areas, nil)
	if len(pathSets) > 0 {
		paths = expandAreaPaths(areas, pathSets[0])
	}
	eventPaths := expandAreaPaths(event.Scope.Areas, event.Scope.Paths)
	for _, eventPath := range eventPaths {
		for _, requestedPath := range paths {
			if scopePathMatches(eventPath, requestedPath) || scopePathMatches(requestedPath, eventPath) {
				reason.Paths = append(reason.Paths, requestedPath)
			}
		}
	}
	return reason
}

func (m matchReason) matched() bool {
	return m.Global || m.Issue || m.PR || len(m.Paths) > 0 || len(m.Areas) > 0
}

func scopePathMatches(pattern, value string) bool {
	pattern = strings.TrimPrefix(strings.TrimSpace(pattern), "./")
	value = strings.TrimPrefix(strings.TrimSpace(value), "./")
	if pattern == "" || value == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "**")
		return strings.HasPrefix(value, prefix)
	}
	return core.PathMatches(pattern, value)
}

func prioritizedEvents(events []DirectionEvent, issue, pr string, areas []string, pathSets ...[]string) []DirectionEvent {
	resolutions := map[string]DirectionEvent{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolutions[event.Challenges] = event
		}
	}
	ordered := append([]DirectionEvent(nil), events...)
	var paths []string
	if len(pathSets) > 0 {
		paths = pathSets[0]
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left := eventPriority(ordered[i], issue, pr, areas, paths, resolutions)
		right := eventPriority(ordered[j], issue, pr, areas, paths, resolutions)
		if left != right {
			return left < right
		}
		if ordered[i].CreatedAt != ordered[j].CreatedAt {
			return ordered[i].CreatedAt < ordered[j].CreatedAt
		}
		return ordered[i].ID < ordered[j].ID
	})
	return ordered
}

func eventPriority(event DirectionEvent, issue, pr string, areas, paths []string, resolutions map[string]DirectionEvent) int {
	reason := reasonForScope(event, issue, pr, areas, paths)
	if event.Conflict != nil || isOpenChallenge(event, resolutions) {
		return 0
	}
	ranked := core.Match(core.DirectionToRecord(event), core.RelevanceContext{Issue: issue, PR: pr, Areas: areas, Paths: expandAreaPaths(areas, paths)})
	kind := 5
	if event.Kind == "review_direction" {
		kind = 1
	} else if event.Kind == "review_requirement" {
		kind = 2
	}
	_ = reason
	return ranked.Tier*10 + kind
}

func isOpenChallenge(event DirectionEvent, resolutions map[string]DirectionEvent) bool {
	if event.Kind != "challenge" {
		return false
	}
	if event.Status != "" && event.Status != "open" {
		return false
	}
	_, resolved := resolutions[event.ID]
	return !resolved
}

func challengeResolution(events []DirectionEvent, challengeID string) (DirectionEvent, bool) {
	var latest DirectionEvent
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges == challengeID {
			latest = event
		}
	}
	return latest, latest.ID != ""
}
