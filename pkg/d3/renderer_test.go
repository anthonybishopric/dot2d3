package d3

import (
	"encoding/json"
	"testing"

	"github.com/anthonybishopric/dot2d3/pkg/ast"
	"github.com/anthonybishopric/dot2d3/pkg/lexer"
	"github.com/anthonybishopric/dot2d3/pkg/parser"
)

func parse(t *testing.T, input string) *ast.Graph {
	t.Helper()
	l := lexer.New("test", []byte(input))
	p := parser.New(l)
	g, err := p.Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return g
}

func TestConvertSimpleDigraph(t *testing.T) {
	g := parse(t, `digraph G { A -> B -> C }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if !d3g.Directed {
		t.Error("expected directed graph")
	}

	if d3g.GraphID != "G" {
		t.Errorf("expected graph ID 'G', got %s", d3g.GraphID)
	}

	if len(d3g.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(d3g.Nodes))
	}

	if len(d3g.Links) != 2 {
		t.Errorf("expected 2 links, got %d", len(d3g.Links))
	}

	// Verify nodes exist
	nodeMap := make(map[string]bool)
	for _, n := range d3g.Nodes {
		nodeMap[n.ID] = true
	}
	for _, id := range []string{"A", "B", "C"} {
		if !nodeMap[id] {
			t.Errorf("missing node %s", id)
		}
	}
}

func TestConvertNodeAttributes(t *testing.T) {
	g := parse(t, `digraph { A [label="Node A", color=red, shape=box] }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if len(d3g.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(d3g.Nodes))
	}

	node := d3g.Nodes[0]
	if node.ID != "A" {
		t.Errorf("expected ID 'A', got %s", node.ID)
	}
	if node.Label != "Node A" {
		t.Errorf("expected label 'Node A', got %s", node.Label)
	}
	if node.Color != "red" {
		t.Errorf("expected color 'red', got %s", node.Color)
	}
	if node.Shape != "box" {
		t.Errorf("expected shape 'box', got %s", node.Shape)
	}
}

func TestConvertEdgeAttributes(t *testing.T) {
	g := parse(t, `digraph { A -> B [label="connects", color=blue] }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if len(d3g.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(d3g.Links))
	}

	link := d3g.Links[0]
	if link.Source != "A" {
		t.Errorf("expected source 'A', got %s", link.Source)
	}
	if link.Target != "B" {
		t.Errorf("expected target 'B', got %s", link.Target)
	}
	if link.Label != "connects" {
		t.Errorf("expected label 'connects', got %s", link.Label)
	}
	if link.Color != "blue" {
		t.Errorf("expected color 'blue', got %s", link.Color)
	}
}

func TestConvertDefaultAttributes(t *testing.T) {
	g := parse(t, `digraph { node [color=red] edge [color=blue] A -> B }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	// Both nodes should have default color
	for _, node := range d3g.Nodes {
		if node.Color != "red" {
			t.Errorf("expected node color 'red', got %s", node.Color)
		}
	}

	// Edge should have default color
	if len(d3g.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(d3g.Links))
	}
	if d3g.Links[0].Color != "blue" {
		t.Errorf("expected link color 'blue', got %s", d3g.Links[0].Color)
	}
}

func TestConvertEdgeShorthand(t *testing.T) {
	g := parse(t, `digraph { A -> {B C D} }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	// Should have 4 nodes: A, B, C, D
	if len(d3g.Nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(d3g.Nodes))
	}

	// Should have 3 links: A->B, A->C, A->D
	if len(d3g.Links) != 3 {
		t.Errorf("expected 3 links, got %d", len(d3g.Links))
	}

	// All links should start from A
	for _, link := range d3g.Links {
		if link.Source != "A" {
			t.Errorf("expected source 'A', got %s", link.Source)
		}
	}
}

func TestConvertStrict(t *testing.T) {
	g := parse(t, `strict digraph { A -> B; A -> B }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if !d3g.Strict {
		t.Error("expected strict graph")
	}

	// Strict should prevent duplicate edges
	if len(d3g.Links) != 1 {
		t.Errorf("expected 1 link (strict mode), got %d", len(d3g.Links))
	}
}

func TestConvertUndirectedGraph(t *testing.T) {
	g := parse(t, `graph { A -- B }`)

	d3g, err := Convert(g)
	if err != nil {
		t.Fatalf("convert error: %v", err)
	}

	if d3g.Directed {
		t.Error("expected undirected graph")
	}
}

func TestRenderHTML(t *testing.T) {
	d3g := &Graph{
		Nodes: []Node{
			{ID: "A", Label: "Node A"},
			{ID: "B", Label: "Node B"},
		},
		Links: []Link{
			{Source: "A", Target: "B"},
		},
		Directed: true,
	}

	opts := RenderOptions{
		Title: "Test Graph",
	}

	html, err := RenderHTML(d3g, opts)
	if err != nil {
		t.Fatalf("render error: %v", err)
	}

	htmlStr := string(html)

	// Check for key elements
	if !contains(htmlStr, "<title>Test Graph</title>") {
		t.Error("expected title in HTML")
	}

	if !contains(htmlStr, "d3js.org") {
		t.Error("expected D3.js reference in HTML")
	}

	if !contains(htmlStr, "Node A") {
		t.Error("expected node label in HTML")
	}

	if !contains(htmlStr, "nodeClick") {
		t.Error("expected nodeClick event handler in HTML")
	}

	if !contains(htmlStr, "d3.drag") {
		t.Error("expected drag behavior in HTML")
	}

	if !contains(htmlStr, "d3.zoom") {
		t.Error("expected zoom behavior in HTML")
	}
}

func TestJSONOutput(t *testing.T) {
	d3g := &Graph{
		Nodes: []Node{
			{ID: "A", Label: "Node A", Color: "red"},
		},
		Links:    []Link{},
		Directed: true,
	}

	jsonBytes, err := json.Marshal(d3g)
	if err != nil {
		t.Fatalf("json error: %v", err)
	}

	// Parse it back
	var parsed Graph
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(parsed.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(parsed.Nodes))
	}

	if parsed.Nodes[0].Label != "Node A" {
		t.Errorf("expected label 'Node A', got %s", parsed.Nodes[0].Label)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
