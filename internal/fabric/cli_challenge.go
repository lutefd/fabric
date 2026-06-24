package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

func runChallenge(args []string) error {
	if len(args) > 0 && args[0] == "resolve" {
		return runChallengeResolve(args[1:])
	}
	return runChallengeCreate(args)
}

func runChallengeCreate(args []string) error {
	fs := flag.NewFlagSet("challenge", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	directionID := fs.String("direction", "", "challenged direction event id")
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	proposal := fs.String("proposal", "", "proposed path")
	reason := fs.String("reason", "", "challenge reason")
	areas := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *directionID == "" {
		return errors.New("challenge requires --direction")
	}
	textArg := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if *proposal == "" {
		*proposal = textArg
	}
	if strings.TrimSpace(*proposal) == "" {
		return errors.New("challenge requires --proposal or text")
	}
	if *issue == "" && *pr == "" && len(areas) == 0 {
		return errors.New("challenge requires --issue, --pr, or --area")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	challenged, ok := findEvent(events, *directionID)
	if !ok {
		return fmt.Errorf("unknown direction %q", *directionID)
	}

	text := challengeText(*directionID, *proposal, *reason)
	event := DirectionEvent{
		Kind:       "challenge",
		CreatedAt:  nowString(),
		Durability: DurabilityLive,
		Scope: EventScope{
			Repo:  repoName(),
			Issue: *issue,
			PR:    *pr,
			Areas: areas,
		},
		Source:     EventSource{Type: "human"},
		Text:       text,
		Confidence: "human_confirmed",
		TTL:        "until_challenge_resolved",
		Challenges: *directionID,
		Status:     "open",
	}
	if err := appendEvent(&event); err != nil {
		return err
	}
	if err := writeFile(challengePath, challengeMarkdown(challenged, event, *proposal, *reason)); err != nil {
		return err
	}

	threads, err := loadThreads()
	if err != nil {
		return err
	}
	stale := staleThreads(event, threads)
	fmt.Printf("Recorded challenge %s against %s.\n", event.ID, *directionID)
	fmt.Printf("Wrote %s\n", challengePath)
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

func runChallengeResolve(args []string) error {
	challengeID := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		challengeID = args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("challenge resolve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	accepted := fs.Bool("accepted", false, "accept the challenge")
	rejected := fs.Bool("rejected", false, "reject the challenge")
	superseded := fs.Bool("superseded", false, "mark the challenge superseded")
	reason := fs.String("reason", "", "resolution reason")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if challengeID == "" && fs.NArg() == 1 {
		challengeID = fs.Arg(0)
	}
	if challengeID == "" || fs.NArg() > 1 {
		return errors.New("challenge resolve requires a challenge event id")
	}
	status, err := resolutionStatus(*accepted, *rejected, *superseded)
	if err != nil {
		return err
	}
	events, err := loadEvents()
	if err != nil {
		return err
	}
	challenge, ok := findEvent(events, challengeID)
	if !ok || challenge.Kind != "challenge" {
		return fmt.Errorf("unknown challenge %q", challengeID)
	}
	if resolution, ok := challengeResolution(events, challengeID); ok {
		return fmt.Errorf("challenge %s already resolved as %s by %s", challengeID, resolution.Status, resolution.ID)
	}

	text := resolutionText(challengeID, status, *reason)
	event := DirectionEvent{
		Kind:       "challenge_resolution",
		CreatedAt:  nowString(),
		Durability: DurabilityLive,
		Scope:      challenge.Scope,
		Source:     EventSource{Type: "human"},
		Text:       text,
		Confidence: "human_confirmed",
		TTL:        challengeResolutionTTL(challenge.Scope),
		Challenges: challengeID,
		Status:     status,
	}
	if err := appendEvent(&event); err != nil {
		return err
	}
	fmt.Printf("Recorded challenge resolution %s for %s: %s.\n", event.ID, challengeID, status)
	return nil
}

func findEvent(events []DirectionEvent, id string) (DirectionEvent, bool) {
	for _, event := range events {
		if event.ID == id {
			return event, true
		}
	}
	return DirectionEvent{}, false
}

func challengeText(directionID, proposal, reason string) string {
	text := "Challenge direction " + directionID + ": " + strings.TrimSpace(proposal)
	if strings.TrimSpace(reason) != "" {
		text += " Reason: " + strings.TrimSpace(reason)
	}
	return text
}

func challengeMarkdown(challenged, challenge DirectionEvent, proposal, reason string) string {
	if strings.TrimSpace(reason) == "" {
		reason = "(none provided)"
	}
	return fmt.Sprintf(`# Direction Challenge

This work intentionally challenges existing direction.

Challenged direction:
%s

Existing direction:
%s

Challenge:
%s

Reason:
%s

Requested outcome:
Approve a scoped exception, reject this challenge, or supersede the existing direction.

Instructions:
- Do not treat this as accidental drift.
- Mention this challenge in the PR or handoff.
- Do not promote this into durable project memory until resolved.
`, challenge.Challenges, challenged.Text, strings.TrimSpace(proposal), strings.TrimSpace(reason))
}

func resolutionStatus(accepted, rejected, superseded bool) (string, error) {
	count := 0
	status := ""
	if accepted {
		count++
		status = "accepted"
	}
	if rejected {
		count++
		status = "rejected"
	}
	if superseded {
		count++
		status = "superseded"
	}
	if count != 1 {
		return "", errors.New("challenge resolve requires exactly one of --accepted, --rejected, or --superseded")
	}
	return status, nil
}

func resolutionText(challengeID, status, reason string) string {
	text := fmt.Sprintf("Challenge %s %s.", challengeID, resolutionPhrase(status))
	if strings.TrimSpace(reason) != "" {
		text += " Reason: " + strings.TrimSpace(reason)
	}
	return text
}

func resolutionPhrase(status string) string {
	switch status {
	case "accepted":
		return "accepted as a scoped exception"
	case "rejected":
		return "rejected"
	case "superseded":
		return "superseded by newer direction"
	default:
		return status
	}
}

func challengeResolutionTTL(scope EventScope) string {
	if scope.PR != "" {
		return "until_pr_closed"
	}
	return "until_issue_closed"
}
