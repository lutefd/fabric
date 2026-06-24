package cli

import "strings"

func capEventsByBudget(events []DirectionEvent, budget int) ([]DirectionEvent, bool) {
	if budget <= 0 {
		return nil, len(events) > 0
	}

	var selected []DirectionEvent
	used := 0
	for _, event := range events {
		cost := approximateTokens(eventBudgetText(event))
		if used+cost > budget {
			return selected, true
		}
		selected = append(selected, event)
		used += cost
	}
	return selected, false
}

func eventBudgetText(event DirectionEvent) string {
	parts := []string{
		event.Text, event.Reason, event.Status, event.LifecycleReason,
		event.Source.Type, event.Source.ThreadID, event.Source.PR, event.Source.URL,
		event.Scope.Issue, event.Scope.PR, strings.Join(event.Scope.Areas, " "), strings.Join(event.Scope.Paths, " "),
		strings.Join(event.RejectedPaths, " "), strings.Join(event.PreferredPaths, " "),
	}
	if event.Conflict != nil {
		parts = append(parts, event.Conflict.Message, strings.Join(event.Conflict.CompetingEventIDs, " "))
	}
	for _, evidence := range event.Evidence {
		parts = append(parts, evidence.Type, evidence.URL, evidence.Author, evidence.Text)
	}
	return strings.Join(parts, " ")
}

func approximateTokens(text string) int {
	if text == "" {
		return 1
	}
	return (len(text) + 3) / 4
}
