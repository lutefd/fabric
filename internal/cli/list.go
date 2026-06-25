package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func runList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	durability := fs.String("durability", "", "live, candidate, or durable")
	status := fs.String("status", "active", "active, inactive, or any")
	issue := fs.String("issue", "", "issue key")
	pr := fs.String("pr", "", "pull request number")
	areas := stringListFlag{}
	paths := stringListFlag{}
	fs.Var(&areas, "area", "area, repeatable")
	fs.Var(&paths, "path", "repository path or glob, repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("list accepts filters only")
	}
	if *durability != "" && *durability != DurabilityLive && *durability != DurabilityCandidate && *durability != DurabilityDurable {
		return errors.New("--durability must be live, candidate, or durable")
	}
	if *status != "active" && *status != "inactive" && *status != "any" {
		return errors.New("--status must be active, inactive, or any")
	}

	events, err := loadEvents()
	if err != nil {
		return err
	}
	actionable := actionableRecordIDs(events)
	filtered := make([]DirectionEvent, 0, len(events))
	for _, event := range events {
		active := actionable[event.ID]
		if (*status == "active" && !active) || (*status == "inactive" && active) {
			continue
		}
		if *durability != "" && normalizeDurability(event.Durability) != *durability {
			continue
		}
		if (*issue != "" || *pr != "" || len(areas) != 0 || len(paths) != 0) &&
			!reasonForScope(event, *issue, *pr, areas, paths).matched() {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt == filtered[j].CreatedAt {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CreatedAt < filtered[j].CreatedAt
	})

	setMachineResult(map[string]any{"records": filtered, "count": len(filtered), "status": *status})
	if len(filtered) == 0 {
		fmt.Println("No matching direction.")
		return nil
	}
	for _, event := range filtered {
		fmt.Printf("%s  %s/%s  %s\n", event.ID, normalizeDurability(event.Durability), normalizeStatus(event.Status), event.Text)
		if scope := compactScope(event.Scope); scope != "" {
			fmt.Printf("  scope: %s\n", scope)
		}
	}
	return nil
}

func compactScope(scope EventScope) string {
	parts := []string{}
	if scope.Global {
		parts = append(parts, "global")
	}
	if scope.Issue != "" {
		parts = append(parts, "issue="+scope.Issue)
	}
	if scope.PR != "" {
		parts = append(parts, "pr="+scope.PR)
	}
	if len(scope.Areas) != 0 {
		parts = append(parts, "areas="+strings.Join(scope.Areas, ","))
	}
	if len(scope.Paths) != 0 {
		parts = append(parts, "paths="+strings.Join(scope.Paths, ","))
	}
	return strings.Join(parts, " ")
}
