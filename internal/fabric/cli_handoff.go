package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

const handoffPath = ".fabric/generated/HANDOFF.md"

func runHandoff(args []string) error {
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	budget := fs.Int("budget", 900, "token budget")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pr == "" {
		return errors.New("handoff requires --pr")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	threads, err := loadThreads()
	if err != nil {
		return err
	}

	resolvedIssue := *issue
	resolvedAreas := areas
	if resolvedIssue == "" && len(resolvedAreas) == 0 {
		for _, thread := range threads {
			if thread.PR == *pr {
				resolvedIssue = thread.Issue
				resolvedAreas = thread.Areas
				break
			}
		}
	}

	matches := relevantEventsForScope(events, resolvedIssue, *pr, resolvedAreas)
	challenges := openChallengesForPR(events, *pr)
	reviewDirection, liveRequirements := partitionReviewEvents(matches)

	markdown := handoffMarkdown(*pr, resolvedIssue, resolvedAreas, challenges, reviewDirection, liveRequirements)
	if err := writeFile(handoffPath, markdown); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", handoffPath)
	_ = budget
	return nil
}

func openChallengesForPR(events []DirectionEvent, pr string) []DirectionEvent {
	var open []DirectionEvent
	resolved := map[string]string{}
	for _, event := range events {
		if event.Kind == "challenge_resolution" && event.Challenges != "" {
			resolved[event.Challenges] = event.Status
		}
	}
	for _, event := range events {
		if event.Kind == "challenge" && event.Scope.PR == pr {
			if status, ok := resolved[event.ID]; !ok || status == "open" {
				open = append(open, event)
			}
		}
	}
	return open
}

func partitionReviewEvents(events []DirectionEvent) (directions, requirements []DirectionEvent) {
	for _, event := range events {
		switch event.Kind {
		case "review_direction":
			directions = append(directions, event)
		case "review_requirement":
			requirements = append(requirements, event)
		}
	}
	return
}

func handoffMarkdown(pr, issue string, areas []string, challenges, directions, requirements []DirectionEvent) string {
	md := "# Fabric Handoff\n\n"
	md += fmt.Sprintf("PR:\n%s\n\n", pr)
	md += fmt.Sprintf("Issue:\n%s\n\n", emptyAsNone(issue))
	md += "Areas:\n"
	if len(areas) == 0 {
		md += "- none\n"
	} else {
		for _, area := range areas {
			md += fmt.Sprintf("- %s\n", area)
		}
	}
	md += "\n"

	md += "Current review direction:\n\n"
	if len(directions) == 0 {
		md += "No active review direction.\n"
	} else {
		for i, event := range directions {
			md += fmt.Sprintf("%d. %s\n", i+1, event.Text)
			if len(event.RejectedPaths) > 0 {
				md += "\n   Rejected paths:\n"
				for _, path := range event.RejectedPaths {
					md += fmt.Sprintf("   - %s\n", path)
				}
			}
			if len(event.PreferredPaths) > 0 {
				md += "\n   Preferred paths:\n"
				for _, path := range event.PreferredPaths {
					md += fmt.Sprintf("   - %s\n", path)
				}
			}
			if event.Reason != "" {
				md += fmt.Sprintf("\n   Reason:\n   %s\n", event.Reason)
			}
			md += "\n"
		}
	}
	md += "\n"

	md += "Active live requirements:\n\n"
	if len(requirements) == 0 {
		md += "No active live requirements.\n"
	} else {
		for i, event := range requirements {
			md += fmt.Sprintf("%d. %s\n", i+1, event.Text)
			if event.Reason != "" {
				md += fmt.Sprintf("\n   Reason:\n   %s\n", event.Reason)
			}
			md += "\n"
		}
	}
	md += "\n"

	md += "Open challenges:\n\n"
	if len(challenges) == 0 {
		md += "- none\n"
	} else {
		for _, event := range challenges {
			md += fmt.Sprintf("- %s\n", event.Text)
		}
	}
	md += "\n"

	md += "Do not reopen:\n\n"
	if len(directions) == 0 {
		md += "- none\n"
	} else {
		for _, event := range directions {
			for _, path := range event.RejectedPaths {
				md += fmt.Sprintf("- %s\n", path)
			}
		}
	}

	return md
}
