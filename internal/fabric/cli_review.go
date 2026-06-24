package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runReview(args []string) error {
	if len(args) == 0 || args[0] != "note" {
		return errors.New(`expected "fabric review note"`)
	}

	fs := flag.NewFlagSet("review note", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	rejects := fs.String("rejects", "", "rejected path")
	prefer := fs.String("prefer", "", "preferred path")
	reason := fs.String("reason", "", "review reason")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *pr == "" {
		return errors.New("review note requires --pr")
	}
	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "" {
		text = structuredReviewText(*rejects, *prefer, *reason)
	}
	if text == "" {
		return errors.New("review note text is required")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}

	event := DirectionEvent{
		Kind:      "review_direction",
		CreatedAt: nowString(),
		Scope: EventScope{
			Repo:  repoName(),
			Issue: *issue,
			PR:    *pr,
			Areas: areas,
		},
		Source: EventSource{
			Type: "review",
			PR:   *pr,
		},
		Text:       text,
		Confidence: "reviewer_confirmed",
		TTL:        "until_pr_closed",
	}
	if err := appendEvent(&event); err != nil {
		return err
	}
	threads, err := loadThreads()
	if err != nil {
		return err
	}
	stale := staleThreads(event, threads)
	fmt.Printf("Recorded review direction %s for PR %s.\n", event.ID, *pr)
	fmt.Printf("Marked %d related threads stale", len(stale))
	if len(stale) == 0 {
		fmt.Println(".")
		return nil
	}
	fmt.Println(":")
	for _, id := range stale {
		fmt.Printf("- %s\n", id)
	}
	return nil
}

func structuredReviewText(rejects, prefer, reason string) string {
	var parts []string
	if strings.TrimSpace(rejects) != "" {
		parts = append(parts, "Reviewer rejected "+strings.TrimSpace(rejects)+".")
	}
	if strings.TrimSpace(prefer) != "" {
		parts = append(parts, "Preferred path: "+strings.TrimSpace(prefer)+".")
	}
	if strings.TrimSpace(reason) != "" {
		parts = append(parts, "Reason: "+strings.TrimSpace(reason)+".")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
