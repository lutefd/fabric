package fabric

import "sort"

func latestRelevantEventID(events []DirectionEvent, issue string, areas []string) string {
	matches := relevantEvents(events, issue, areas)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1].ID
}

func relevantEvents(events []DirectionEvent, issue string, areas []string) []DirectionEvent {
	var matches []DirectionEvent
	for _, event := range events {
		if reasonFor(event, issue, areas).matched() {
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
		if reasonFor(event, thread.Issue, thread.Areas).matched() {
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
		if !reasonFor(event, thread.Issue, thread.Areas).matched() {
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
	reason := matchReason{Global: event.Scope.Global}
	if event.Scope.Issue != "" && issue != "" && event.Scope.Issue == issue {
		reason.Issue = true
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
	return m.Global || m.Issue || len(m.Areas) > 0
}
