package fabric

import "sort"

func latestRelevantEventID(events []DirectionEvent, issue string, areas []string) string {
	matches := relevantEventsForScope(events, issue, "", areas)
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
		if reasonForScope(event, issue, pr, areas).matched() {
			matches = append(matches, event)
		}
	}
	return matches
}

func relevantEventsSince(events []DirectionEvent, issue string, areas []string, lastSeen string) []DirectionEvent {
	var matches []DirectionEvent
	lastSeenNumber := eventNumber(lastSeen)
	for _, event := range relevantEvents(events, issue, areas) {
		if eventNumber(event.ID) > lastSeenNumber {
			matches = append(matches, event)
		}
	}
	return matches
}

func staleThreads(event DirectionEvent, threads map[string]ThreadRecord) []string {
	var stale []string
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

func reasonFor(event DirectionEvent, issue string, areas []string) matchReason {
	return reasonForScope(event, issue, "", areas)
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
