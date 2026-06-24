package fabric

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

const consolidationPath = ".fabric/generated/CONSOLIDATION.md"

func runExpire(args []string) error {
	fs := flag.NewFlagSet("expire", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	reason := fs.String("reason", "", "reason for expiration")
	force := fs.Bool("force", false, "force expiration of durable direction")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("expected: fabric expire <event-id>")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	eventID := fs.Arg(0)
	events, err := loadEvents()
	if err != nil {
		return err
	}
	event, ok := findEvent(events, eventID)
	if !ok {
		return fmt.Errorf("event %s not found", eventID)
	}
	if event.Durability == DurabilityDurable && !*force {
		return fmt.Errorf("event %s is durable; use --force to expire it", eventID)
	}
	updated, err := updateEvent(eventID, func(e *DirectionEvent) error {
		e.Status = StatusExpired
		return nil
	}, *reason)
	if err != nil {
		return err
	}
	fmt.Printf("Expired %s.\n", updated.ID)
	if *reason != "" {
		fmt.Printf("Reason: %s\n", *reason)
	}
	return nil
}

func runDiscard(args []string) error {
	fs := flag.NewFlagSet("discard", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	reason := fs.String("reason", "", "reason for discard")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("expected: fabric discard <event-id>")
	}
	if *reason == "" {
		return errors.New("discard requires --reason")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	eventID := fs.Arg(0)
	updated, err := updateEvent(eventID, func(e *DirectionEvent) error {
		e.Status = StatusDiscarded
		return nil
	}, *reason)
	if err != nil {
		return err
	}
	fmt.Printf("Discarded %s.\n", updated.ID)
	fmt.Printf("Reason: %s\n", *reason)
	return nil
}

func runKeep(args []string) error {
	fs := flag.NewFlagSet("keep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	candidate := fs.Bool("candidate", false, "keep as candidate")
	reason := fs.String("reason", "", "reason for keeping")
	if err := fs.Parse(flagsFirst(args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("expected: fabric keep <event-id>")
	}
	if !*candidate {
		return errors.New("keep requires --candidate")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}
	eventID := fs.Arg(0)
	updated, err := updateEvent(eventID, func(e *DirectionEvent) error {
		e.Durability = DurabilityCandidate
		e.Status = StatusActive
		return nil
	}, *reason)
	if err != nil {
		return err
	}
	fmt.Printf("Kept %s as candidate.\n", updated.ID)
	return nil
}

type consolidationScope struct {
	PR     string
	Issue  string
	Areas  []string
	IsPR   bool
	IsIssue bool
}

func runConsolidate(args []string) error {
	fs := flag.NewFlagSet("consolidate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pr := fs.String("pr", "", "pull request number")
	issue := fs.String("issue", "", "issue key")
	budget := fs.Int("budget", 1200, "token budget")
	includeInactive := fs.Bool("include-inactive", false, "include expired/discarded/superseded events")
	includeExpired := fs.Bool("include-expired", false, "alias for --include-inactive")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if (*pr == "" && *issue == "") || (*pr != "" && *issue != "") {
		return errors.New("consolidate requires exactly one of --pr or --issue")
	}
	if err := ensureInitialized(); err != nil {
		return err
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}

	scope := consolidationScope{IsPR: *pr != "", IsIssue: *issue != ""}
	if scope.IsPR {
		scope.PR = *pr
		scope.Issue, scope.Areas = inferScopeFromPR(events, *pr, "", nil)
	} else {
		scope.Issue = *issue
	}

	includeInactive = boolPtr(*includeInactive || *includeExpired)
	_ = budget

	matched := collectConsolidationEvents(events, scope, *includeInactive)
	report := buildConsolidationReport(scope, matched)
	markdown := consolidationMarkdown(report)
	if err := writeFile(consolidationPath, markdown); err != nil {
		return err
	}
	fmt.Print(summaryForConsolidation(report))
	return nil
}

func boolPtr(b bool) *bool {
	return &b
}

func collectConsolidationEvents(events []DirectionEvent, scope consolidationScope, includeInactive bool) []DirectionEvent {
	var matched []DirectionEvent
	for _, event := range events {
		if !matchesConsolidationScope(event, scope) {
			continue
		}
		if !includeInactive && !isActiveEvent(event) {
			continue
		}
		matched = append(matched, event)
	}
	return matched
}

func matchesConsolidationScope(event DirectionEvent, scope consolidationScope) bool {
	if scope.IsPR {
		if event.Scope.PR == scope.PR || event.Source.PR == scope.PR {
			return true
		}
		if scope.Issue != "" && event.Scope.Issue == scope.Issue {
			return true
		}
		return false
	}
	return event.Scope.Issue == scope.Issue
}

type consolidationEvent struct {
	Event          DirectionEvent
	SuggestedAction string
	WhyTemporary   []string
	WhyDurable     []string
}

type consolidationReport struct {
	Scope           consolidationScope
	DurableActive   []DirectionEvent
	CandidateActive []consolidationEvent
	LiveActive      []consolidationEvent
	OpenChallenges  []DirectionEvent
	Inactive        []DirectionEvent
}

func buildConsolidationReport(scope consolidationScope, events []DirectionEvent) consolidationReport {
	report := consolidationReport{Scope: scope}
	resolutions := resolutionsByChallenge(events)
	for _, event := range events {
		switch event.Durability {
		case DurabilityDurable:
			if isActiveEvent(event) {
				report.DurableActive = append(report.DurableActive, event)
			} else {
				report.Inactive = append(report.Inactive, event)
			}
		case DurabilityCandidate:
			if isActiveEvent(event) {
				report.CandidateActive = append(report.CandidateActive, suggestForCandidate(event))
			} else {
				report.Inactive = append(report.Inactive, event)
			}
		case DurabilityLive:
			if isActiveEvent(event) {
				if isOpenChallenge(event, resolutions) {
					report.OpenChallenges = append(report.OpenChallenges, event)
				} else {
					report.LiveActive = append(report.LiveActive, suggestForLive(event))
				}
			} else {
				report.Inactive = append(report.Inactive, event)
			}
		}
	}
	return report
}

func suggestForCandidate(event DirectionEvent) consolidationEvent {
	ce := consolidationEvent{Event: event}
	switch event.Kind {
	case "review_direction":
		if event.ReviewType == "rejection" || event.ReviewType == "preference" || len(event.RejectedPaths) > 0 || len(event.PreferredPaths) > 0 {
			ce.SuggestedAction = fmt.Sprintf("fabric promote %s", event.ID)
			ce.WhyDurable = append(ce.WhyDurable, "It captures rejected or preferred paths that future agents should not reopen.")
		} else {
			ce.SuggestedAction = fmt.Sprintf("fabric promote %s", event.ID)
		}
	case "review_requirement":
		ce.SuggestedAction = fmt.Sprintf("fabric expire %s", event.ID)
		ce.WhyTemporary = append(ce.WhyTemporary, "It is a PR-local checklist or test item.")
	default:
		if isRequirementLike(event) {
			ce.SuggestedAction = fmt.Sprintf("fabric expire %s", event.ID)
		} else {
			ce.SuggestedAction = fmt.Sprintf("fabric promote %s", event.ID)
		}
	}
	if ce.SuggestedAction == "" {
		ce.SuggestedAction = fmt.Sprintf("fabric keep %s --candidate", event.ID)
	}
	return ce
}

func suggestForLive(event DirectionEvent) consolidationEvent {
	ce := consolidationEvent{Event: event}
	switch event.Kind {
	case "review_direction":
		ce.SuggestedAction = fmt.Sprintf("fabric keep %s --candidate", event.ID)
		ce.WhyDurable = append(ce.WhyDurable, "It contains direction with rejected/preferred paths that may matter later.")
	case "review_requirement":
		ce.SuggestedAction = fmt.Sprintf("fabric expire %s", event.ID)
		ce.WhyTemporary = append(ce.WhyTemporary, "It is a PR-local checklist or test item.")
		ce.WhyTemporary = append(ce.WhyTemporary, "The implementation is likely complete.")
	default:
		if isRequirementLike(event) {
			ce.SuggestedAction = fmt.Sprintf("fabric expire %s", event.ID)
			ce.WhyTemporary = append(ce.WhyTemporary, "It is a task-local checklist item.")
		} else {
			ce.SuggestedAction = fmt.Sprintf("fabric expire %s", event.ID)
		}
	}
	return ce
}

func isRequirementLike(event DirectionEvent) bool {
	lower := strings.ToLower(event.Text)
	return strings.Contains(lower, "test") ||
		strings.Contains(lower, "checklist") ||
		strings.Contains(lower, "add tests") ||
		event.Kind == "review_requirement"
}

func consolidationMarkdown(report consolidationReport) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Fabric Consolidation")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Target:")
	if report.Scope.IsPR {
		fmt.Fprintf(&b, "PR %s\n", report.Scope.PR)
	} else {
		fmt.Fprintf(&b, "Issue %s\n", report.Scope.Issue)
	}
	fmt.Fprintln(&b)

	writeEventList(&b, "Durable directions already kept", report.DurableActive, func(e DirectionEvent) string {
		return fmt.Sprintf("%s\n   %s", e.ID, e.Text)
	})

	writeCandidateList(&b, "Candidate directions to review", report.CandidateActive)
	writeLiveList(&b, "Live directions likely to expire", report.LiveActive)
	writeEventList(&b, "Open challenges", report.OpenChallenges, func(e DirectionEvent) string {
		return fmt.Sprintf("%s\n   %s", e.ID, e.Text)
	})
	writeEventList(&b, "Inactive events", report.Inactive, func(e DirectionEvent) string {
		return fmt.Sprintf("%s (%s/%s)\n   %s", e.ID, e.Durability, normalizeStatus(e.Status), e.Text)
	})

	fmt.Fprintln(&b, "## Suggested cleanup")
	fmt.Fprintln(&b)
	var actions []string
	for _, ce := range report.CandidateActive {
		actions = append(actions, ce.SuggestedAction)
	}
	for _, ce := range report.LiveActive {
		actions = append(actions, ce.SuggestedAction)
	}
	for _, e := range report.OpenChallenges {
		actions = append(actions, fmt.Sprintf("resolve challenge %s before consolidating as durable", e.ID))
	}
	if len(actions) == 0 {
		fmt.Fprintln(&b, "No cleanup actions suggested.")
	} else {
		for _, action := range actions {
			fmt.Fprintf(&b, "%s\n", action)
		}
	}
	return b.String()
}

func writeEventList(b *strings.Builder, title string, events []DirectionEvent, format func(DirectionEvent) string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(events) == 0 {
		fmt.Fprintln(b, "None.")
	} else {
		for i, event := range events {
			fmt.Fprintf(b, "%d. %s\n", i+1, format(event))
		}
	}
	fmt.Fprintln(b)
}

func writeCandidateList(b *strings.Builder, title string, items []consolidationEvent) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(items) == 0 {
		fmt.Fprintln(b, "None.")
	} else {
		for i, item := range items {
			fmt.Fprintf(b, "%d. %s\n   %s\n", i+1, item.Event.ID, item.Event.Text)
			if len(item.WhyDurable) > 0 {
				fmt.Fprintln(b, "\n   Why it may be durable:")
				for _, reason := range item.WhyDurable {
					fmt.Fprintf(b, "   - %s\n", reason)
				}
			}
			fmt.Fprintf(b, "\n   Suggested action:\n   %s\n", item.SuggestedAction)
		}
	}
	fmt.Fprintln(b)
}

func writeLiveList(b *strings.Builder, title string, items []consolidationEvent) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(items) == 0 {
		fmt.Fprintln(b, "None.")
	} else {
		for i, item := range items {
			fmt.Fprintf(b, "%d. %s\n   %s\n", i+1, item.Event.ID, item.Event.Text)
			if len(item.WhyTemporary) > 0 {
				fmt.Fprintln(b, "\n   Why likely temporary:")
				for _, reason := range item.WhyTemporary {
					fmt.Fprintf(b, "   - %s\n", reason)
				}
			}
			fmt.Fprintf(b, "\n   Suggested action:\n   %s\n", item.SuggestedAction)
		}
	}
	fmt.Fprintln(b)
}

func summaryForConsolidation(report consolidationReport) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Fabric Consolidation")
	if report.Scope.IsPR {
		fmt.Fprintf(&b, "PR: %s\n", report.Scope.PR)
	} else {
		fmt.Fprintf(&b, "Issue: %s\n", report.Scope.Issue)
	}
	fmt.Fprintf(&b, "- Durable active: %d\n", len(report.DurableActive))
	fmt.Fprintf(&b, "- Candidate active: %d\n", len(report.CandidateActive))
	fmt.Fprintf(&b, "- Live active: %d\n", len(report.LiveActive))
	fmt.Fprintf(&b, "- Open challenges: %d\n", len(report.OpenChallenges))
	fmt.Fprintf(&b, "- Inactive: %d\n", len(report.Inactive))
	fmt.Fprintf(&b, "Wrote %s\n", consolidationPath)
	return b.String()
}
