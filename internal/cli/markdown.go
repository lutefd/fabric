package cli

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
	fmt.Fprintln(&b, "New relevant revisions since last sync:")
	fmt.Fprintln(&b)
	if len(events) == 0 {
		fmt.Fprintln(&b, "No direction included within the current budget.")
	} else {
		for i, event := range events {
			writeProjectedRecord(&b, i+1, event)
		}
	}
	writeBudgetOmission(&b, omitted)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Source:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "(none included)")
	} else {
		for _, event := range events {
			fmt.Fprintln(&b, projectedSource(event))
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Why it applies:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "(none included)")
	} else {
		for _, event := range events {
			writeReasonLines(&b, event, thread.Issue, thread.PR, thread.Areas)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Next action:")
	fmt.Fprintln(&b, "Adjust your plan before continuing.")
	return b.String()
}

func projectedSource(event DirectionEvent) string {
	threadID := emptyAsUnknown(event.Source.ThreadID)
	switch event.Source.Type {
	case "human":
		return fmt.Sprintf("Human note from related thread %s.", threadID)
	case "review", "pr_ingest":
		if event.Source.PR != "" {
			return fmt.Sprintf("Review direction from PR %s (thread %s).", event.Source.PR, threadID)
		}
		return fmt.Sprintf("Review direction from thread %s.", threadID)
	case "tool":
		return fmt.Sprintf("Tool finding from thread %s.", threadID)
	case "agent":
		return fmt.Sprintf("Agent finding from thread %s.", threadID)
	default:
		return fmt.Sprintf("Direction source %s from thread %s.", emptyAsUnknown(event.Source.Type), threadID)
	}
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
			writeProjectedRecord(&b, i+1, event)
		}
	}
	writeBudgetOmission(&b, omitted)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Scope match:")
	if len(events) == 0 {
		fmt.Fprintln(&b, "- None")
	} else {
		for _, event := range events {
			writeReasonLines(&b, event, issue, "", areas)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Instructions:")
	fmt.Fprintln(&b, "- Follow this direction unless the task explicitly requires challenging it.")
	fmt.Fprintln(&b, "- Do not silently implement a conflicting approach.")
	return b.String()
}

func writeProjectedRecord(b *strings.Builder, index int, event DirectionEvent) {
	label := ""
	if event.Conflict != nil {
		label = "[conflict] "
	} else if !isActiveEvent(event) {
		label = "[" + event.Status + "] "
	}
	fmt.Fprintf(b, "%d. %s%s\n", index, label, event.Text)
	if event.LifecycleReason != "" && !isActiveEvent(event) {
		fmt.Fprintf(b, "   Lifecycle reason: %s\n", event.LifecycleReason)
	}
	if event.Conflict != nil {
		fmt.Fprintf(b, "   %s\n", event.Conflict.Message)
		fmt.Fprintf(b, "   Competing revisions: %s\n", strings.Join(event.Conflict.CompetingEventIDs, ", "))
	}
}

func continuationMarkdown(issue, pr string, events []DirectionEvent, omitted bool) string {
	var b strings.Builder
	resolutions := resolutionsByChallenge(events)
	fmt.Fprintln(&b, "# Continuation Context")
	fmt.Fprintln(&b)
	if pr != "" {
		fmt.Fprintln(&b, "PR:")
		fmt.Fprintln(&b, pr)
		fmt.Fprintln(&b)
	}
	if issue != "" {
		fmt.Fprintln(&b, "Issue:")
		fmt.Fprintln(&b, issue)
		fmt.Fprintln(&b)
	}
	fmt.Fprintln(&b, "Open challenge:")
	fmt.Fprintln(&b)
	challengeIndex := 0
	for _, event := range events {
		if !isOpenChallenge(event, resolutions) {
			continue
		}
		challengeIndex++
		fmt.Fprintf(&b, "%d. Direction %s is being challenged.\n", challengeIndex, event.Challenges)
		fmt.Fprintf(&b, "   Proposed exception: %s\n", challengeProposal(event))
		fmt.Fprintln(&b, "   Do not assume the old direction is final for this PR.")
	}
	if challengeIndex == 0 {
		fmt.Fprintln(&b, "No open challenge found.")
	}
	writeBudgetOmission(&b, omitted)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Resolved challenge:")
	fmt.Fprintln(&b)
	resolutionIndex := 0
	for _, event := range events {
		if event.Kind != "challenge_resolution" {
			continue
		}
		resolutionIndex++
		fmt.Fprintf(&b, "%d. %s\n", resolutionIndex, event.Text)
	}
	if resolutionIndex == 0 {
		fmt.Fprintln(&b, "No resolved challenge found.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Current review direction:")
	fmt.Fprintln(&b)
	reviewIndex := 0
	for _, event := range events {
		if event.Kind != "review_direction" {
			continue
		}
		reviewIndex++
		fmt.Fprintf(&b, "%d. %s\n", reviewIndex, event.Text)
		if len(event.RejectedPaths) > 0 {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "   Rejected paths:")
			for _, path := range event.RejectedPaths {
				fmt.Fprintf(&b, "   - %s\n", path)
			}
		}
		if len(event.PreferredPaths) > 0 {
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, "   Preferred paths:")
			for _, path := range event.PreferredPaths {
				fmt.Fprintf(&b, "   - %s\n", path)
			}
		}
		if event.Reason != "" {
			fmt.Fprintln(&b)
			fmt.Fprintf(&b, "   Reason:\n   %s\n", event.Reason)
		}
		fmt.Fprintln(&b)
	}
	if reviewIndex == 0 {
		fmt.Fprintln(&b, "No active review direction found.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Active live requirements:")
	fmt.Fprintln(&b)
	requirementIndex := 0
	for _, event := range events {
		if event.Kind != "review_requirement" {
			continue
		}
		requirementIndex++
		fmt.Fprintf(&b, "%d. %s\n", requirementIndex, event.Text)
		if event.Reason != "" {
			fmt.Fprintln(&b)
			fmt.Fprintf(&b, "   Reason:\n   %s\n", event.Reason)
		}
		fmt.Fprintln(&b)
	}
	if requirementIndex == 0 {
		fmt.Fprintln(&b, "No active live requirements.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Active issue direction:")
	fmt.Fprintln(&b)
	issueIndex := 0
	for _, event := range events {
		if event.Kind == "review_direction" || event.Kind == "review_requirement" || event.Kind == "challenge" || event.Kind == "challenge_resolution" {
			continue
		}
		issueIndex++
		fmt.Fprintf(&b, "%d. %s\n", issueIndex, event.Text)
	}
	if issueIndex == 0 {
		fmt.Fprintln(&b, "No active issue direction found.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Instructions:")
	fmt.Fprintln(&b, "- Address the review direction first.")
	fmt.Fprintln(&b, "- Do not reopen rejected implementation paths.")
	fmt.Fprintln(&b, "- If the review direction conflicts with active project/task direction, stop and ask whether to challenge it.")
	return b.String()
}

func resolutionsByChallenge(events []DirectionEvent) map[string]DirectionEvent {
	resolutions := map[string]DirectionEvent{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolutions[event.Challenges] = event
		}
	}
	return resolutions
}

func challengeProposal(event DirectionEvent) string {
	text := strings.TrimPrefix(event.Text, "Challenge direction "+event.Challenges+": ")
	if before, _, found := strings.Cut(text, " Reason: "); found {
		text = before
	}
	return strings.TrimSpace(text)
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
		printChallengeFields(event)
		fmt.Println("Scope:")
		fmt.Printf("issue: %s\n", emptyAsNone(event.Scope.Issue))
		fmt.Printf("area: %s\n", emptyAsNone(strings.Join(event.Scope.Areas, ", ")))
		fmt.Println()
		fmt.Println("Source:")
		fmt.Printf("%s %s\n", event.Source.Type, sourceThread(event.Source.ThreadID))
		fmt.Println()
		seen, stale, err := seenAndStaleFromReceipts(event, threads)
		if err != nil {
			return err
		}
		fmt.Println("Seen by:")
		printIDList(seen)
		fmt.Println()
		fmt.Println("Stale:")
		printIDList(stale)
		fmt.Println()
	}
	return nil
}

func printExplainPR(pr string, events []DirectionEvent, threads map[string]ThreadRecord) error {
	fmt.Printf("Active direction for PR %s:\n\n", pr)
	if len(events) == 0 {
		fmt.Println("No active direction found.")
		return nil
	}
	for _, event := range events {
		fmt.Println(event.ID)
		fmt.Println("Kind:")
		fmt.Println(event.Kind)
		fmt.Println()
		fmt.Println("Text:")
		fmt.Println(event.Text)
		fmt.Println()
		printChallengeFields(event)
		fmt.Println("Scope:")
		fmt.Printf("pr: %s\n", emptyAsNone(event.Scope.PR))
		fmt.Printf("issue: %s\n", emptyAsNone(event.Scope.Issue))
		fmt.Printf("area: %s\n", emptyAsNone(strings.Join(event.Scope.Areas, ", ")))
		fmt.Println()
		fmt.Println("Source:")
		fmt.Println(event.Source.Type)
		fmt.Println()
		seen, stale, err := seenAndStaleFromReceipts(event, threads)
		if err != nil {
			return err
		}
		fmt.Println("Seen by:")
		printIDList(seen)
		fmt.Println()
		fmt.Println("Stale:")
		printIDList(stale)
		fmt.Println()
	}
	return nil
}

func writeReasonLines(b *strings.Builder, event DirectionEvent, issue, pr string, areas []string) {
	reason := reasonForScope(event, issue, pr, areas)
	if reason.Global {
		fmt.Fprintln(b, "- Repo-wide direction")
	}
	if reason.PR {
		fmt.Fprintf(b, "- Same PR: %s\n", event.Scope.PR)
	}
	if reason.Issue {
		fmt.Fprintf(b, "- Same issue: %s\n", event.Scope.Issue)
	}
	for _, area := range reason.Areas {
		fmt.Fprintf(b, "- Same area: %s\n", area)
	}
}

func printChallengeFields(event DirectionEvent) {
	if event.Challenges == "" && event.Status == "" {
		return
	}
	fmt.Println("Challenge state:")
	if event.Challenges != "" {
		fmt.Printf("challenges: %s\n", event.Challenges)
	}
	if event.Status != "" {
		fmt.Printf("status: %s\n", event.Status)
	}
	fmt.Println()
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
