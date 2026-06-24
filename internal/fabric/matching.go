package fabric

import "sort"

func latestRelevantEventID(events []DirectionEvent, issue string, areas []string) string {
	return latestRelevantEventIDForScope(events, issue, "", areas)
}

func latestRelevantEventIDForScope(events []DirectionEvent, issue, pr string, areas []string) string {
	matches := relevantEventsForScope(events, issue, pr, areas)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1].ID
}

func relevantEvents(events []DirectionEvent, issue string, areas []string) []DirectionEvent {
	return relevantEventsForScope(events, issue, "", areas)
}

func relevantEventsForScope(events []DirectionEvent, issue, pr string, areas []string) []DirectionEvent {
	var matches []DirectionEvent
	for _, event := range events {
		if !isActiveEvent(event) {
			continue
		}
		if reasonForScope(event, issue, pr, areas).matched() {
			matches = append(matches, event)
		}
	}
	return matches
}

func relevantEventsSinceForScope(events []DirectionEvent, issue, pr string, areas []string, lastSeen string) []DirectionEvent {
	var matches []DirectionEvent
	lastSeenNumber := eventNumber(lastSeen)
	for _, event := range relevantEventsForScope(events, issue, pr, areas) {
		if eventNumber(event.ID) > lastSeenNumber {
			matches = append(matches, event)
		}
	}
	return matches
}

func staleThreads(event DirectionEvent, threads map[string]ThreadRecord) []string {
	var stale []string
	if !isActiveEvent(event) {
		return stale
	}
	eventID := eventNumber(event.ID)
	for id, thread := range threads {
		if id == event.Source.ThreadID {
			continue
		}
		if eventID <= eventNumber(thread.LastSeenEventID) {
			continue
		}
		if reasonForScope(event, thread.Issue, thread.PR, thread.Areas).matched() {
			stale = append(stale, id)
		}
	}
	sort.Strings(stale)
	return stale
}

func seenAndStale(event DirectionEvent, threads map[string]ThreadRecord) ([]string, []string) {
	var seen []string
	var stale []string
	if !isActiveEvent(event) {
		return seen, stale
	}
	for id, thread := range threads {
		if !reasonForScope(event, thread.Issue, thread.PR, thread.Areas).matched() {
			continue
		}
		if eventNumber(thread.LastSeenEventID) >= eventNumber(event.ID) {
			seen = append(seen, id)
		} else {
			stale = append(stale, id)
		}
	}
	sort.Strings(seen)
	sort.Strings(stale)
	return seen, stale
}

func reasonForScope(event DirectionEvent, issue, pr string, areas []string) matchReason {
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
	return reason
}

func (m matchReason) matched() bool {
	return m.Global || m.Issue || m.PR || len(m.Areas) > 0
}

func prioritizedEvents(events []DirectionEvent, issue, pr string, areas []string) []DirectionEvent {
	resolutions := map[string]DirectionEvent{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolutions[event.Challenges] = event
		}
	}
	ordered := append([]DirectionEvent(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := eventPriority(ordered[i], issue, pr, areas, resolutions)
		right := eventPriority(ordered[j], issue, pr, areas, resolutions)
		if left != right {
			return left < right
		}
		return eventNumber(ordered[i].ID) < eventNumber(ordered[j].ID)
	})
	return ordered
}

func eventPriority(event DirectionEvent, issue, pr string, areas []string, resolutions map[string]DirectionEvent) int {
	reason := reasonForScope(event, issue, pr, areas)
	if isOpenChallenge(event, resolutions) {
		if reason.PR {
			return 1
		}
		if reason.Issue {
			return 2
		}
		return 3
	}
	if event.Kind == "review_direction" {
		if reason.PR {
			return 4
		}
		if reason.Issue {
			return 5
		}
		if len(reason.Areas) > 0 {
			return 6
		}
		return 7
	}
	if event.Kind == "review_requirement" {
		if reason.PR {
			return 8
		}
		if reason.Issue {
			return 9
		}
		if len(reason.Areas) > 0 {
			return 10
		}
		return 11
	}
	if reason.Issue {
		return 12
	}
	if len(reason.Areas) > 0 {
		return 13
	}
	return 14
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
