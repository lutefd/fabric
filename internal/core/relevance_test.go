package core

import (
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestRankUsesDeterministicScopeTiers(t *testing.T) {
	records := []protocol.Record{
		{RecordID: "global", Kind: "note", Scope: protocol.Scope{Global: true}},
		{RecordID: "area", Kind: "note", Scope: protocol.Scope{Areas: []string{"api"}}},
		{RecordID: "path", Kind: "note", Scope: protocol.Scope{Paths: []string{"internal/**"}}},
		{RecordID: "issue", Kind: "note", Scope: protocol.Scope{Issue: "FAB-1"}},
		{RecordID: "pr", Kind: "note", Scope: protocol.Scope{PR: "7"}},
	}
	ranked := Rank(records, RelevanceContext{Issue: "FAB-1", PR: "7", Areas: []string{"api"}, Paths: []string{"internal/core/a.go"}})
	want := []string{"pr", "issue", "area", "path", "global"}
	for i, id := range want {
		if ranked[i].Record.RecordID != id {
			t.Fatalf("rank %d = %s, want %s", i, ranked[i].Record.RecordID, id)
		}
	}
}

func TestRankTieBreaksByKindCreatedAtAndID(t *testing.T) {
	records := []protocol.Record{
		{RecordID: "z", Kind: "finding", CreatedAt: "2026-06-24T18:00:02Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "b", Kind: "review_requirement", CreatedAt: "2026-06-24T18:00:01Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "a", Kind: "review_requirement", CreatedAt: "2026-06-24T18:00:01Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "c", Kind: "decision", CreatedAt: "2026-06-24T18:00:00Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "d", Kind: "review_direction", CreatedAt: "2026-06-24T18:00:03Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "e", Kind: "unknown", CreatedAt: "2026-06-24T18:00:04Z", Scope: protocol.Scope{Global: true}},
	}
	ranked := Rank(records, RelevanceContext{})
	want := []string{"d", "a", "b", "c", "z", "e"}
	for i, id := range want {
		if ranked[i].Record.RecordID != id {
			t.Fatalf("rank %d = %s, want %s", i, ranked[i].Record.RecordID, id)
		}
	}
}

func TestRankTieBreaksByCreatedAt(t *testing.T) {
	records := []protocol.Record{
		{RecordID: "late", Kind: "finding", CreatedAt: "2026-06-24T18:00:02Z", Scope: protocol.Scope{Global: true}},
		{RecordID: "early", Kind: "finding", CreatedAt: "2026-06-24T18:00:01Z", Scope: protocol.Scope{Global: true}},
	}
	ranked := Rank(records, RelevanceContext{})
	if ranked[0].Record.RecordID != "early" {
		t.Fatalf("first record = %s, want early", ranked[0].Record.RecordID)
	}
}

func TestPathMatchesTrimsAndRejectsInvalidPatterns(t *testing.T) {
	if !PathMatches(" ./internal/**", "./internal/core/model.go") {
		t.Fatal("recursive path pattern did not match")
	}
	if !PathMatches("*.go", "main.go") {
		t.Fatal("glob pattern did not match")
	}
	if PathMatches("", "main.go") || PathMatches("[", "main.go") {
		t.Fatal("empty or invalid pattern matched")
	}
}
