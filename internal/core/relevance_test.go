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
