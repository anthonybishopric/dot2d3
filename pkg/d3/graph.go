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
	ID          string            `json:"id"`
	Label       string            `json:"label,omitempty"`
	Color       string            `json:"color,omitempty"`     // Border/stroke color
	FillColor   string            `json:"fillColor,omitempty"` // Fill color
	Shape       string            `json:"shape,omitempty"`
	Style       string            `json:"style,omitempty"`
	Group       string            `json:"group,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	OnPath      bool              `json:"onPath,omitempty"`      // Node is part of highlighted path
	PathInvalid bool              `json:"pathInvalid,omitempty"` // Red highlight - last valid node before error
}

// Link represents an edge for D3 visualization.
type Link struct {
	Source     string            `json:"source"`
	Target     string            `json:"target"`
	Label      string            `json:"label,omitempty"`
	Color      string            `json:"color,omitempty"`
	Style      string            `json:"style,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	OnPath     bool              `json:"onPath,omitempty"` // Edge is part of highlighted path
}

// Subgraph represents subgraph grouping information.
type Subgraph struct {
	ID    string   `json:"id"`
	Label string   `json:"label,omitempty"`
	Nodes []string `json:"nodes"`
}

// PathValidationResult contains the result of validating a path against a graph.
type PathValidationResult struct {
	Valid         bool         `json:"valid"`
	Error         string       `json:"error,omitempty"`
	InvalidEdge   *InvalidEdge `json:"invalidEdge,omitempty"`
	LastValidNode string       `json:"lastValidNode,omitempty"`
}

// InvalidEdge describes an edge that failed validation.
type InvalidEdge struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	InvalidNode string `json:"invalidNode"`
}
