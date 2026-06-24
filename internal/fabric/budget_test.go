package fabric

import "testing"

func TestCapEventsByBudgetIncludesUntilBudgetReached(t *testing.T) {
	events := []DirectionEvent{
		{ID: "evt_000001", Text: "12345678"},
		{ID: "evt_000002", Text: "12345678"},
		{ID: "evt_000003", Text: "12345678"},
	}

	selected, omitted := capEventsByBudget(events, 4)
	if !omitted {
		t.Fatal("omitted = false, want true")
	}
	if len(selected) != 2 {
		t.Fatalf("selected count = %d, want 2", len(selected))
	}
	if selected[1].ID != "evt_000002" {
		t.Fatalf("last selected = %q, want evt_000002", selected[1].ID)
	}
}

func TestCapEventsByBudgetOmitsAllWhenFirstEventExceedsBudget(t *testing.T) {
	selected, omitted := capEventsByBudget([]DirectionEvent{{ID: "evt_000001", Text: "12345678"}}, 1)
	if !omitted {
		t.Fatal("omitted = false, want true")
	}
	if len(selected) != 0 {
		t.Fatalf("selected count = %d, want 0", len(selected))
	}
}

func TestCapEventsByBudgetHandlesEmptyAndInvalidBudgets(t *testing.T) {
	selected, omitted := capEventsByBudget(nil, 0)
	if omitted || len(selected) != 0 {
		t.Fatalf("empty events selected=%v omitted=%v, want none/false", selected, omitted)
	}

	selected, omitted = capEventsByBudget([]DirectionEvent{{ID: "evt_000001", Text: ""}}, 0)
	if !omitted || len(selected) != 0 {
		t.Fatalf("zero budget selected=%v omitted=%v, want none/true", selected, omitted)
	}

	if got := approximateTokens(""); got != 1 {
		t.Fatalf("empty token estimate = %d, want 1", got)
	}
	if got := approximateTokens("12345"); got != 2 {
		t.Fatalf("rounded token estimate = %d, want 2", got)
	}
}
