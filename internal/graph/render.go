package graph

import (
	"fmt"
	"strings"
)

// Render produces an ASCII tree representation of the dependency graph.
func Render(g *Graph) string {
	entries := g.EntryPoints()
	if len(entries) == 0 {
		// No clear entry points, use all modules
		entries = g.Modules
	}

	var sb strings.Builder
	visited := make(map[string]bool)

	for i, entry := range entries {
		if i > 0 {
			sb.WriteString("\n")
		}
		renderNode(&sb, g, entry, "", true, visited)
	}

	return sb.String()
}

func renderNode(sb *strings.Builder, g *Graph, node, prefix string, isLast bool, visited map[string]bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	if prefix == "" {
		// Root node
		sb.WriteString(node)
	} else {
		sb.WriteString(prefix)
		sb.WriteString(connector)
		sb.WriteString(node)
	}

	if visited[node] {
		sb.WriteString(" (cycle)")
		sb.WriteString("\n")
		return
	}
	sb.WriteString("\n")

	visited[node] = true
	deps := g.Dependencies[node]

	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, dep := range deps {
		last := i == len(deps)-1
		renderNode(sb, g, dep, childPrefix, last, visited)
	}

	delete(visited, node)
}

// RenderFlat produces a flat list of dependencies for each module.
func RenderFlat(g *Graph) string {
	var sb strings.Builder
	for _, mod := range g.Modules {
		deps := g.Dependencies[mod]
		if len(deps) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s -> %s\n", mod, strings.Join(deps, ", "))
	}
	return sb.String()
}
