package d3

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"html/template"

	"github.com/anthonybishopric/dot2d3/pkg/ast"
)

// Converter converts an AST graph to a D3 graph structure.
type Converter struct {
	nodes      map[string]*Node
	links      []Link
	subgraphs  []Subgraph
	directed   bool
	strict     bool
	graphID    string

	// Default attributes from attr statements
	nodeDefaults map[string]string
	edgeDefaults map[string]string

	// Current subgraph context
	currentSubgraph string
}

// Convert transforms an AST graph into a D3 graph structure.
func Convert(g *ast.Graph) (*Graph, error) {
	c := &Converter{
		nodes:        make(map[string]*Node),
		directed:     g.Directed,
		strict:       g.Strict,
		nodeDefaults: make(map[string]string),
		edgeDefaults: make(map[string]string),
	}

	if g.ID != nil {
		c.graphID = g.ID.Name
	}

	// Process all statements
	c.processStatements(g.Statements, "")

	// Build the final graph
	nodes := make([]Node, 0, len(c.nodes))
	for _, n := range c.nodes {
		nodes = append(nodes, *n)
	}

	return &Graph{
		Nodes:     nodes,
		Links:     c.links,
		Directed:  c.directed,
		Strict:    c.strict,
		GraphID:   c.graphID,
		Subgraphs: c.subgraphs,
	}, nil
}

func (c *Converter) processStatements(stmts []ast.Statement, subgraphID string) {
	for _, stmt := range stmts {
		c.processStatement(stmt, subgraphID)
	}
}

func (c *Converter) processStatement(stmt ast.Statement, subgraphID string) {
	switch s := stmt.(type) {
	case *ast.NodeStmt:
		c.processNodeStmt(s, subgraphID)
	case *ast.EdgeStmt:
		c.processEdgeStmt(s, subgraphID)
	case *ast.AttrStmt:
		c.processAttrStmt(s)
	case *ast.AttrAssign:
		// Graph-level attributes, ignore for now
	case *ast.Subgraph:
		c.processSubgraph(s)
	}
}

func (c *Converter) processNodeStmt(stmt *ast.NodeStmt, subgraphID string) {
	id := stmt.NodeID.ID.Name
	node := c.getOrCreateNode(id)

	// Apply default attributes
	for k, v := range c.nodeDefaults {
		c.applyNodeAttr(node, k, v)
	}

	// Apply statement attributes
	if stmt.Attrs != nil {
		for _, attr := range stmt.Attrs.Attrs {
			c.applyNodeAttr(node, attr.Key.Name, attr.Value.Name)
		}
	}

	// Set subgraph membership
	if subgraphID != "" {
		node.Group = subgraphID
	}
}

func (c *Converter) processEdgeStmt(stmt *ast.EdgeStmt, subgraphID string) {
	// Collect all endpoints
	endpoints := c.collectEndpoints(stmt.Left, subgraphID)

	for _, right := range stmt.Rights {
		rightEndpoints := c.collectEndpoints(right.Endpoint, subgraphID)

		// Create edges from all left endpoints to all right endpoints
		for _, leftID := range endpoints {
			for _, rightID := range rightEndpoints {
				link := Link{
					Source: leftID,
					Target: rightID,
				}

				// Apply default edge attributes
				for k, v := range c.edgeDefaults {
					c.applyLinkAttr(&link, k, v)
				}

				// Apply statement attributes
				if stmt.Attrs != nil {
					for _, attr := range stmt.Attrs.Attrs {
						c.applyLinkAttr(&link, attr.Key.Name, attr.Value.Name)
					}
				}

				// Check for duplicates if strict
				if c.strict && c.linkExists(link.Source, link.Target) {
					continue
				}

				c.links = append(c.links, link)
			}
		}

		// The right endpoints become the left endpoints for the next edge
		endpoints = rightEndpoints
	}
}

func (c *Converter) collectEndpoints(ep ast.EdgeEndpoint, subgraphID string) []string {
	var ids []string

	switch e := ep.(type) {
	case *ast.NodeID:
		id := e.ID.Name
		c.ensureNode(id, subgraphID)
		ids = append(ids, id)
	case *ast.NodeGroup:
		for _, n := range e.Nodes {
			id := n.ID.Name
			c.ensureNode(id, subgraphID)
			ids = append(ids, id)
		}
	case *ast.Subgraph:
		// Process subgraph and collect all node IDs within it
		sgID := ""
		if e.ID != nil {
			sgID = e.ID.Name
		}
		ids = c.processSubgraphNodes(e, sgID)
	}

	return ids
}

func (c *Converter) processSubgraphNodes(sg *ast.Subgraph, subgraphID string) []string {
	var nodeIDs []string

	for _, stmt := range sg.Statements {
		switch s := stmt.(type) {
		case *ast.NodeStmt:
			id := s.NodeID.ID.Name
			c.ensureNode(id, subgraphID)
			nodeIDs = append(nodeIDs, id)
		case *ast.EdgeStmt:
			// Process edges within subgraph
			c.processEdgeStmt(s, subgraphID)
			// Collect node IDs from edge endpoints
			ids := c.collectEndpoints(s.Left, subgraphID)
			nodeIDs = append(nodeIDs, ids...)
			for _, r := range s.Rights {
				ids = c.collectEndpoints(r.Endpoint, subgraphID)
				nodeIDs = append(nodeIDs, ids...)
			}
		case *ast.Subgraph:
			ids := c.processSubgraphNodes(s, subgraphID)
			nodeIDs = append(nodeIDs, ids...)
		}
	}

	return nodeIDs
}

