package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/lutefd/fabric/protocol"
)

func runIngestPR(args []string) error {
	if len(args) > 0 && args[0] == "template" {
		return runIngestPRTemplate(args[1:])
	}

	fs := flag.NewFlagSet("ingest-pr", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fromFile := fs.String("from-file", "", "path to review ingest file")
	stdin := fs.Bool("stdin", false, "read ingest from stdin")
	dryRun := fs.Bool("dry-run", false, "print events without creating them")
	allowDurable := fs.Bool("allow-durable", false, "allow creating durable events from ingest")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *pr == "" {
		return errors.New("ingest-pr requires --pr")
	}
	if *issue == "" && len(areas) == 0 {
		return errors.New("ingest-pr requires --issue or --area")
	}
	if (*fromFile == "" && !*stdin) || (*fromFile != "" && *stdin) {
		return errors.New("ingest-pr requires exactly one of --from-file or --stdin")
	}

	var source string
	if *fromFile != "" {
		data, err := os.ReadFile(*fromFile)
		if err != nil {
			return err
		}
		source = string(data)
	} else {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		source = string(data)
	}

	return ingestPR(*pr, *issue, areas, source, *dryRun, *allowDurable)
}

func runIngestPRTemplate(args []string) error {
	fs := flag.NewFlagSet("ingest-pr template", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pr == "" {
		return errors.New("ingest-pr template requires --pr")
	}
	if err := writeFile(ingestTemplatePath, ingestTemplate(*pr, *issue, areas)); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", ingestTemplatePath)
	return nil
}

func ingestPR(pr, issue string, areas []string, source string, dryRun, allowDurable bool) error {
	if err := ensureInitialized(); err != nil {
		return err
	}

	ingest, err := parseIngestFile(source)
	if err != nil {
		return err
	}
	if ingest.PR != "" && ingest.PR != pr {
		return fmt.Errorf("ingest file PR %q does not match --pr %q", ingest.PR, pr)
	}
	if ingest.Issue != "" && ingest.Issue != issue {
		return fmt.Errorf("ingest file issue %q does not match --issue %q", ingest.Issue, issue)
	}
	if len(ingest.Areas) > 0 {
		areas = ingest.Areas
	}

	repo := repoName()
	events, err := loadEvents()
	if err != nil {
		return err
	}

	var created []DirectionEvent
	for _, item := range ingest.Items {
		durability := inferIngestDurability(item)
		if durability == DurabilityDurable && !allowDurable {
			return fmt.Errorf("item %q would be durable; use --allow-durable or choose candidate/live", item.ReviewSays)
		}

		event := DirectionEvent{
			Kind:           inferIngestKind(item),
			CreatedAt:      nowString(),
			Durability:     durability,
			Scope:          EventScope{Repo: repo, Issue: issue, PR: pr, Areas: areas},
			Source:         EventSource{Type: "pr_ingest", PR: pr},
			Text:           item.ReviewSays,
			ReviewType:     item.Type,
			Reason:         item.Reason,
			RejectedPaths:  item.RejectedPaths,
			PreferredPaths: item.PreferredPaths,
			Evidence:       item.Evidence,
			Confidence:     "reviewer_confirmed",
			TTL:            "until_pr_closed",
		}
		if dryRun {
			id, err := protocol.NewRecordID()
			if err != nil {
				return err
			}
			event.ID = id
			events = append(events, event)
		} else {
			if err := appendEvent(&event); err != nil {
				return err
			}
		}
		created = append(created, event)
	}

	if dryRun {
		fmt.Println("Would create events:")
	} else {
		fmt.Printf("Ingested PR review direction for PR %s.\n\nCreated events:\n", pr)
	}
	for _, event := range created {
		fmt.Printf("- %s %s %s\n", event.ID, event.Kind, event.Durability)
	}
	for _, warning := range ingest.Warnings {
		fmt.Printf("Warning: %s\n", warning)
	}

	if !dryRun {
		threads, err := loadThreads()
		if err != nil {
			return err
		}
		for _, event := range created {
			stale, err := staleThreads(event, threads)
			if err != nil {
				return err
			}
			if len(stale) > 0 {
				fmt.Printf("\nMarked %d related threads stale:\n", len(stale))
				for _, id := range stale {
					fmt.Printf("- %s\n", id)
				}
			}
		}
		fmt.Printf("\nWrote:\n- %s\n\nNext:\n- run fabric continue --pr %s\n- run fabric handoff --pr %s\n", continuePath, pr, pr)
	}
	setMachineResult(map[string]any{"dry_run": dryRun, "records": created, "warnings": ingest.Warnings})

	return nil
}
