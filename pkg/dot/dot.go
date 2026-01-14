// Package dot provides high-level functions for parsing DOT files
// and generating D3.js visualizations.
package dot

import (
	"encoding/json"

	"github.com/anthonybishopric/gographviz/pkg/ast"
	"github.com/anthonybishopric/gographviz/pkg/d3"
	"github.com/anthonybishopric/gographviz/pkg/lexer"
	"github.com/anthonybishopric/gographviz/pkg/parser"
)

// Parse parses DOT source code and returns the AST.
func Parse(filename string, src []byte) (*ast.Graph, error) {
	l := lexer.New(filename, src)
	p := parser.New(l)
	return p.Parse()
}

// ToD3Graph converts an AST graph to a D3-compatible graph structure.
func ToD3Graph(graph *ast.Graph) (*d3.Graph, error) {
	return d3.Convert(graph)
}

// ToJSON generates JSON output for D3 visualization.
func ToJSON(graph *ast.Graph) ([]byte, error) {
	d3g, err := ToD3Graph(graph)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(d3g, "", "  ")
}

// RenderOptions configures HTML rendering.
type RenderOptions = d3.RenderOptions

// ToHTML generates a self-contained HTML file with D3 visualization.
func ToHTML(graph *ast.Graph, opts RenderOptions) ([]byte, error) {
	d3g, err := ToD3Graph(graph)
	if err != nil {
		return nil, err
	}
	return d3.RenderHTML(d3g, opts)
}

// ParseAndRenderHTML is a convenience function that parses DOT and renders HTML.
func ParseAndRenderHTML(filename string, src []byte, opts RenderOptions) ([]byte, error) {
	graph, err := Parse(filename, src)
	if err != nil {
		return nil, err
	}
	return ToHTML(graph, opts)
}

// ParseAndRenderJSON is a convenience function that parses DOT and renders JSON.
func ParseAndRenderJSON(filename string, src []byte) ([]byte, error) {
	graph, err := Parse(filename, src)
	if err != nil {
		return nil, err
	}
	return ToJSON(graph)
}