func (c *Converter) processAttrStmt(stmt *ast.AttrStmt) {
	if stmt.Attrs == nil {
		return
	}

	switch stmt.Kind {
	case ast.NodeAttr:
		for _, attr := range stmt.Attrs.Attrs {
			c.nodeDefaults[attr.Key.Name] = attr.Value.Name
		}
	case ast.EdgeAttr:
		for _, attr := range stmt.Attrs.Attrs {
			c.edgeDefaults[attr.Key.Name] = attr.Value.Name
		}
	case ast.GraphAttr:
		// Graph attributes, ignore for now
	}
}

func (c *Converter) processSubgraph(sg *ast.Subgraph) {
	sgID := ""
	if sg.ID != nil {
		sgID = sg.ID.Name
	}

	var nodeIDs []string
	for _, stmt := range sg.Statements {
		c.processStatement(stmt, sgID)
		// Collect nodes added by this statement
		switch s := stmt.(type) {
		case *ast.NodeStmt:
			nodeIDs = append(nodeIDs, s.NodeID.ID.Name)
		case *ast.EdgeStmt:
			ids := c.collectEndpoints(s.Left, sgID)
			nodeIDs = append(nodeIDs, ids...)
			for _, r := range s.Rights {
				ids = c.collectEndpoints(r.Endpoint, sgID)
				nodeIDs = append(nodeIDs, ids...)
			}
		}
	}

	if sgID != "" {
		sub := Subgraph{
			ID:    sgID,
			Nodes: nodeIDs,
		}
		// Check for label in subgraph statements
		for _, stmt := range sg.Statements {
			if assign, ok := stmt.(*ast.AttrAssign); ok && assign.Key.Name == "label" {
				sub.Label = assign.Value.Name
			}
		}
		c.subgraphs = append(c.subgraphs, sub)
	}
}

func (c *Converter) getOrCreateNode(id string) *Node {
	if n, ok := c.nodes[id]; ok {
		return n
	}
	n := &Node{
		ID:    id,
		Label: id, // Default label is the ID
	}
	c.nodes[id] = n
	return n
}

func (c *Converter) ensureNode(id string, subgraphID string) {
	node := c.getOrCreateNode(id)

	// Apply default attributes to newly created nodes
	for k, v := range c.nodeDefaults {
		// Only apply if the attribute isn't already set
		switch k {
		case "label":
			if node.Label == id { // Still has default label
				c.applyNodeAttr(node, k, v)
			}
		case "color", "fillcolor":
			if node.Color == "" {
				c.applyNodeAttr(node, k, v)
			}
		case "shape":
			if node.Shape == "" {
				c.applyNodeAttr(node, k, v)
			}
		case "style":
			if node.Style == "" {
				c.applyNodeAttr(node, k, v)
			}
		default:
			if node.Attributes == nil || node.Attributes[k] == "" {
				c.applyNodeAttr(node, k, v)
			}
		}
	}

	if subgraphID != "" && node.Group == "" {
		node.Group = subgraphID
	}
}

func (c *Converter) applyNodeAttr(node *Node, key, value string) {
	switch key {
	case "label":
		node.Label = value
	case "color", "fillcolor":
		node.Color = value
	case "shape":
		node.Shape = value
	case "style":
		node.Style = value
	default:
		if node.Attributes == nil {
			node.Attributes = make(map[string]string)
		}
		node.Attributes[key] = value
	}
}

func (c *Converter) applyLinkAttr(link *Link, key, value string) {
	switch key {
	case "label":
		link.Label = value
	case "color":
		link.Color = value
	case "style":
		link.Style = value
	default:
		if link.Attributes == nil {
			link.Attributes = make(map[string]string)
		}
		link.Attributes[key] = value
	}
}

func (c *Converter) linkExists(source, target string) bool {
	for _, l := range c.links {
		if l.Source == source && l.Target == target {
			return true
		}
		// For undirected graphs, also check reverse
		if !c.directed && l.Source == target && l.Target == source {
			return true
		}
	}
	return false
}

