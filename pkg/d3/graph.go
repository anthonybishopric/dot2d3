// Package d3 provides types and functions for generating D3.js visualizations.
package d3

// Graph represents a graph structure for D3 force simulation.
type Graph struct {
	Nodes     []Node     `json:"nodes"`
	Links     []Link     `json:"links"`
	Directed  bool       `json:"directed"`
	Strict    bool       `json:"strict,omitempty"`
	GraphID   string     `json:"graphId,omitempty"`
	Subgraphs []Subgraph `json:"subgraphs,omitempty"`
}

// Node represents a node for D3 visualization.
type Node struct {
	ID         string            `json:"id"`
	Label      string            `json:"label,omitempty"`
	Color      string            `json:"color,omitempty"`
	Shape      string            `json:"shape,omitempty"`
	Style      string            `json:"style,omitempty"`
	Group      string            `json:"group,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Link represents an edge for D3 visualization.
type Link struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Label      string            `json:"label,omitempty"`
	Color      string            `json:"color,omitempty"`
	Style      string            `json:"style,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Subgraph represents subgraph grouping information.
type Subgraph struct {
	ID    string   `json:"id"`
	Label string   `json:"label,omitempty"`
	Nodes []string `json:"nodes"`
}
