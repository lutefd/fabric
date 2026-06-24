package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

func runContinue(args []string) error {
	fs := flag.NewFlagSet("continue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	threadID := fs.String("thread", "", "thread id")
	budget := fs.Int("budget", 700, "token budget")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pr == "" && *issue == "" && len(areas) == 0 {
		return errors.New("continue requires --pr, --issue, or --area")
	}
	if *budget <= 0 {
		return errors.New("--budget must be positive")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	resolvedIssue, resolvedAreas := inferScopeFromPR(events, *pr, *issue, areas)
	matches := relevantEventsForScope(events, resolvedIssue, *pr, resolvedAreas)
	capped, omitted := capEventsByBudget(prioritizedEvents(matches, resolvedIssue, *pr, resolvedAreas), *budget)
	markdown := continuationMarkdown(resolvedIssue, *pr, capped, omitted)
	if err := writeFile(continuePath, markdown); err != nil {
		return err
	}
	if *threadID != "" {
		record := ThreadRecord{
			ThreadID:        *threadID,
			CreatedAt:       nowString(),
			Issue:           resolvedIssue,
			PR:              *pr,
			Areas:           resolvedAreas,
			LastSeenEventID: latestEventID(matches),
		}
		if err := appendLedger(threadsPath, record); err != nil {
			return err
		}
	}
	fmt.Print(markdown)
	return nil
}

func inferScopeFromPR(events []DirectionEvent, pr, issue string, areas []string) (string, []string) {
	resolvedIssue := issue
	resolvedAreas := append([]string(nil), areas...)
	if pr == "" {
		return resolvedIssue, resolvedAreas
	}
	seenArea := map[string]bool{}
	for _, area := range resolvedAreas {
		seenArea[area] = true
	}
	for _, event := range events {
		if event.Scope.PR != pr {
			continue
		}
		if resolvedIssue == "" {
			resolvedIssue = event.Scope.Issue
		}
		for _, area := range event.Scope.Areas {
			if area == "" || seenArea[area] {
				continue
			}
			resolvedAreas = append(resolvedAreas, area)
			seenArea[area] = true
		}
	}
	return resolvedIssue, resolvedAreas
}

func latestEventID(events []DirectionEvent) string {
	if len(events) == 0 {
		return ""
	}
	return events[len(events)-1].ID
}
