package core

import (
	"path"
	"sort"
	"strings"

	"github.com/lutefd/fabric/protocol"
)

type RelevanceContext struct {
	Issue string
	PR    string
	Areas []string
	Paths []string
}

type RankedRecord struct {
	Record  protocol.Record
	Reasons []protocol.MatchReason
	Tier    int
}

func Match(record protocol.Record, context RelevanceContext) RankedRecord {
	ranked := RankedRecord{Record: record, Tier: 100}
	if record.Scope.PR != "" && context.PR != "" && record.Scope.PR == context.PR {
		ranked.Tier = 10
		ranked.Reasons = append(ranked.Reasons, protocol.MatchReason{Kind: "pr", Value: record.Scope.PR})
	}
	if record.Scope.Issue != "" && context.Issue != "" && record.Scope.Issue == context.Issue {
		if ranked.Tier > 20 {
			ranked.Tier = 20
		}
		ranked.Reasons = append(ranked.Reasons, protocol.MatchReason{Kind: "issue", Value: record.Scope.Issue})
	}
	for _, pattern := range record.Scope.Paths {
		for _, value := range context.Paths {
			if PathMatches(pattern, value) {
				if ranked.Tier > 30 {
					ranked.Tier = 30
				}
				ranked.Reasons = append(ranked.Reasons, protocol.MatchReason{Kind: "path", Value: value})
			}
		}
	}
	for _, recordArea := range record.Scope.Areas {
		for _, contextArea := range context.Areas {
			if recordArea != "" && recordArea == contextArea {
				if ranked.Tier > 30 {
					ranked.Tier = 30
				}
				ranked.Reasons = append(ranked.Reasons, protocol.MatchReason{Kind: "area", Value: contextArea})
			}
		}
	}
	if record.Scope.Global {
		if ranked.Tier > 50 {
			ranked.Tier = 50
		}
		ranked.Reasons = append(ranked.Reasons, protocol.MatchReason{Kind: "global"})
	}
	return ranked
}

func Rank(records []protocol.Record, context RelevanceContext) []RankedRecord {
	var ranked []RankedRecord
	for _, record := range records {
		match := Match(record, context)
		if len(match.Reasons) > 0 {
			ranked = append(ranked, match)
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Tier != ranked[j].Tier {
			return ranked[i].Tier < ranked[j].Tier
		}
		leftKind, rightKind := kindPriority(ranked[i].Record.Kind), kindPriority(ranked[j].Record.Kind)
		if leftKind != rightKind {
			return leftKind < rightKind
		}
		if ranked[i].Record.CreatedAt != ranked[j].Record.CreatedAt {
			return ranked[i].Record.CreatedAt < ranked[j].Record.CreatedAt
		}
		return ranked[i].Record.RecordID < ranked[j].Record.RecordID
	})
	return ranked
}

func PathMatches(pattern, value string) bool {
	pattern = strings.TrimPrefix(strings.TrimSpace(pattern), "./")
	value = strings.TrimPrefix(strings.TrimSpace(value), "./")
	if pattern == "" || value == "" {
		return false
	}
	if strings.HasSuffix(pattern, "/**") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "**"))
	}
	matched, err := path.Match(pattern, value)
	return err == nil && matched
}

func kindPriority(kind string) int {
	switch kind {
	case "review_direction":
		return 1
	case "review_requirement":
		return 2
	case "decision":
		return 3
	case "finding":
		return 4
	default:
		return 5
	}
}
