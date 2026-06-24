package core

import (
	"sort"

	"github.com/lutefd/fabric/protocol"
)

func Traverse(root protocol.NodeRef, relations []protocol.Relation, direction string, allowed map[string]bool, depth int) protocol.Graph {
	if depth <= 0 {
		depth = 1
	}
	graph := protocol.Graph{Root: root}
	nodes := map[string]protocol.NodeRef{root.Key(): root}
	frontier := []protocol.NodeRef{root}
	seenRelations := map[string]bool{}
	for level := 0; level < depth && len(frontier) > 0; level++ {
		var next []protocol.NodeRef
		for _, node := range frontier {
			for _, relation := range relations {
				if len(allowed) > 0 && !allowed[relation.Type] {
					continue
				}
				outgoing := relation.From.Key() == node.Key()
				incoming := relation.To.Key() == node.Key()
				if direction == "outgoing" && !outgoing || direction == "incoming" && !incoming || direction == "both" && !outgoing && !incoming {
					continue
				}
				if seenRelations[relation.RelationID] {
					continue
				}
				seenRelations[relation.RelationID] = true
				graph.Relations = append(graph.Relations, relation)
				for _, candidate := range []protocol.NodeRef{relation.From, relation.To} {
					if _, ok := nodes[candidate.Key()]; !ok {
						nodes[candidate.Key()] = candidate
						next = append(next, candidate)
					}
				}
			}
		}
		frontier = next
	}
	for _, node := range nodes {
		graph.Nodes = append(graph.Nodes, node)
	}
	sort.Slice(graph.Nodes, func(i, j int) bool { return graph.Nodes[i].Key() < graph.Nodes[j].Key() })
	return graph
}
