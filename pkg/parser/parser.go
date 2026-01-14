// Package parser implements a parser for the DOT language.
package parser

import (
	"fmt"
	"strings"

	"github.com/anthonybishopric/gographviz/pkg/ast"
	"github.com/anthonybishopric/gographviz/pkg/lexer"
	"github.com/anthonybishopric/gographviz/pkg/token"
)

// Parser parses DOT source code into an AST.
type Parser struct {
	lexer *lexer.Lexer

	// Current token
	pos token.Position
	tok token.Token
	lit string

	// Lookahead token
	peekPos token.Position
	peekTok token.Token
	peekLit string

	Errors []Error
}

// Error represents a parser error.
type Error struct {
	Pos token.Position
	Msg string
}

func (e Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

// New creates a new Parser for the given lexer.
func New(l *lexer.Lexer) *Parser {
	p := &Parser{lexer: l}
	// Initialize current and peek tokens
	p.next()
	p.next()
	return p
}

func (p *Parser) next() {
	p.pos = p.peekPos
	p.tok = p.peekTok
	p.lit = p.peekLit
	p.peekPos, p.peekTok, p.peekLit = p.lexer.Scan()
}

func (p *Parser) error(pos token.Position, msg string) {
	p.Errors = append(p.Errors, Error{Pos: pos, Msg: msg})
}

func (p *Parser) errorf(pos token.Position, format string, args ...interface{}) {
	p.error(pos, fmt.Sprintf(format, args...))
}

func (p *Parser) expect(tok token.Token) token.Position {
	pos := p.pos
	if p.tok != tok {
		p.errorf(p.pos, "expected %s, got %s", tok, p.tok)
	}
	p.next()
	return pos
}

// isID returns true if the current token can be an ID.
func (p *Parser) isID() bool {
	return p.tok == token.IDENT || p.tok == token.STRING || p.tok == token.HTML
}

// Parse parses a complete DOT graph.
func (p *Parser) Parse() (*ast.Graph, error) {
	g := p.parseGraph()

	// Collect all errors
	var allErrors []error
	for _, e := range p.lexer.Errors {
		allErrors = append(allErrors, e)
	}
	for _, e := range p.Errors {
		allErrors = append(allErrors, e)
	}

	if len(allErrors) > 0 {
		var msgs []string
		for _, e := range allErrors {
			msgs = append(msgs, e.Error())
		}
		return g, fmt.Errorf("parse errors:\n%s", strings.Join(msgs, "\n"))
	}

	return g, nil
}

// parseGraph parses: [ 'strict' ] ('graph' | 'digraph') [ ID ] '{' stmt_list '}'
func (p *Parser) parseGraph() *ast.Graph {
	g := &ast.Graph{Position: p.pos}

	// Optional 'strict'
	if p.tok == token.STRICT {
		g.Strict = true
		p.next()
	}

	// 'graph' or 'digraph'
	if p.tok == token.GRAPH {
		g.Directed = false
		p.next()
	} else if p.tok == token.DIGRAPH {
		g.Directed = true
		p.next()
	} else {
		p.errorf(p.pos, "expected 'graph' or 'digraph', got %s", p.tok)
		return g
	}

	// Optional ID
	if p.isID() {
		g.ID = p.parseIdent()
	}

	// '{' stmt_list '}'
	p.expect(token.LBRACE)
	g.Statements = p.parseStmtList()
	p.expect(token.RBRACE)

	return g
}

// parseStmtList parses: [ stmt [ ';' ] stmt_list ]
func (p *Parser) parseStmtList() []Statement {
	var stmts []Statement

	for p.tok != token.RBRACE && p.tok != token.EOF {
		stmt := p.parseStmt()
		if stmt != nil {
			stmts = append(stmts, stmt)
		}
		// Optional semicolon
		if p.tok == token.SEMICOLON {
			p.next()
		}
	}

	return stmts
}

type Statement = ast.Statement

// parseStmt parses: node_stmt | edge_stmt | attr_stmt | ID '=' ID | subgraph
func (p *Parser) parseStmt() Statement {
	switch p.tok {
	case token.GRAPH:
		// attr_stmt: graph attr_list
		return p.parseAttrStmt(ast.GraphAttr)
	case token.NODE:
		// attr_stmt: node attr_list
		return p.parseAttrStmt(ast.NodeAttr)
	case token.EDGE:
		// attr_stmt: edge attr_list
		return p.parseAttrStmt(ast.EdgeAttr)
	case token.SUBGRAPH, token.LBRACE:
		// subgraph
		sub := p.parseSubgraph()
		// Check if this is actually an edge statement
		if p.tok == token.ARROW || p.tok == token.DASHDASH {
			return p.parseEdgeStmt(sub)
		}
		return sub
	case token.IDENT, token.STRING, token.HTML:
		// Could be: node_stmt, edge_stmt, or ID '=' ID
		return p.parseIDStmt()
	default:
		p.errorf(p.pos, "unexpected token %s in statement", p.tok)
		p.next()
		return nil
	}
}

// parseIDStmt handles statements starting with an ID.
// Could be: node_stmt, edge_stmt, or ID '=' ID
func (p *Parser) parseIDStmt() Statement {
	// Parse the first ID
	id := p.parseIdent()
	pos := id.Position

	// Check for '=' (attribute assignment)
	if p.tok == token.EQUAL {
		p.next()
		if !p.isID() {
			p.errorf(p.pos, "expected identifier after '='")
			return nil
		}
		value := p.parseIdent()
		return &ast.AttrAssign{
			Position: pos,
			Key:      id,
			Value:    value,
		}
	}

	// Parse as NodeID (with optional port)
	nodeID := &ast.NodeID{
		Position: pos,
		ID:       id,
	}
	if p.tok == token.COLON {
		nodeID.Port = p.parsePort()
	}

	// Check for edge operators
	if p.tok == token.ARROW || p.tok == token.DASHDASH {
		return p.parseEdgeStmt(nodeID)
	}

	// Must be a node statement
	var attrs *ast.AttrList
	if p.tok == token.LBRACKET {
		attrs = p.parseAttrList()
	}

	return &ast.NodeStmt{
		Position: pos,
		NodeID:   nodeID,
		Attrs:    attrs,
	}
}

// parseAttrStmt parses: (graph | node | edge) attr_list
func (p *Parser) parseAttrStmt(kind ast.AttrKind) *ast.AttrStmt {
	pos := p.pos
	p.next() // consume keyword

	var attrs *ast.AttrList
	if p.tok == token.LBRACKET {
		attrs = p.parseAttrList()
	}

	return &ast.AttrStmt{
		Position: pos,
		Kind:     kind,
		Attrs:    attrs,
	}
}

// parseEdgeStmt parses an edge statement given the left endpoint.
func (p *Parser) parseEdgeStmt(left ast.EdgeEndpoint) *ast.EdgeStmt {
	stmt := &ast.EdgeStmt{
		Position: left.Pos(),
		Left:     left,
	}

	// Parse edge RHS chain
	for p.tok == token.ARROW || p.tok == token.DASHDASH {
		directed := p.tok == token.ARROW
		p.next()

		var endpoint ast.EdgeEndpoint
		if p.tok == token.SUBGRAPH || p.tok == token.LBRACE {
			endpoint = p.parseSubgraphOrGroup()
		} else if p.isID() {
			endpoint = p.parseNodeID()
		} else {
			p.errorf(p.pos, "expected node ID or subgraph after edge operator")
			break
		}

		stmt.Rights = append(stmt.Rights, ast.EdgeRight{
			Position: endpoint.Pos(),
			Directed: directed,
			Endpoint: endpoint,
		})
	}

	// Optional attribute list
	if p.tok == token.LBRACKET {
		stmt.Attrs = p.parseAttrList()
	}

	return stmt
}

// parseSubgraphOrGroup parses a subgraph or node group ({A B C}).
func (p *Parser) parseSubgraphOrGroup() ast.EdgeEndpoint {
	if p.tok == token.SUBGRAPH {
		return p.parseSubgraph()
	}

	// Might be a node group {A B C} or anonymous subgraph
	pos := p.pos
	p.expect(token.LBRACE)

	// Peek to see if this looks like a node group (just IDs) or subgraph (statements)
	// A node group contains only node IDs separated by spaces
	// For simplicity, we'll parse as a subgraph if we see statements
	// Otherwise as a node group

	// Try to parse as node group first
	var nodes []*ast.NodeID
	for p.isID() {
		nodes = append(nodes, p.parseNodeID())
	}

	if p.tok == token.RBRACE && len(nodes) > 0 {
		p.next()
		return &ast.NodeGroup{
			Position: pos,
			Nodes:    nodes,
		}
	}

	// Not a simple node group, parse as subgraph
	// We already consumed some nodes, convert them to statements
	var stmts []Statement
	for _, n := range nodes {
		stmts = append(stmts, &ast.NodeStmt{
			Position: n.Position,
			NodeID:   n,
		})
	}

	// Continue parsing statements
	moreStmts := p.parseStmtList()
	stmts = append(stmts, moreStmts...)

	p.expect(token.RBRACE)

	return &ast.Subgraph{
		Position:   pos,
		Statements: stmts,
	}
}

// parseSubgraph parses: [ 'subgraph' [ ID ] ] '{' stmt_list '}'
func (p *Parser) parseSubgraph() *ast.Subgraph {
	sub := &ast.Subgraph{Position: p.pos}

	if p.tok == token.SUBGRAPH {
		p.next()
		if p.isID() {
			sub.ID = p.parseIdent()
		}
	}

	p.expect(token.LBRACE)
	sub.Statements = p.parseStmtList()
	p.expect(token.RBRACE)

	return sub
}

// parseNodeID parses: ID [ port ]
func (p *Parser) parseNodeID() *ast.NodeID {
	nodeID := &ast.NodeID{
		Position: p.pos,
		ID:       p.parseIdent(),
	}
	if p.tok == token.COLON {
		nodeID.Port = p.parsePort()
	}
	return nodeID
}

// parsePort parses: ':' ID [ ':' compass_pt ]
func (p *Parser) parsePort() *ast.Port {
	pos := p.pos
	p.expect(token.COLON)

	port := &ast.Port{Position: pos}
	if p.isID() {
		port.ID = p.parseIdent()
	}

	if p.tok == token.COLON {
		p.next()
		if p.isID() {
			port.Compass = p.parseIdent()
		}
	}

	return port
}

// parseAttrList parses: '[' [ a_list ] ']' [ attr_list ]
func (p *Parser) parseAttrList() *ast.AttrList {
	list := &ast.AttrList{Position: p.pos}

	for p.tok == token.LBRACKET {
		p.next()

		// Parse a_list: ID '=' ID [ (';' | ',') ] [ a_list ]
		for p.isID() {
			attr := &ast.Attr{Position: p.pos}
			attr.Key = p.parseIdent()

			if p.tok == token.EQUAL {
				p.next()
				if p.isID() {
					attr.Value = p.parseIdent()
				} else {
					p.errorf(p.pos, "expected value after '='")
					attr.Value = &ast.Ident{Position: p.pos, Name: ""}
				}
			} else {
				// Attribute without value (treat as true)
				attr.Value = &ast.Ident{Position: attr.Key.Position, Name: "true"}
			}

			list.Attrs = append(list.Attrs, attr)

			// Optional separator
			if p.tok == token.SEMICOLON || p.tok == token.COMMA {
				p.next()
			}
		}

		p.expect(token.RBRACKET)
	}

	return list
}

// parseIdent parses an identifier.
func (p *Parser) parseIdent() *ast.Ident {
	id := &ast.Ident{Position: p.pos}

	switch p.tok {
	case token.IDENT:
		id.Name = p.lit
	case token.STRING:
		id.Name = p.lit
		id.Quoted = true
	case token.HTML:
		id.Name = p.lit
		id.HTML = true
	default:
		p.errorf(p.pos, "expected identifier, got %s", p.tok)
		id.Name = ""
	}

	p.next()
	return id
}
