package fabric

import (
	"fmt"
	"strings"
)

const budgetOmittedMessage = "Some relevant direction was omitted because the budget was reached.\nRun with --budget N to include more."

func syncMarkdown(thread ThreadRecord, events []DirectionEvent, omitted bool) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Sync Delta")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Thread:")
	fmt.Fprintln(&b, thread.ThreadID)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "New relevant direction since last sync:")
	fmt.Fprintln(&b)
	if len(events) == 0 {
		fmt.Fprintln(&b, "No direction included within the current budget.")
	} else {
		for i, event := range events {
			fmt.Fprintf(&b, "%d. %s\n", i+1, event.Text)
		}
	}
	writeBudgetOmission(&b, omitted)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Source:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "(none included)")
	} else {
		for _, event := range events {
			fmt.Fprintf(&b, "Human note from related thread %s.\n", emptyAsUnknown(event.Source.ThreadID))
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Why it applies:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "(none included)")
	} else {
		for _, event := range events {
			writeReasonLines(&b, event, thread.Issue, thread.Areas)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Next action:")
	fmt.Fprintln(&b, "Adjust your plan before continuing.")
	return b.String()
}

func noSyncMarkdown(threadID string) string {
	return fmt.Sprintf(`# Sync Delta

Thread:
%s

No new relevant direction for this thread.
`, threadID)
}

func preflightMarkdown(task, issue string, areas []string, events []DirectionEvent, omitted bool) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Task Direction")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Task:")
	fmt.Fprintln(&b, task)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Relevant direction:")
	fmt.Fprintln(&b)
	if len(events) == 0 && omitted {
		fmt.Fprintln(&b, "No direction included within the current budget.")
	} else if len(events) == 0 {
		fmt.Fprintln(&b, "No active direction found.")
	} else {
		for i, event := range events {
			fmt.Fprintf(&b, "%d. %s\n", i+1, event.Text)
		}
	}
	writeBudgetOmission(&b, omitted)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Scope match:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "- None")
	} else {
		for _, event := range events {
			writeReasonLines(&b, event, issue, areas)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Instructions:")
	fmt.Fprintln(&b, "- Follow this direction unless the task explicitly requires challenging it.")
	fmt.Fprintln(&b, "- Do not silently implement a conflicting approach.")
	return b.String()
}

func writeBudgetOmission(b *strings.Builder, omitted bool) {
	if !omitted {
		return
	}
	fmt.Fprintln(b)
	fmt.Fprintln(b, budgetOmittedMessage)
}

func printExplain(issue string, areas []string, events []DirectionEvent, threads map[string]ThreadRecord) error {
	if issue != "" {
		fmt.Printf("Active direction for issue %s:\n\n", issue)
	} else {
		fmt.Printf("Active direction for area %s:\n\n", strings.Join(areas, ", "))
	}
	if len(events) == 0 {
		fmt.Println("No active direction found.")
		return nil
	}
	for _, event := range events {
		fmt.Println(event.ID)
		fmt.Println("Text:")
		fmt.Println(event.Text)
		fmt.Println()
		fmt.Println("Scope:")
		fmt.Printf("issue: %s\n", emptyAsNone(event.Scope.Issue))
		fmt.Printf("area: %s\n", emptyAsNone(strings.Join(event.Scope.Areas, ", ")))
		fmt.Println()
		fmt.Println("Source:")
		fmt.Printf("%s %s\n", event.Source.Type, sourceThread(event.Source.ThreadID))
		fmt.Println()
		seen, stale := seenAndStale(event, threads)
		fmt.Println("Seen by:")
		printIDList(seen)
		fmt.Println()
		fmt.Println("Stale:")
		printIDList(stale)
		fmt.Println()
	}
	return nil
}

func writeReasonLines(b *strings.Builder, event DirectionEvent, issue string, areas []string) {
	reason := reasonFor(event, issue, areas)
	if reason.Global {
		fmt.Fprintln(b, "- Repo-wide direction")
	}
	if reason.Issue {
		fmt.Fprintf(b, "- Same issue: %s\n", event.Scope.Issue)
	}
	for _, area := range reason.Areas {
		fmt.Fprintf(b, "- Same area: %s\n", area)
	}
}

func printIDList(ids []string) {
	if len(ids) == 0 {
		fmt.Println("- (none)")
		return
	}
	for _, id := range ids {
		fmt.Printf("- %s\n", id)
	}
}
