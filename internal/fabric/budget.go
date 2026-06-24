package fabric

func capEventsByBudget(events []DirectionEvent, budget int) ([]DirectionEvent, bool) {
	if budget <= 0 {
		return nil, len(events) > 0
	}

	var selected []DirectionEvent
	used := 0
	for _, event := range events {
		cost := approximateTokens(event.Text)
		if used+cost > budget {
			return selected, true
		}
		selected = append(selected, event)
		used += cost
	}
	return selected, false
}

func approximateTokens(text string) int {
	if text == "" {
		return 1
	}
	return (len(text) + 3) / 4
}
