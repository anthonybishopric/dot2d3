// Package token defines constants representing the lexical tokens of the DOT language.
package token

import "fmt"

// Token represents a lexical token in the DOT language.
type Token int

const (
	ILLEGAL Token = iota
	EOF
	COMMENT

	// Literals
	IDENT  // identifier (alphanumeric starting with letter/underscore, or numeric)
	STRING // "quoted string"
	HTML   // <html string>

	// Operators and delimiters
	LBRACE    // {
	RBRACE    // }
	LBRACKET  // [
	RBRACKET  // ]
	SEMICOLON // ;
	COLON     // :
	COMMA     // ,
	EQUAL     // =
	ARROW     // ->
	DASHDASH  // --

	// Keywords
	keyword_beg
	STRICT   // strict
	GRAPH    // graph
	DIGRAPH  // digraph
	SUBGRAPH // subgraph
	NODE     // node
	EDGE     // edge
	keyword_end
)

var tokens = [...]string{
	ILLEGAL: "ILLEGAL",
	EOF:     "EOF",
	COMMENT: "COMMENT",

	IDENT:  "IDENT",
	STRING: "STRING",
	HTML:   "HTML",

	LBRACE:    "{",
	RBRACE:    "}",
	LBRACKET:  "[",
	RBRACKET:  "]",
	SEMICOLON: ";",
	COLON:     ":",
	COMMA:     ",",
	EQUAL:     "=",
	ARROW:     "->",
	DASHDASH:  "--",

	STRICT:   "strict",
	GRAPH:    "graph",
	DIGRAPH:  "digraph",
	SUBGRAPH: "subgraph",
	NODE:     "node",
	EDGE:     "edge",
}

// String returns the string representation of the token.
func (t Token) String() string {
	if t >= 0 && int(t) < len(tokens) {
		return tokens[t]
	}
	return fmt.Sprintf("Token(%d)", t)
}

// IsKeyword returns true if the token is a keyword.
func (t Token) IsKeyword() bool {
	return t > keyword_beg && t < keyword_end
}

var keywords map[string]Token

func init() {
	keywords = make(map[string]Token)
	for i := keyword_beg + 1; i < keyword_end; i++ {
		keywords[tokens[i]] = i
	}
}

// Lookup returns the token associated with a given identifier string.
// If the string is a keyword, the keyword token is returned.
// Otherwise, IDENT is returned.
func Lookup(ident string) Token {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

// Position represents a position in source code.
type Position struct {
	Filename string
	Offset   int // byte offset
	Line     int // 1-indexed line number
	Column   int // 1-indexed column number
}

// String returns a string representation of the position.
func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// IsValid returns true if the position is valid.
func (p Position) IsValid() bool {
	return p.Line > 0
}
