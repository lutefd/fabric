package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/lutefd/fabric/protocol"
)

func runContinue(args []string) error {
	fs := flag.NewFlagSet("continue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	threadID := fs.String("thread", "", "thread id")
	budget := fs.Int("budget", 700, "token budget")
	areas := stringListFlag{}
	paths := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fs.Var(&paths, "path", "repository path or glob, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pr == "" && *issue == "" && len(areas) == 0 && len(paths) == 0 {
		return errors.New("continue requires --pr, --issue, --area, or --path")
	}
	if *budget <= 0 {
		return errors.New("--budget must be positive")
	}
	resolvedThreadID, err := resolveThreadID(*threadID)
	if err != nil {
		return err
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	resolvedIssue, resolvedAreas := inferScopeFromPR(events, *pr, *issue, areas)
	matches := relevantEventsForScope(events, resolvedIssue, *pr, resolvedAreas, paths)
	capped, omitted := capEventsByBudget(prioritizedEvents(matches, resolvedIssue, *pr, resolvedAreas, paths), *budget)
	markdown := continuationMarkdown(resolvedIssue, *pr, capped, omitted)
	if err := writeFile(continuePath, markdown); err != nil {
		return err
	}
	record := ThreadRecord{
		ThreadID:  resolvedThreadID,
		CreatedAt: nowString(),
		Issue:     resolvedIssue,
		PR:        *pr,
		Areas:     resolvedAreas,
		Paths:     paths,
		UpdatedAt: nowString(),
	}
	if err := saveCurrentThreadID(resolvedThreadID); err != nil {
		return err
	}
	if err := saveRuntimeThread(record, protocol.EventThreadScopeChanged); err != nil {
		return err
	}
	projection, err := createProjection("continue", resolvedThreadID,
		protocol.Scope{Repo: repoName(), Issue: resolvedIssue, PR: *pr, Areas: resolvedAreas, Paths: paths}, capped, omitted)
	if err != nil {
		return err
	}
	if _, err := recordProjectionReceipt(projection, protocol.ReceiptDelivered, "fabric-cli"); err != nil {
		return err
	}
	setMachineResult(map[string]any{"thread": record, "projection": projection, "records": capped})
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