// ApplyPathHighlighting validates and applies path highlighting to a graph.
// The pathGraph contains edges that should be highlighted in the main graph.
// Returns a validation result indicating success or the first failing edge.
func ApplyPathHighlighting(g *Graph, pathGraph *ast.Graph) *PathValidationResult {
	// Build lookup maps for quick access
	nodeMap := make(map[string]*Node)
	for i := range g.Nodes {
		nodeMap[g.Nodes[i].ID] = &g.Nodes[i]
	}

	// Helper to find a link by source and target
	findLink := func(source, target string) *Link {
		for i := range g.Links {
			if g.Links[i].Source == source && g.Links[i].Target == target {
				return &g.Links[i]
			}
			// For undirected graphs, also check reverse
			if !g.Directed && g.Links[i].Source == target && g.Links[i].Target == source {
				return &g.Links[i]
			}
		}
		return nil
	}

	// Extract edges from path graph and validate each one
	for _, stmt := range pathGraph.Statements {
		edgeStmt, ok := stmt.(*ast.EdgeStmt)
		if !ok {
			continue // Skip non-edge statements in path
		}

		// Process edge chain
		leftNodes := collectPathEndpoints(edgeStmt.Left)
		for _, right := range edgeStmt.Rights {
			rightNodes := collectPathEndpoints(right.Endpoint)

			// Check all combinations
			for _, leftID := range leftNodes {
				for _, rightID := range rightNodes {
					// Check if both nodes exist
					leftNode, leftExists := nodeMap[leftID]
					rightNode, rightExists := nodeMap[rightID]

					if !leftExists && !rightExists {
						// Neither node exists
						return &PathValidationResult{
							Valid: false,
							Error: "edge '" + leftID + " -> " + rightID + "' references unknown node '" + leftID + "'",
							InvalidEdge: &InvalidEdge{
								Source:      leftID,
								Target:      rightID,
								InvalidNode: leftID,
							},
						}
					}

					if !leftExists {
						// Left node doesn't exist, mark right as error point
						rightNode.PathInvalid = true
						return &PathValidationResult{
							Valid: false,
							Error: "edge '" + leftID + " -> " + rightID + "' references unknown node '" + leftID + "'",
							InvalidEdge: &InvalidEdge{
								Source:      leftID,
								Target:      rightID,
								InvalidNode: leftID,
							},
							LastValidNode: rightID,
						}
					}

					if !rightExists {
						// Right node doesn't exist, mark left as error point
						leftNode.PathInvalid = true
						return &PathValidationResult{
							Valid: false,
							Error: "edge '" + leftID + " -> " + rightID + "' references unknown node '" + rightID + "'",
							InvalidEdge: &InvalidEdge{
								Source:      leftID,
								Target:      rightID,
								InvalidNode: rightID,
							},
							LastValidNode: leftID,
						}
					}

					// Both nodes exist, mark them as on path
					leftNode.OnPath = true
					rightNode.OnPath = true

					// Find and mark the link
					link := findLink(leftID, rightID)
					if link != nil {
						link.OnPath = true
					}
					// Note: We don't error if the edge doesn't exist in the graph,
					// we just don't highlight it. The nodes are still valid.
				}
			}

			// Move to next edge in chain
			leftNodes = rightNodes
		}
	}

	return &PathValidationResult{Valid: true}
}

// collectPathEndpoints extracts node IDs from an edge endpoint for path validation.
func collectPathEndpoints(ep ast.EdgeEndpoint) []string {
	var ids []string

	switch e := ep.(type) {
	case *ast.NodeID:
		ids = append(ids, e.ID.Name)
	case *ast.NodeGroup:
		for _, n := range e.Nodes {
			ids = append(ids, n.ID.Name)
		}
	case *ast.Subgraph:
		// Recursively collect from subgraph statements
		for _, stmt := range e.Statements {
			if nodeStmt, ok := stmt.(*ast.NodeStmt); ok {
				ids = append(ids, nodeStmt.NodeID.ID.Name)
			}
			if edgeStmt, ok := stmt.(*ast.EdgeStmt); ok {
				ids = append(ids, collectPathEndpoints(edgeStmt.Left)...)
				for _, r := range edgeStmt.Rights {
					ids = append(ids, collectPathEndpoints(r.Endpoint)...)
				}
			}
		}
	}

	return ids
}

// RenderOptions configures HTML rendering.
type RenderOptions struct {
	Title    string
	Width    int
	Height   int
	PathAST  *ast.Graph // Optional path graph to highlight
	GraphDOT string     // Original graph DOT source (for shareable links)
	PathDOT  string     // Original path DOT source (for shareable links)
}

// RenderHTML generates a self-contained HTML file with the D3 visualization.
// If opts.PathAST is set, path highlighting will be applied.
func RenderHTML(g *Graph, opts RenderOptions) ([]byte, error) {
	html, _, err := RenderHTMLWithValidation(g, opts)
	return html, err
}

