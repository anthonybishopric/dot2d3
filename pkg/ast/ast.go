// Package ast defines the abstract syntax tree for the DOT language.
package ast

import "github.com/anthonybishopric/gographviz/pkg/token"

// Node is the interface implemented by all AST nodes.
type Node interface {
	Pos() token.Position
}

// Statement is the interface for statement nodes.
type Statement interface {
	Node
	stmtNode()
}

// EdgeEndpoint represents something that can be an endpoint of an edge.
type EdgeEndpoint interface {
	Node
	edgeEndpointNode()
}

// Graph represents a complete DOT graph.
type Graph struct {
	Position   token.Position
	Strict     bool        // strict keyword present
	Directed   bool        // digraph vs graph
	ID         *Ident      // optional graph ID
	Statements []Statement // statements in the graph body
}

func (g *Graph) Pos() token.Position { return g.Position }

// Ident represents an identifier.
type Ident struct {
	Position token.Position
	Name     string
	Quoted   bool // was it a quoted string?
	HTML     bool // was it an HTML string?
}

func (i *Ident) Pos() token.Position { return i.Position }

// NodeID represents a node identifier with optional port.
type NodeID struct {
	Position token.Position
	ID       *Ident
	Port     *Port // optional
}

func (n *NodeID) Pos() token.Position      { return n.Position }
func (n *NodeID) edgeEndpointNode()        {}
func (n *NodeID) String() string           { return n.ID.Name }

// Port represents a port specification: :ID[:compass_pt]
type Port struct {
	Position token.Position
	ID       *Ident // port name
	Compass  *Ident // optional compass point (n, ne, e, se, s, sw, w, nw, c, _)
}

func (p *Port) Pos() token.Position { return p.Position }

// NodeStmt represents a node statement: ID [attr_list]
type NodeStmt struct {
	Position token.Position
	NodeID   *NodeID
	Attrs    *AttrList // optional
}

func (n *NodeStmt) Pos() token.Position { return n.Position }
func (n *NodeStmt) stmtNode()           {}

// EdgeStmt represents an edge statement.
type EdgeStmt struct {
	Position token.Position
	Left     EdgeEndpoint // first node/subgraph
	Rights   []EdgeRight  // subsequent edges
	Attrs    *AttrList    // optional
}

func (e *EdgeStmt) Pos() token.Position { return e.Position }
func (e *EdgeStmt) stmtNode()           {}

// EdgeRight represents the right side of an edge.
type EdgeRight struct {
	Position token.Position
	Directed bool         // true for ->, false for --
	Endpoint EdgeEndpoint // target node/subgraph
}

func (e *EdgeRight) Pos() token.Position { return e.Position }

// AttrStmt represents a default attribute statement: (graph|node|edge) attr_list
type AttrStmt struct {
	Position token.Position
	Kind     AttrKind
	Attrs    *AttrList
}

func (a *AttrStmt) Pos() token.Position { return a.Position }
func (a *AttrStmt) stmtNode()           {}

// AttrKind indicates the type of attribute statement.
type AttrKind int

const (
	GraphAttr AttrKind = iota
	NodeAttr
	EdgeAttr
)

func (k AttrKind) String() string {
	switch k {
	case GraphAttr:
		return "graph"
	case NodeAttr:
		return "node"
	case EdgeAttr:
		return "edge"
	default:
		return "unknown"
	}
}

// AttrAssign represents a top-level attribute assignment: ID = ID
type AttrAssign struct {
	Position token.Position
	Key      *Ident
	Value    *Ident
}

func (a *AttrAssign) Pos() token.Position { return a.Position }
func (a *AttrAssign) stmtNode()           {}

// AttrList represents a list of attributes: [attr1=val1, attr2=val2]
type AttrList struct {
	Position token.Position
	Attrs    []*Attr
}

func (a *AttrList) Pos() token.Position { return a.Position }

// Get returns the value for the given key, or empty string if not found.
func (a *AttrList) Get(key string) string {
	if a == nil {
		return ""
	}
	for _, attr := range a.Attrs {
		if attr.Key.Name == key {
			return attr.Value.Name
		}
	}
	return ""
}

// Attr represents a single attribute: ID = ID
type Attr struct {
	Position token.Position
	Key      *Ident
	Value    *Ident
}

func (a *Attr) Pos() token.Position { return a.Position }

// Subgraph represents a subgraph: subgraph [ID] { stmt_list }
type Subgraph struct {
	Position   token.Position
	ID         *Ident      // optional
	Statements []Statement // statements in the subgraph body
}

func (s *Subgraph) Pos() token.Position { return s.Position }
func (s *Subgraph) stmtNode()           {}
func (s *Subgraph) edgeEndpointNode()   {}

// NodeGroup represents edge shorthand: {A B C}
// Used during parsing and expanded into individual edges.
type NodeGroup struct {
	Position token.Position
	Nodes    []*NodeID
}

func (n *NodeGroup) Pos() token.Position { return n.Position }
func (n *NodeGroup) edgeEndpointNode()   {}
