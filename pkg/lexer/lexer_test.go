package lexer

import (
	"testing"

	"github.com/anthonybishopric/gographviz/pkg/token"
)

func TestLexer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []struct {
			tok token.Token
			lit string
		}
	}{
		{
			name:  "simple digraph",
			input: `digraph G { A -> B }`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.DIGRAPH, ""},
				{token.IDENT, "G"},
				{token.LBRACE, ""},
				{token.IDENT, "A"},
				{token.ARROW, ""},
				{token.IDENT, "B"},
				{token.RBRACE, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "undirected graph",
			input: `graph { A -- B }`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.GRAPH, ""},
				{token.LBRACE, ""},
				{token.IDENT, "A"},
				{token.DASHDASH, ""},
				{token.IDENT, "B"},
				{token.RBRACE, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "strict digraph",
			input: `strict digraph { }`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.STRICT, ""},
				{token.DIGRAPH, ""},
				{token.LBRACE, ""},
				{token.RBRACE, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "attributes",
			input: `[color=red, shape=box]`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.LBRACKET, ""},
				{token.IDENT, "color"},
				{token.EQUAL, ""},
				{token.IDENT, "red"},
				{token.COMMA, ""},
				{token.IDENT, "shape"},
				{token.EQUAL, ""},
				{token.IDENT, "box"},
				{token.RBRACKET, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "quoted string",
			input: `"hello world"`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.STRING, "hello world"},
				{token.EOF, ""},
			},
		},
		{
			name:  "html string",
			input: `<<b>bold</b>>`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.HTML, "<b>bold</b>"},
				{token.EOF, ""},
			},
		},
		{
			name:  "numbers",
			input: `1.5 -2 3`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.IDENT, "1.5"},
				{token.IDENT, "-2"},
				{token.IDENT, "3"},
				{token.EOF, ""},
			},
		},
		{
			name:  "line comment",
			input: "A // comment\nB",
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.IDENT, "A"},
				{token.IDENT, "B"},
				{token.EOF, ""},
			},
		},
		{
			name:  "block comment",
			input: "A /* block\ncomment */ B",
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.IDENT, "A"},
				{token.IDENT, "B"},
				{token.EOF, ""},
			},
		},
		{
			name:  "preprocessor line",
			input: "A\n# preprocessor\nB",
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.IDENT, "A"},
				{token.IDENT, "B"},
				{token.EOF, ""},
			},
		},
		{
			name:  "subgraph keyword",
			input: `subgraph cluster_0 { }`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.SUBGRAPH, ""},
				{token.IDENT, "cluster_0"},
				{token.LBRACE, ""},
				{token.RBRACE, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "node and edge keywords",
			input: `node [shape=box] edge [color=red]`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.NODE, ""},
				{token.LBRACKET, ""},
				{token.IDENT, "shape"},
				{token.EQUAL, ""},
				{token.IDENT, "box"},
				{token.RBRACKET, ""},
				{token.EDGE, ""},
				{token.LBRACKET, ""},
				{token.IDENT, "color"},
				{token.EQUAL, ""},
				{token.IDENT, "red"},
				{token.RBRACKET, ""},
				{token.EOF, ""},
			},
		},
		{
			name:  "port syntax",
			input: `A:port:n`,
			tokens: []struct {
				tok token.Token
				lit string
			}{
				{token.IDENT, "A"},
				{token.COLON, ""},
				{token.IDENT, "port"},
				{token.COLON, ""},
				{token.IDENT, "n"},
				{token.EOF, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New("test", []byte(tt.input))

			for i, expected := range tt.tokens {
				_, tok, lit := l.Scan()

				if tok != expected.tok {
					t.Errorf("token %d: expected %v, got %v", i, expected.tok, tok)
				}

				if expected.lit != "" && lit != expected.lit {
					t.Errorf("token %d: expected literal %q, got %q", i, expected.lit, lit)
				}
			}
		})
	}
}

func TestLexerErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated string", `"hello`},
		{"unterminated block comment", `/* hello`},
		{"unterminated html string", `<hello`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := New("test", []byte(tt.input))

			// Scan all tokens
			for {
				_, tok, _ := l.Scan()
				if tok == token.EOF {
					break
				}
			}

			if len(l.Errors) == 0 {
				t.Errorf("expected error for %q", tt.input)
			}
		})
	}
}
