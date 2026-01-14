package parser

import (
	"testing"

	"github.com/anthonybishopric/gographviz/pkg/ast"
	"github.com/anthonybishopric/gographviz/pkg/lexer"
)

func TestParseSimpleGraph(t *testing.T) {
	input := `graph G { A -- B }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if g.Directed {
		t.Error("expected undirected graph")
	}

	if g.Strict {
		t.Error("expected non-strict graph")
	}

	if g.ID == nil || g.ID.Name != "G" {
		t.Errorf("expected graph ID 'G', got %v", g.ID)
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}
}

func TestParseDigraph(t *testing.T) {
	input := `digraph { A -> B -> C }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !g.Directed {
		t.Error("expected directed graph")
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}

	edge, ok := g.Statements[0].(*ast.EdgeStmt)
	if !ok {
		t.Fatalf("expected EdgeStmt, got %T", g.Statements[0])
	}

	if len(edge.Rights) != 2 {
		t.Errorf("expected 2 edge rights, got %d", len(edge.Rights))
	}
}

func TestParseStrictDigraph(t *testing.T) {
	input := `strict digraph G { A -> B }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !g.Strict {
		t.Error("expected strict graph")
	}

	if !g.Directed {
		t.Error("expected directed graph")
	}
}

func TestParseNodeAttributes(t *testing.T) {
	input := `digraph { A [label="Node A", color=red] }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}

	node, ok := g.Statements[0].(*ast.NodeStmt)
	if !ok {
		t.Fatalf("expected NodeStmt, got %T", g.Statements[0])
	}

	if node.NodeID.ID.Name != "A" {
		t.Errorf("expected node ID 'A', got %s", node.NodeID.ID.Name)
	}

	if node.Attrs == nil || len(node.Attrs.Attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %v", node.Attrs)
	}

	if node.Attrs.Get("label") != "Node A" {
		t.Errorf("expected label 'Node A', got %s", node.Attrs.Get("label"))
	}

	if node.Attrs.Get("color") != "red" {
		t.Errorf("expected color 'red', got %s", node.Attrs.Get("color"))
	}
}

func TestParseEdgeAttributes(t *testing.T) {
	input := `digraph { A -> B [label="connects", style=dashed] }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	edge, ok := g.Statements[0].(*ast.EdgeStmt)
	if !ok {
		t.Fatalf("expected EdgeStmt, got %T", g.Statements[0])
	}

	if edge.Attrs == nil || len(edge.Attrs.Attrs) != 2 {
		t.Fatalf("expected 2 attributes, got %v", edge.Attrs)
	}

	if edge.Attrs.Get("label") != "connects" {
		t.Errorf("expected label 'connects', got %s", edge.Attrs.Get("label"))
	}
}

func TestParseDefaultAttributes(t *testing.T) {
	input := `digraph { node [shape=box] edge [color=red] A -> B }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 3 {
		t.Fatalf("expected 3 statements, got %d", len(g.Statements))
	}

	nodeAttr, ok := g.Statements[0].(*ast.AttrStmt)
	if !ok {
		t.Fatalf("expected AttrStmt, got %T", g.Statements[0])
	}
	if nodeAttr.Kind != ast.NodeAttr {
		t.Errorf("expected NodeAttr, got %v", nodeAttr.Kind)
	}

	edgeAttr, ok := g.Statements[1].(*ast.AttrStmt)
	if !ok {
		t.Fatalf("expected AttrStmt, got %T", g.Statements[1])
	}
	if edgeAttr.Kind != ast.EdgeAttr {
		t.Errorf("expected EdgeAttr, got %v", edgeAttr.Kind)
	}
}

func TestParseSubgraph(t *testing.T) {
	input := `digraph { subgraph cluster_0 { A; B } }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}

	sub, ok := g.Statements[0].(*ast.Subgraph)
	if !ok {
		t.Fatalf("expected Subgraph, got %T", g.Statements[0])
	}

	if sub.ID == nil || sub.ID.Name != "cluster_0" {
		t.Errorf("expected subgraph ID 'cluster_0', got %v", sub.ID)
	}

	if len(sub.Statements) != 2 {
		t.Errorf("expected 2 statements in subgraph, got %d", len(sub.Statements))
	}
}

func TestParseAttributeAssignment(t *testing.T) {
	input := `digraph { label = "My Graph" }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}

	assign, ok := g.Statements[0].(*ast.AttrAssign)
	if !ok {
		t.Fatalf("expected AttrAssign, got %T", g.Statements[0])
	}

	if assign.Key.Name != "label" {
		t.Errorf("expected key 'label', got %s", assign.Key.Name)
	}

	if assign.Value.Name != "My Graph" {
		t.Errorf("expected value 'My Graph', got %s", assign.Value.Name)
	}
}

func TestParseEdgeShorthand(t *testing.T) {
	input := `digraph { A -> {B C D} }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(g.Statements))
	}

	edge, ok := g.Statements[0].(*ast.EdgeStmt)
	if !ok {
		t.Fatalf("expected EdgeStmt, got %T", g.Statements[0])
	}

	// Left should be A
	leftNode, ok := edge.Left.(*ast.NodeID)
	if !ok {
		t.Fatalf("expected NodeID, got %T", edge.Left)
	}
	if leftNode.ID.Name != "A" {
		t.Errorf("expected left node 'A', got %s", leftNode.ID.Name)
	}

	// Right should be a node group with B, C, D
	if len(edge.Rights) != 1 {
		t.Fatalf("expected 1 edge right, got %d", len(edge.Rights))
	}

	group, ok := edge.Rights[0].Endpoint.(*ast.NodeGroup)
	if !ok {
		t.Fatalf("expected NodeGroup, got %T", edge.Rights[0].Endpoint)
	}

	if len(group.Nodes) != 3 {
		t.Errorf("expected 3 nodes in group, got %d", len(group.Nodes))
	}
}

func TestParseComments(t *testing.T) {
	input := `
	// Line comment
	digraph {
		/* Block
		   comment */
		A -> B
		# Preprocessor line
		B -> C
	}
	`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Statements) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(g.Statements))
	}
}

func TestParseHTMLLabel(t *testing.T) {
	input := `digraph { A [label=<<b>Bold</b>>] }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	node, ok := g.Statements[0].(*ast.NodeStmt)
	if !ok {
		t.Fatalf("expected NodeStmt, got %T", g.Statements[0])
	}

	label := node.Attrs.Get("label")
	if label != "<b>Bold</b>" {
		t.Errorf("expected HTML label '<b>Bold</b>', got %s", label)
	}
}

func TestParseCaseInsensitiveKeywords(t *testing.T) {
	input := `DIGRAPH { NODE [shape=box] A -> B }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !g.Directed {
		t.Error("expected directed graph")
	}
}

func TestParsePort(t *testing.T) {
	input := `digraph { A:port1 -> B:port2:n }`

	l := lexer.New("test", []byte(input))
	p := New(l)
	g, err := p.Parse()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	edge, ok := g.Statements[0].(*ast.EdgeStmt)
	if !ok {
		t.Fatalf("expected EdgeStmt, got %T", g.Statements[0])
	}

	leftNode, ok := edge.Left.(*ast.NodeID)
	if !ok {
		t.Fatalf("expected NodeID, got %T", edge.Left)
	}

	if leftNode.Port == nil {
		t.Fatal("expected port on left node")
	}

	if leftNode.Port.ID.Name != "port1" {
		t.Errorf("expected port 'port1', got %s", leftNode.Port.ID.Name)
	}
}
