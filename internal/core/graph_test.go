package core

import (
	"testing"

	"github.com/lutefd/fabric/protocol"
)

func TestTraverseHonorsDirectionDepthAndAllowedTypes(t *testing.T) {
	root := protocol.NodeRef{Kind: "record", ID: "root"}
	mid := protocol.NodeRef{Kind: "record", ID: "mid"}
	leaf := protocol.NodeRef{Kind: "record", ID: "leaf"}
	relations := []protocol.Relation{
		{RelationID: "rel-1", Type: protocol.RelationInformedBy, From: root, To: mid},
		{RelationID: "rel-2", Type: protocol.RelationSupersedes, From: mid, To: leaf},
		{RelationID: "rel-3", Type: protocol.RelationChallenges, From: leaf, To: root},
	}

	graph := Traverse(root, relations, "outgoing", map[string]bool{protocol.RelationInformedBy: true}, 3)
	if len(graph.Relations) != 1 || len(graph.Nodes) != 2 {
		t.Fatalf("filtered graph = %#v", graph)
	}

	graph = Traverse(root, relations, "both", nil, 2)
	if len(graph.Relations) != 3 || len(graph.Nodes) != 3 {
		t.Fatalf("bidirectional graph = %#v", graph)
	}

	graph = Traverse(root, relations, "incoming", nil, 0)
	if len(graph.Relations) != 1 || graph.Relations[0].RelationID != "rel-3" {
		t.Fatalf("incoming default-depth graph = %#v", graph)
	}
}