// RenderHTMLWithValidation generates HTML and returns path validation result.
// If path validation fails, HTML is still generated with the error node highlighted red.
func RenderHTMLWithValidation(g *Graph, opts RenderOptions) ([]byte, *PathValidationResult, error) {
	if opts.Title == "" {
		opts.Title = "Graph Visualization"
		if g.GraphID != "" {
			opts.Title = g.GraphID
		}
	}

	// Apply path highlighting if provided
	var pathResult *PathValidationResult
	if opts.PathAST != nil {
		pathResult = ApplyPathHighlighting(g, opts.PathAST)
	}

	graphJSON, err := json.Marshal(g)
	if err != nil {
		return nil, nil, err
	}

	// Encode DOT sources as base64 to avoid escaping issues in the template
	graphDOTb64 := ""
	if opts.GraphDOT != "" {
		graphDOTb64 = base64.StdEncoding.EncodeToString([]byte(opts.GraphDOT))
	}
	pathDOTb64 := ""
	if opts.PathDOT != "" {
		pathDOTb64 = base64.StdEncoding.EncodeToString([]byte(opts.PathDOT))
	}

	data := struct {
		Title       string
		GraphJSON   template.JS
		GraphDOTb64 string
		PathDOTb64  string
	}{
		Title:       opts.Title,
		GraphJSON:   template.JS(graphJSON),
		GraphDOTb64: graphDOTb64,
		PathDOTb64:  pathDOTb64,
	}

	tmpl, err := template.New("graph").Parse(htmlTemplate)
	if err != nil {
		return nil, nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, nil, err
	}

	return buf.Bytes(), pathResult, nil
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            overflow: hidden;
            background: #f5f5f5;
        }
        svg {
            width: 100vw;
            height: 100vh;
            background: white;
        }
        .node { cursor: pointer; }
        .node:hover { filter: brightness(0.85); }
        .node.selected ellipse,
        .node.selected rect,
        .node.selected polygon {
            stroke: #ff6b00;
            stroke-width: 3;
        }
        .node.filtered-out { opacity: 0.15; }
        .link {
            stroke: #999;
            stroke-opacity: 0.6;
            fill: none;
        }
        .link.directed { marker-end: url(#arrowhead); }
        .link.filtered-out { opacity: 0.08; }
        .node-label {
            font-size: 12px;
            pointer-events: none;
            text-anchor: middle;
            dominant-baseline: central;
            fill: #333;
        }
        .node.filtered-out .node-label { opacity: 0.3; }
        .link-label {
            font-size: 10px;
            fill: #666;
            cursor: pointer;
            transition: fill 0.15s;
        }
        .link-label:hover {
            fill: #333;
        }
        .link-label.filtered-out { opacity: 0.15; }
        .link.highlighted {
            stroke: #ff6b00 !important;
            stroke-opacity: 1;
            stroke-width: 3;
        }
        .link.directed.highlighted {
            marker-end: url(#arrowhead-highlighted);
        }
        .link-label.highlighted {
            fill: #ff6b00;
            font-weight: 600;
        }
        /* Dimmed elements - use color overrides instead of opacity for performance */
        .node.dimmed ellipse,
        .node.dimmed rect,
        .node.dimmed polygon {
            fill: #e0e0e0 !important;
            stroke: #ccc !important;
        }
        .node.dimmed text {
            fill: #bbb !important;
        }
        .link.dimmed {
            stroke: #eee !important;
        }
        .link-label.dimmed {
            fill: #ccc !important;
        }
        /* Path highlighting - orange for valid path */
        .node.on-path ellipse,
        .node.on-path rect,
        .node.on-path polygon {
            stroke: #ff6b00;
            stroke-width: 4;
        }
        .link.on-path {
            stroke: #ff6b00 !important;
            stroke-opacity: 1;
            stroke-width: 4;
        }
        .link.directed.on-path {
            marker-end: url(#arrowhead-path);
        }
        /* Path invalid node - red highlight */
        .node.path-invalid ellipse,
        .node.path-invalid rect,
        .node.path-invalid polygon {
            stroke: #f44336;
            stroke-width: 5;
        }
        .tooltip {
            position: absolute;
            background: rgba(0, 0, 0, 0.85);
            color: white;
            padding: 8px 12px;
            border-radius: 4px;
            font-size: 12px;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.15s;
            max-width: 300px;
            z-index: 1000;
        }
        .tooltip strong { color: #fff; }
        .tooltip .attr { color: #aaa; margin-top: 4px; }
        .controls {
            position: absolute;
            top: 16px;
            left: 16px;
            background: white;
            border-radius: 8px;
            box-shadow: 0 2px 12px rgba(0,0,0,0.15);
            padding: 16px;
            z-index: 100;
            min-width: 220px;
        }
        .controls h3 {
            font-size: 14px;
            font-weight: 600;
            margin-bottom: 12px;
            color: #333;
        }
        .control-group {
            margin-bottom: 12px;
        }
        .control-group:last-child {
            margin-bottom: 0;
        }
        .control-group label {
            display: block;
            font-size: 12px;
            color: #666;
            margin-bottom: 6px;
        }
        .slider-container {
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .slider-container input[type="range"] {
            flex: 1;
            height: 6px;
            -webkit-appearance: none;
            appearance: none;
            background: #e0e0e0;
            border-radius: 3px;
            outline: none;
        }
        .slider-container input[type="range"]::-webkit-slider-thumb {
            -webkit-appearance: none;
            appearance: none;
            width: 16px;
            height: 16px;
            background: #4a90d9;
            border-radius: 50%;
            cursor: pointer;
        }
        .slider-container input[type="range"]::-moz-range-thumb {
            width: 16px;
            height: 16px;
            background: #4a90d9;
            border-radius: 50%;
            cursor: pointer;
            border: none;
        }
        .slider-value {
            min-width: 36px;
            text-align: center;
            font-size: 13px;
            font-weight: 500;
            color: #333;
        }
        .selected-node {
            font-size: 13px;
            color: #333;
            padding: 8px 10px;
            background: #f5f5f5;
            border-radius: 4px;
        }
        .selected-node.none {
            color: #999;
            font-style: italic;
        }
        .clear-btn {
            margin-top: 8px;
            padding: 6px 12px;
            font-size: 12px;
            background: #f0f0f0;
            border: 1px solid #ddd;
            border-radius: 4px;
            cursor: pointer;
            color: #666;
        }
        .clear-btn:hover {
            background: #e8e8e8;
            color: #333;
        }
        .help-text {
            font-size: 11px;
            color: #999;
            margin-top: 12px;
            line-height: 1.4;
        }
        .share-section {
            margin-top: 16px;
            padding-top: 16px;
            border-top: 1px solid #eee;
        }
        .share-btn {
            width: 100%;
            padding: 8px 12px;
            font-size: 13px;
            background: #5cb85c;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            color: white;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
        }
        .share-btn:hover { background: #4cae4c; }
        .share-btn:disabled {
            background: #ccc;
            cursor: not-allowed;
        }
        .share-btn.copied {
            background: #337ab7;
        }
        .edit-btn {
            width: 100%;
            padding: 8px 12px;
            font-size: 13px;
            background: #f0f0f0;
            border: 1px solid #ddd;
            border-radius: 4px;
            cursor: pointer;
            color: #666;
            display: flex;
            align-items: center;
            justify-content: center;
            gap: 6px;
            margin-top: 8px;
            text-decoration: none;
        }
        .edit-btn:hover {
            background: #e8e8e8;
            color: #333;
        }
        .share-feedback {
            font-size: 11px;
            color: #666;
            margin-top: 6px;
            text-align: center;
        }
        .share-feedback.error {
            color: #c9302c;
        }
        .share-feedback.success {
            color: #3c763d;
        }
    </style>
</head>
<body>
    <div class="controls">
        <h3>Graph Filter</h3>
        <div class="control-group">
            <label>Selected Node</label>
            <div class="selected-node none" id="selected-display">Click a node to select</div>
            <button class="clear-btn" id="clear-selection" style="display: none;">Clear Selection</button>
        </div>
        <div class="control-group">
            <label>Degree of Separation</label>
            <div class="slider-container">
                <input type="range" id="degree-slider" min="0" max="5" value="0" step="1">
                <span class="slider-value" id="degree-value">All</span>
            </div>
        </div>
        <div class="help-text">
            Select a node and adjust the degree slider to filter the view to nodes within N connections.
            Set to "All" to show the complete graph.
        </div>
        <div class="share-section">
            <button class="share-btn" id="copy-link-btn">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
                    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
                </svg>
                Copy Link
            </button>
            <a class="edit-btn" id="edit-btn" href="#">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"></path>
                    <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"></path>
                </svg>
                Edit Graph
            </a>
            <div class="share-feedback" id="share-feedback"></div>
        </div>
    </div>
    <div class="tooltip" id="tooltip"></div>
    <svg id="graph"></svg>

    <script>
    const graphData = {{.GraphJSON}};

    // Original DOT sources for shareable links (base64 encoded to avoid escaping issues)
    const graphDOTb64 = "{{.GraphDOTb64}}";
    const pathDOTb64 = "{{.PathDOTb64}}";

    // Decode the DOT sources
    function decodeB64(s) {
        if (!s) return "";
        try {
            return decodeURIComponent(escape(atob(s)));
        } catch (e) {
            return "";
        }
    }
    const graphDOT = decodeB64(graphDOTb64);
    const pathDOT = decodeB64(pathDOTb64);

    const width = window.innerWidth;
    const height = window.innerHeight;

    // State for filtering
    let selectedNodeId = null;
    let degreeFilter = 0; // 0 means "All" (no filter)

    // Build adjacency list for traversal (treat as undirected for reachability)
    const adjacency = new Map();
    graphData.nodes.forEach(n => adjacency.set(n.id, new Set()));
    graphData.links.forEach(l => {
        const sourceId = typeof l.source === 'object' ? l.source.id : l.source;
        const targetId = typeof l.target === 'object' ? l.target.id : l.target;
        adjacency.get(sourceId).add(targetId);
        adjacency.get(targetId).add(sourceId);
    });

    // BFS to find nodes within N degrees of a starting node
    function getNodesWithinDegree(startId, maxDegree) {
        if (!startId || maxDegree <= 0) return null; // null means show all

        const visited = new Set([startId]);
        const queue = [{id: startId, depth: 0}];

        while (queue.length > 0) {
            const {id, depth} = queue.shift();
            if (depth >= maxDegree) continue;

            for (const neighborId of adjacency.get(id) || []) {
                if (!visited.has(neighborId)) {
                    visited.add(neighborId);
                    queue.push({id: neighborId, depth: depth + 1});
                }
            }
        }

        return visited;
    }

    // Update filter display and apply filtering
    function updateFilter() {
        const visibleNodes = getNodesWithinDegree(selectedNodeId, degreeFilter);

        // Update node visibility
        node.classed("filtered-out", d => {
            if (!visibleNodes) return false; // Show all
            return !visibleNodes.has(d.id);
        });

        // Update selected state
        node.classed("selected", d => d.id === selectedNodeId);

        // Update link visibility
        link.classed("filtered-out", d => {
            if (!visibleNodes) return false;
            const sourceId = typeof d.source === 'object' ? d.source.id : d.source;
            const targetId = typeof d.target === 'object' ? d.target.id : d.target;
            return !visibleNodes.has(sourceId) || !visibleNodes.has(targetId);
        });

        // Update link label visibility
        linkLabel.classed("filtered-out", d => {
            if (!visibleNodes) return false;
            const sourceId = typeof d.source === 'object' ? d.source.id : d.source;
            const targetId = typeof d.target === 'object' ? d.target.id : d.target;
            return !visibleNodes.has(sourceId) || !visibleNodes.has(targetId);
        });

        // Update UI
        const selectedDisplay = document.getElementById("selected-display");
        const clearBtn = document.getElementById("clear-selection");

        if (selectedNodeId) {
            const selectedNode = graphData.nodes.find(n => n.id === selectedNodeId);
            selectedDisplay.textContent = selectedNode ? (selectedNode.label || selectedNode.id) : selectedNodeId;
            selectedDisplay.classList.remove("none");
            clearBtn.style.display = "block";
        } else {
            selectedDisplay.textContent = "Click a node to select";
            selectedDisplay.classList.add("none");
            clearBtn.style.display = "none";
        }

        // Emit filter change event
        const filterEvent = new CustomEvent("filterChange", {
            detail: {
                selectedNodeId,
                degree: degreeFilter,
                visibleNodeCount: visibleNodes ? visibleNodes.size : graphData.nodes.length
            },
            bubbles: true
        });
        document.dispatchEvent(filterEvent);
    }

    // Slider event handler
    const degreeSlider = document.getElementById("degree-slider");
    const degreeValue = document.getElementById("degree-value");

    degreeSlider.addEventListener("input", function() {
        degreeFilter = parseInt(this.value);
        degreeValue.textContent = degreeFilter === 0 ? "All" : degreeFilter;
        updateFilter();
    });

    // Clear selection button
    document.getElementById("clear-selection").addEventListener("click", function() {
        selectedNodeId = null;
        updateFilter();
    });

    const svg = d3.select("#graph")
        .attr("viewBox", [0, 0, width, height]);

    // Container for zoom/pan
    const g = svg.append("g");

    // Zoom behavior
    const zoom = d3.zoom()
        .scaleExtent([0.1, 4])
        .on("zoom", (event) => {
            g.attr("transform", event.transform);
        });
    svg.call(zoom);

    // Arrow markers for directed graphs
    if (graphData.directed) {
        const defs = svg.append("defs");

        // Default arrowhead
        defs.append("marker")
            .attr("id", "arrowhead")
            .attr("viewBox", "0 -5 10 10")
            .attr("refX", 25)
            .attr("refY", 0)
            .attr("markerWidth", 6)
            .attr("markerHeight", 6)
            .attr("orient", "auto")
            .append("path")
            .attr("d", "M0,-5L10,0L0,5")
            .attr("fill", "#999");

        // Highlighted arrowhead (orange)
        defs.append("marker")
            .attr("id", "arrowhead-highlighted")
            .attr("viewBox", "0 -5 10 10")
            .attr("refX", 25)
            .attr("refY", 0)
            .attr("markerWidth", 6)
            .attr("markerHeight", 6)
            .attr("orient", "auto")
            .append("path")
            .attr("d", "M0,-5L10,0L0,5")
            .attr("fill", "#ff6b00");

        // Path arrowhead (orange)
        defs.append("marker")
            .attr("id", "arrowhead-path")
            .attr("viewBox", "0 -5 10 10")
            .attr("refX", 25)
            .attr("refY", 0)
            .attr("markerWidth", 8)
            .attr("markerHeight", 8)
            .attr("orient", "auto")
            .append("path")
            .attr("d", "M0,-5L10,0L0,5")
            .attr("fill", "#ff6b00");
    }

    // Force simulation
    const simulation = d3.forceSimulation(graphData.nodes)
        .force("link", d3.forceLink(graphData.links)
            .id(d => d.id)
            .distance(120))
        .force("charge", d3.forceManyBody().strength(-400))
        .force("center", d3.forceCenter(width / 2, height / 2))
        .force("collision", d3.forceCollide().radius(40));

    // Check if path highlighting is active
    const hasPath = graphData.nodes.some(n => n.onPath) || graphData.links.some(l => l.onPath);

    // Draw links
    const link = g.append("g")
        .attr("class", "links")
        .selectAll("line")
        .data(graphData.links)
        .join("line")
        .attr("class", d => graphData.directed ? "link directed" : "link")
        .classed("on-path", d => d.onPath)
        .classed("dimmed", d => hasPath && !d.onPath)
        .attr("stroke", d => d.color || "#999")
        .attr("stroke-width", 2)
        .attr("stroke-dasharray", d => d.style === "dashed" ? "5,5" : null);

    // Detect bidirectional edges and assign label offsets
    const edgePairs = new Map(); // key: "A|B" (sorted), value: array of link indices
    graphData.links.forEach((l, i) => {
        const sourceId = typeof l.source === 'object' ? l.source.id : l.source;
        const targetId = typeof l.target === 'object' ? l.target.id : l.target;
        const key = [sourceId, targetId].sort().join('|');
        if (!edgePairs.has(key)) {
            edgePairs.set(key, []);
        }
        edgePairs.get(key).push(i);
    });

    // Assign offset index to each link for label positioning
    graphData.links.forEach((l, i) => {
        const sourceId = typeof l.source === 'object' ? l.source.id : l.source;
        const targetId = typeof l.target === 'object' ? l.target.id : l.target;
        const key = [sourceId, targetId].sort().join('|');
        const pairLinks = edgePairs.get(key);
        if (pairLinks.length > 1) {
            // Multiple edges between same nodes
            const idx = pairLinks.indexOf(i);
            l._labelOffset = (idx - (pairLinks.length - 1) / 2) * 14; // 14px spacing
        } else {
            l._labelOffset = 0;
        }
        l._index = i; // Store index for highlighting
    });

    // State for highlighted edge
    let highlightedEdgeIndex = null;

    function updateEdgeHighlight() {
        link.classed("highlighted", (d, i) => i === highlightedEdgeIndex);
        linkLabel.classed("highlighted", d => d._index === highlightedEdgeIndex);
    }

    // Draw link labels
    const linkLabel = g.append("g")
        .attr("class", "link-labels")
        .selectAll("text")
        .data(graphData.links.filter(d => d.label))
        .join("text")
        .attr("class", "link-label")
        .classed("dimmed", d => hasPath && !d.onPath)
        .text(d => d.label)
        .on("click", function(event, d) {
            event.stopPropagation();
            // Toggle highlight
            if (highlightedEdgeIndex === d._index) {
                highlightedEdgeIndex = null;
            } else {
                highlightedEdgeIndex = d._index;
            }
            updateEdgeHighlight();

            // Emit custom event
            const customEvent = new CustomEvent("edgeLabelClick", {
                detail: {
                    source: typeof d.source === 'object' ? d.source.id : d.source,
                    target: typeof d.target === 'object' ? d.target.id : d.target,
                    label: d.label,
                    highlighted: highlightedEdgeIndex === d._index
                },
                bubbles: true
            });
            document.dispatchEvent(customEvent);
        });

    // Draw nodes
    const node = g.append("g")
        .attr("class", "nodes")
        .selectAll("g")
        .data(graphData.nodes)
        .join("g")
        .attr("class", "node")
        .classed("on-path", d => d.onPath)
        .classed("path-invalid", d => d.pathInvalid)
        .classed("dimmed", d => hasPath && !d.onPath && !d.pathInvalid)
        .call(drag(simulation));

    // Color scale for nodes without explicit colors
    const colorScale = d3.scaleOrdinal(d3.schemeTableau10);

    // Node shapes
    node.each(function(d) {
        const el = d3.select(this);
        const shape = (d.shape || "ellipse").toLowerCase();
        const color = d.color || colorScale(d.group || d.id);

        if (shape === "box" || shape === "rect" || shape === "rectangle" || shape === "square") {
            el.append("rect")
                .attr("width", 50)
                .attr("height", 30)
                .attr("x", -25)
                .attr("y", -15)
                .attr("rx", 4)
                .attr("fill", color)
                .attr("stroke", d3.color(color).darker(0.5))
                .attr("stroke-width", 1.5);
        } else if (shape === "diamond") {
            el.append("polygon")
                .attr("points", "0,-20 20,0 0,20 -20,0")
                .attr("fill", color)
                .attr("stroke", d3.color(color).darker(0.5))
                .attr("stroke-width", 1.5);
        } else {
            // Default: ellipse/circle
            el.append("ellipse")
                .attr("rx", 25)
                .attr("ry", 18)
                .attr("fill", color)
                .attr("stroke", d3.color(color).darker(0.5))
                .attr("stroke-width", 1.5);
        }
    });

    // Node labels
    node.append("text")
        .attr("class", "node-label")
        .attr("dy", 1)
        .text(d => d.label || d.id);

    // Tooltip
    const tooltip = d3.select("#tooltip");

    node.on("mouseover", function(event, d) {
        let html = '<strong>' + (d.label || d.id) + '</strong>';
        if (d.attributes && Object.keys(d.attributes).length > 0) {
            html += '<div class="attr">';
            for (const [k, v] of Object.entries(d.attributes)) {
                html += k + ': ' + v + '<br>';
            }
            html += '</div>';
        }

        tooltip
            .style("opacity", 1)
            .style("left", (event.pageX + 12) + "px")
            .style("top", (event.pageY - 12) + "px")
            .html(html);
    })
    .on("mousemove", function(event) {
        tooltip
            .style("left", (event.pageX + 12) + "px")
            .style("top", (event.pageY - 12) + "px");
    })
    .on("mouseout", function() {
        tooltip.style("opacity", 0);
    });

    // Node click handler - selects node and emits custom event
    node.on("click", function(event, d) {
        event.stopPropagation();

        // Toggle selection
        if (selectedNodeId === d.id) {
            selectedNodeId = null;
        } else {
            selectedNodeId = d.id;
        }
        updateFilter();

        // Emit custom event
        const customEvent = new CustomEvent("nodeClick", {
            detail: {
                id: d.id,
                label: d.label,
                color: d.color,
                shape: d.shape,
                group: d.group,
                attributes: d.attributes || {},
                position: { x: d.x, y: d.y },
                selected: selectedNodeId === d.id
            },
            bubbles: true
        });
        document.dispatchEvent(customEvent);

        console.log("Node clicked:", d);
    });

    // Click on background to deselect node and clear edge highlight
    svg.on("click", function(event) {
        if (event.target === this || event.target.tagName === 'svg') {
            selectedNodeId = null;
            highlightedEdgeIndex = null;
            updateFilter();
            updateEdgeHighlight();
        }
    });

    // Drag behavior
    function drag(simulation) {
        function dragstarted(event) {
            if (!event.active) simulation.alphaTarget(0.3).restart();
            event.subject.fx = event.subject.x;
            event.subject.fy = event.subject.y;
        }

        function dragged(event) {
            event.subject.fx = event.x;
            event.subject.fy = event.y;
        }

        function dragended(event) {
            if (!event.active) simulation.alphaTarget(0);
            event.subject.fx = null;
            event.subject.fy = null;
        }

        return d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended);
    }

    // Update positions on tick
    simulation.on("tick", () => {
        link
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        // Position link labels with offset perpendicular to the edge
        linkLabel.attr("transform", d => {
            const midX = (d.source.x + d.target.x) / 2;
            const midY = (d.source.y + d.target.y) / 2;

            // Calculate perpendicular offset
            const dx = d.target.x - d.source.x;
            const dy = d.target.y - d.source.y;
            const len = Math.sqrt(dx * dx + dy * dy) || 1;

            // Perpendicular unit vector
            const perpX = -dy / len;
            const perpY = dx / len;

            // Apply offset
            const offset = d._labelOffset || 0;
            const offsetX = midX + perpX * offset;
            const offsetY = midY + perpY * offset;

            return ` + "`" + `translate(${offsetX},${offsetY})` + "`" + `;
        });

        node.attr("transform", d => ` + "`" + `translate(${d.x},${d.y})` + "`" + `);
    });

    // Listen for events (example usage)
    document.addEventListener("nodeClick", function(e) {
        console.log("nodeClick event:", e.detail);
    });

    document.addEventListener("edgeLabelClick", function(e) {
        console.log("edgeLabelClick event:", e.detail);
    });

    document.addEventListener("filterChange", function(e) {
        console.log("filterChange event:", e.detail);
    });

    // Reset zoom on double-click
    svg.on("dblclick.zoom", null);
    svg.on("dblclick", function() {
        svg.transition().duration(500).call(
            zoom.transform,
            d3.zoomIdentity.translate(0, 0).scale(1)
        );
    });

    // Share link functionality
    const copyLinkBtn = document.getElementById("copy-link-btn");
    const editBtn = document.getElementById("edit-btn");
    const shareFeedback = document.getElementById("share-feedback");

    function generateShareableURL() {
        // Use base64 encoding for the graph and path DOT
        const params = new URLSearchParams();

        if (graphDOT) {
            // Use URL-safe base64 encoding
            const graphB64 = btoa(unescape(encodeURIComponent(graphDOT)))
                .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
            params.set('g', graphB64);
        }

        if (pathDOT) {
            const pathB64 = btoa(unescape(encodeURIComponent(pathDOT)))
                .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
            params.set('p', pathB64);
        }

        // Build the URL - use current origin or a base URL
        const baseURL = window.location.origin + '/';
        return baseURL + '?' + params.toString();
    }

    // Set the edit button href (same as share link - both go to editor)
    if (graphDOT) {
        editBtn.href = generateShareableURL();
    } else {
        editBtn.style.display = 'none';
    }

    copyLinkBtn.addEventListener("click", async function() {
        if (!graphDOT) {
            shareFeedback.textContent = "No graph data to share";
            shareFeedback.className = "share-feedback error";
            return;
        }

        const shareURL = generateShareableURL();

        try {
            await navigator.clipboard.writeText(shareURL);
            copyLinkBtn.classList.add("copied");
            copyLinkBtn.innerHTML = ` + "`" + `
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <polyline points="20 6 9 17 4 12"></polyline>
                </svg>
                Copied!
            ` + "`" + `;
            shareFeedback.textContent = "Link copied to clipboard";
            shareFeedback.className = "share-feedback success";

            // Reset button after 2 seconds
            setTimeout(() => {
                copyLinkBtn.classList.remove("copied");
                copyLinkBtn.innerHTML = ` + "`" + `
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
                        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
                    </svg>
                    Copy Link
                ` + "`" + `;
                shareFeedback.textContent = "";
            }, 2000);
        } catch (err) {
            // Fallback for older browsers - show the URL
            shareFeedback.innerHTML = ` + "`" + `<a href="${shareURL}" target="_blank" style="word-break:break-all;font-size:10px;">Open link</a>` + "`" + `;
            shareFeedback.className = "share-feedback";
        }
    });
    </script>
</body>
</html>`
