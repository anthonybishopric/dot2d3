package d3

import (
	"bytes"
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
		// Check for label, color, and style in subgraph statements
		for _, stmt := range sg.Statements {
			if assign, ok := stmt.(*ast.AttrAssign); ok {
				switch assign.Key.Name {
				case "label":
					sub.Label = assign.Value.Name
				case "color":
					sub.Color = assign.Value.Name
				case "style":
					sub.Style = assign.Value.Name
				}
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
		case "color":
			if node.Color == "" {
				c.applyNodeAttr(node, k, v)
			}
		case "fillcolor":
			if node.FillColor == "" {
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
	case "color":
		node.Color = value // Border/stroke color
	case "fillcolor":
		node.FillColor = value // Fill color
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
	Title   string
	Width   int
	Height  int
	PathAST *ast.Graph // Optional path graph to highlight
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

	data := struct {
		Title     string
		GraphJSON template.JS
	}{
		Title:     opts.Title,
		GraphJSON: template.JS(graphJSON),
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
        #graph {
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
            stroke-opacity: 0.6;
            fill: none;
            cursor: pointer;
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
        /* Unified edge for multi-edge node pairs */
        .unified-link {
            stroke-opacity: 0.6;
            fill: none;
        }
        .unified-link.bidirectional {
            marker-start: url(#arrowhead-reverse);
            marker-end: url(#arrowhead);
        }
        .unified-link.highlighted {
            stroke: #ff6b00 !important;
            stroke-opacity: 1;
            stroke-width: 3;
        }
        /* Curved edge shown when a specific edge label is selected */
        .curved-edge {
            fill: none;
            stroke-opacity: 0;
            pointer-events: none;
            transition: stroke-opacity 0.15s;
        }
        .curved-edge.visible {
            stroke-opacity: 1;
            stroke-width: 3;
        }
        .curved-edge.directed {
            marker-end: url(#arrowhead-curved);
        }
        /* Multi-edge label container */
        .multi-edge-labels {
            pointer-events: all;
        }
        .multi-edge-label {
            font-size: 10px;
            fill: #666;
            cursor: pointer;
            transition: fill 0.15s;
        }
        .multi-edge-label:hover {
            fill: #333;
        }
        .multi-edge-label.highlighted {
            fill: #ff6b00;
            font-weight: 600;
        }
        .unified-link.filtered-out { opacity: 0.08; }
        .multi-edge-labels.filtered-out { opacity: 0.15; }
        .curved-edge.filtered-out { opacity: 0.08; }
        /* Dimmed elements - use opacity to preserve custom colors */
        .node.dimmed {
            opacity: 0.25;
        }
        .link.dimmed {
            opacity: 0.15;
        }
        .link-label.dimmed {
            opacity: 0.25;
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
        .checkbox-control {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .checkbox-control input[type="checkbox"] {
            width: 16px;
            height: 16px;
            cursor: pointer;
        }
        .checkbox-control span {
            font-size: 13px;
            color: #333;
            cursor: pointer;
            user-select: none;
        }
        /* Cluster/Subgraph styling */
        .cluster-hull {
            fill-opacity: 0.15;
            stroke-width: 2;
            stroke-dasharray: 5,3;
        }
        .cluster-hull.filled {
            fill-opacity: 0.25;
        }
        .cluster-label {
            font-size: 14px;
            font-weight: 600;
            fill: #555;
            pointer-events: none;
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
                <input type="range" id="degree-slider" min="0" max="5" value="1" step="1">
                <span class="slider-value" id="degree-value">1</span>
            </div>
        </div>
        <div class="control-group">
            <label class="checkbox-control">
                <input type="checkbox" id="lock-positions">
                <span>Lock node positions</span>
            </label>
        </div>
        <div class="help-text">
            Select a node and adjust the degree slider to filter the view to nodes within N connections.
            Set to "All" to show the complete graph.
        </div>
    </div>
    <div class="tooltip" id="tooltip"></div>
    <svg id="graph"></svg>

    <script>
    const graphData = {{.GraphJSON}};

    const width = window.innerWidth;
    const height = window.innerHeight;

    // State for filtering
    let selectedNodeId = null;
    let degreeFilter = 1; // 0 means "All" (no filter), default to 1
    let positionsLocked = false; // When true, simulation is stopped but dragging still works

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

        // Update single-edge link visibility
        if (typeof link !== 'undefined') {
            link.classed("filtered-out", d => {
                if (!visibleNodes) return false;
                const sourceId = typeof d.source === 'object' ? d.source.id : d.source;
                const targetId = typeof d.target === 'object' ? d.target.id : d.target;
                return !visibleNodes.has(sourceId) || !visibleNodes.has(targetId);
            });
        }

        // Update unified link visibility (for multi-edge groups)
        if (typeof unifiedLinks !== 'undefined') {
            unifiedLinks.classed("filtered-out", d => {
                if (!visibleNodes) return false;
                return !visibleNodes.has(d.nodeA) || !visibleNodes.has(d.nodeB);
            });
        }

        // Update single-edge link label visibility
        if (typeof linkLabel !== 'undefined') {
            linkLabel.classed("filtered-out", d => {
                if (!visibleNodes) return false;
                const sourceId = typeof d.source === 'object' ? d.source.id : d.source;
                const targetId = typeof d.target === 'object' ? d.target.id : d.target;
                return !visibleNodes.has(sourceId) || !visibleNodes.has(targetId);
            });
        }

        // Update multi-edge label visibility
        if (typeof multiEdgeLabelContainers !== 'undefined') {
            multiEdgeLabelContainers.forEach(({ container, group }) => {
                const isFiltered = visibleNodes && (!visibleNodes.has(group.nodeA) || !visibleNodes.has(group.nodeB));
                container.classed("filtered-out", isFiltered);
            });
        }

        // Update curved edges visibility
        if (typeof curvedEdges !== 'undefined') {
            curvedEdges.forEach(({ link, path, group }) => {
                const isFiltered = visibleNodes && (!visibleNodes.has(group.nodeA) || !visibleNodes.has(group.nodeB));
                path.classed("filtered-out", isFiltered);
            });
        }

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

    // Lock positions checkbox
    document.getElementById("lock-positions").addEventListener("change", function() {
        positionsLocked = this.checked;
        if (positionsLocked) {
            // Stop the simulation and fix all nodes at current positions
            simulation.stop();
            graphData.nodes.forEach(n => {
                n.fx = n.x;
                n.fy = n.y;
            });
        } else {
            // Unfix all nodes and restart simulation
            graphData.nodes.forEach(n => {
                n.fx = null;
                n.fy = null;
            });
            simulation.alpha(0.3).restart();
        }
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

        // Reverse arrowhead (for bidirectional unified edges)
        defs.append("marker")
            .attr("id", "arrowhead-reverse")
            .attr("viewBox", "0 -5 10 10")
            .attr("refX", -15)
            .attr("refY", 0)
            .attr("markerWidth", 6)
            .attr("markerHeight", 6)
            .attr("orient", "auto")
            .append("path")
            .attr("d", "M10,-5L0,0L10,5")
            .attr("fill", "#999");

        // Arrowhead for curved edges (refX=0 since we'll adjust the path endpoint)
        defs.append("marker")
            .attr("id", "arrowhead-curved")
            .attr("viewBox", "0 -5 10 10")
            .attr("refX", 10)
            .attr("refY", 0)
            .attr("markerWidth", 6)
            .attr("markerHeight", 6)
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

    // Clustering forces - attract nodes within same cluster, repel different clusters
    const clusterAttractionStrength = 0.15;
    const clusterRepulsionStrength = 0.8;
    const clusterRepulsionDistance = 200; // Minimum distance between cluster centers

    if (graphData.subgraphs && graphData.subgraphs.length > 0) {
        // Build node lookup by id for quick access
        const nodeById = new Map(graphData.nodes.map(n => [n.id, n]));

        simulation.force("cluster", function(alpha) {
            // First pass: calculate centroid for each subgraph
            const centroids = [];
            graphData.subgraphs.forEach((sg, i) => {
                if (!sg.nodes || sg.nodes.length === 0) return;

                let cx = 0, cy = 0, count = 0;
                sg.nodes.forEach(nodeId => {
                    const node = nodeById.get(nodeId);
                    if (node && node.x !== undefined && node.y !== undefined) {
                        cx += node.x;
                        cy += node.y;
                        count++;
                    }
                });

                if (count > 0) {
                    centroids.push({ sg, cx: cx / count, cy: cy / count, count, index: i });
                }
            });

            // Second pass: apply attraction toward own centroid
            centroids.forEach(({ sg, cx, cy }) => {
                sg.nodes.forEach(nodeId => {
                    const node = nodeById.get(nodeId);
                    if (node && node.x !== undefined && node.y !== undefined) {
                        node.vx += (cx - node.x) * alpha * clusterAttractionStrength;
                        node.vy += (cy - node.y) * alpha * clusterAttractionStrength;
                    }
                });
            });

            // Third pass: apply repulsion between cluster centroids
            for (let i = 0; i < centroids.length; i++) {
                for (let j = i + 1; j < centroids.length; j++) {
                    const c1 = centroids[i];
                    const c2 = centroids[j];

                    const dx = c2.cx - c1.cx;
                    const dy = c2.cy - c1.cy;
                    const dist = Math.sqrt(dx * dx + dy * dy) || 1;

                    // Apply repulsion if clusters are too close
                    if (dist < clusterRepulsionDistance) {
                        const force = (clusterRepulsionDistance - dist) / dist * alpha * clusterRepulsionStrength;
                        const fx = dx * force;
                        const fy = dy * force;

                        // Apply repulsion force to all nodes in each cluster
                        c1.sg.nodes.forEach(nodeId => {
                            const node = nodeById.get(nodeId);
                            if (node && node.x !== undefined) {
                                node.vx -= fx / c1.count;
                                node.vy -= fy / c1.count;
                            }
                        });
                        c2.sg.nodes.forEach(nodeId => {
                            const node = nodeById.get(nodeId);
                            if (node && node.x !== undefined) {
                                node.vx += fx / c2.count;
                                node.vy += fy / c2.count;
                            }
                        });
                    }
                }
            }
        });
    }

    // Check if path highlighting is active
    const hasPath = graphData.nodes.some(n => n.onPath) || graphData.links.some(l => l.onPath);

    // Normalize color values - converts various formats to CSS-compatible colors
    function normalizeColor(color) {
        if (!color) return null;
        // Convert 0xRRGGBB or 0xRGB format to #RRGGBB or #RGB
        if (typeof color === 'string' && color.toLowerCase().startsWith('0x')) {
            return '#' + color.slice(2);
        }
        // Add # prefix if it looks like a hex code without one (6 or 3 hex chars)
        if (typeof color === 'string' && /^[0-9a-fA-F]{6}$/.test(color)) {
            return '#' + color;
        }
        if (typeof color === 'string' && /^[0-9a-fA-F]{3}$/.test(color)) {
            return '#' + color;
        }
        return color;
    }

    // Safe color darkening - returns fallback if color is invalid
    function safeColorDarker(color, amount, fallback) {
        const parsed = d3.color(color);
        if (parsed) {
            return parsed.darker(amount);
        }
        return fallback || color;
    }

    // Color scale for clusters without explicit colors
    const clusterColorScale = d3.scaleOrdinal(d3.schemeSet2);

    // Draw cluster hulls (convex hulls around subgraph nodes)
    // Build node lookup for hull calculations
    const nodeByIdForHull = new Map(graphData.nodes.map(n => [n.id, n]));

    // Helper function to compute expanded convex hull with padding
    function computeHullPath(nodeIds, padding = 30) {
        const points = [];
        nodeIds.forEach(id => {
            const node = nodeByIdForHull.get(id);
            if (node && node.x !== undefined && node.y !== undefined) {
                // Add points around each node to create padding
                const offsets = [
                    [0, -padding], [padding, 0], [0, padding], [-padding, 0],
                    [padding * 0.7, -padding * 0.7], [padding * 0.7, padding * 0.7],
                    [-padding * 0.7, padding * 0.7], [-padding * 0.7, -padding * 0.7]
                ];
                offsets.forEach(([dx, dy]) => {
                    points.push([node.x + dx, node.y + dy]);
                });
            }
        });

        if (points.length < 3) return null;

        const hull = d3.polygonHull(points);
        if (!hull) return null;

        // Create smooth path using curve
        return d3.line().curve(d3.curveCatmullRomClosed.alpha(0.5))(hull);
    }

    // Create hull group (drawn first so it's behind everything)
    const hullGroup = g.append("g").attr("class", "cluster-hulls");
    const labelGroup = g.append("g").attr("class", "cluster-labels");

    // Create hull paths and labels for each subgraph
    const clusterHulls = [];
    const clusterLabels = [];
    if (graphData.subgraphs && graphData.subgraphs.length > 0) {
        graphData.subgraphs.forEach((sg, i) => {
            if (!sg.nodes || sg.nodes.length === 0) return;

            const hullColor = normalizeColor(sg.color) || clusterColorScale(sg.id || i);
            const isFilled = sg.style === 'filled';

            const hullPath = hullGroup.append("path")
                .attr("class", "cluster-hull" + (isFilled ? " filled" : ""))
                .attr("fill", hullColor)
                .attr("stroke", hullColor)
                .datum(sg);

            clusterHulls.push({ sg, path: hullPath });

            // Add label if present
            if (sg.label) {
                const label = labelGroup.append("text")
                    .attr("class", "cluster-label")
                    .text(sg.label)
                    .style("fill", d3.color(hullColor).darker(1));

                clusterLabels.push({ sg, label });
            }
        });
    }

    // Function to update hull paths
    function updateHulls() {
        clusterHulls.forEach(({ sg, path }) => {
            const pathData = computeHullPath(sg.nodes);
            if (pathData) {
                path.attr("d", pathData);
            }
        });

        // Update label positions (top of hull)
        clusterLabels.forEach(({ sg, label }) => {
            let minY = Infinity, sumX = 0, count = 0;
            sg.nodes.forEach(id => {
                const node = nodeByIdForHull.get(id);
                if (node && node.x !== undefined && node.y !== undefined) {
                    sumX += node.x;
                    count++;
                    if (node.y < minY) minY = node.y;
                }
            });
            if (count > 0) {
                label.attr("x", sumX / count)
                     .attr("y", minY - 40)
                     .attr("text-anchor", "middle");
            }
        });
    }

    // Detect multi-edge pairs and classify them
    const edgePairs = new Map(); // key: "A|B" (sorted), value: { links: [], directions: Set }
    graphData.links.forEach((l, i) => {
        const sourceId = typeof l.source === 'object' ? l.source.id : l.source;
        const targetId = typeof l.target === 'object' ? l.target.id : l.target;
        const sortedKey = [sourceId, targetId].sort().join('|');
        const directionKey = sourceId + '->' + targetId;

        if (!edgePairs.has(sortedKey)) {
            edgePairs.set(sortedKey, { links: [], directions: new Set(), sourceId: sortedKey.split('|')[0], targetId: sortedKey.split('|')[1] });
        }
        const pair = edgePairs.get(sortedKey);
        pair.links.push(i);
        pair.directions.add(directionKey);

        l._index = i;
        l._pairKey = sortedKey;
    });

    // Separate single-edge and multi-edge links
    const singleEdgeLinks = [];
    const multiEdgeGroups = [];

    edgePairs.forEach((pair, key) => {
        if (pair.links.length === 1) {
            singleEdgeLinks.push(graphData.links[pair.links[0]]);
        } else {
            // Check if bidirectional (has edges in both directions)
            const [nodeA, nodeB] = key.split('|');
            const isBidirectional = pair.directions.has(nodeA + '->' + nodeB) && pair.directions.has(nodeB + '->' + nodeA);
            multiEdgeGroups.push({
                key,
                nodeA,
                nodeB,
                linkIndices: pair.links,
                links: pair.links.map(i => graphData.links[i]),
                isBidirectional
            });
        }
    });

    // State for highlighted edge
    let highlightedEdgeIndex = null;

    // Draw single-edge links (unchanged behavior)
    const link = g.append("g")
        .attr("class", "links")
        .selectAll("line")
        .data(singleEdgeLinks)
        .join("line")
        .attr("class", d => graphData.directed ? "link directed" : "link")
        .classed("on-path", d => d.onPath)
        .classed("dimmed", d => hasPath && !d.onPath)
        .attr("stroke", d => normalizeColor(d.color) || "#999")
        .attr("stroke-width", 2)
        .attr("stroke-dasharray", d => d.style === "dashed" ? "5,5" : null)
        .on("click", function(event, d) {
            event.stopPropagation();
            if (highlightedEdgeIndex === d._index) {
                highlightedEdgeIndex = null;
            } else {
                highlightedEdgeIndex = d._index;
            }
            updateEdgeHighlight();

            const customEvent = new CustomEvent("edgeClick", {
                detail: {
                    source: typeof d.source === 'object' ? d.source.id : d.source,
                    target: typeof d.target === 'object' ? d.target.id : d.target,
                    label: d.label,
                    color: d.color,
                    highlighted: highlightedEdgeIndex === d._index
                },
                bubbles: true
            });
            document.dispatchEvent(customEvent);
        });

    // Draw unified lines for multi-edge groups
    const unifiedLinkGroup = g.append("g").attr("class", "unified-links");
    const curvedEdgeGroup = g.append("g").attr("class", "curved-edges");

    const unifiedLinks = unifiedLinkGroup.selectAll("line")
        .data(multiEdgeGroups)
        .join("line")
        .attr("class", d => {
            let cls = "unified-link";
            if (graphData.directed) cls += " directed";
            if (d.isBidirectional) cls += " bidirectional";
            return cls;
        })
        .attr("stroke", "#999")
        .attr("stroke-width", 2);

    // Draw curved paths for each edge in multi-edge groups (initially hidden)
    const curvedEdges = [];
    multiEdgeGroups.forEach(group => {
        // Track how many edges go in each direction for offset calculation
        const directionCounts = { forward: 0, backward: 0 };

        group.links.forEach((link) => {
            const sourceId = typeof link.source === 'object' ? link.source.id : link.source;
            const targetId = typeof link.target === 'object' ? link.target.id : link.target;

            // Determine if this edge goes "forward" (nodeA -> nodeB) or "backward" (nodeB -> nodeA)
            // based on the sorted key order
            const isForward = sourceId === group.nodeA;

            // Curve direction: forward edges curve one way, backward edges curve the other
            const baseDirection = isForward ? 1 : -1;

            // For multiple edges in the same direction, offset them further
            const dirKey = isForward ? 'forward' : 'backward';
            const sameDirectionIndex = directionCounts[dirKey];
            directionCounts[dirKey]++;

            // Offset increases for each additional edge in the same direction
            const curveOffset = 40 + sameDirectionIndex * 25;
            const curveDirection = baseDirection;

            const path = curvedEdgeGroup.append("path")
                .datum(link)
                .attr("class", "curved-edge")
                .attr("stroke", normalizeColor(link.color) || "#ff6b00")
                .attr("stroke-width", 3);

            curvedEdges.push({
                link,
                path,
                curveDirection,
                curveOffset,
                group
            });

            link._curvedEdge = { path, curveDirection, curveOffset };
        });
    });

    // Draw labels for single-edge links
    const singleEdgeLabels = singleEdgeLinks.filter(d => d.label);
    const linkLabel = g.append("g")
        .attr("class", "link-labels")
        .selectAll("text")
        .data(singleEdgeLabels)
        .join("text")
        .attr("class", "link-label")
        .classed("dimmed", d => hasPath && !d.onPath)
        .text(d => d.label)
        .on("click", function(event, d) {
            event.stopPropagation();
            if (highlightedEdgeIndex === d._index) {
                highlightedEdgeIndex = null;
            } else {
                highlightedEdgeIndex = d._index;
            }
            updateEdgeHighlight();

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

    // Draw stacked labels for multi-edge groups
    const multiEdgeLabelGroup = g.append("g").attr("class", "multi-edge-label-groups");
    const multiEdgeLabelContainers = [];

    multiEdgeGroups.forEach(group => {
        const container = multiEdgeLabelGroup.append("g")
            .attr("class", "multi-edge-labels")
            .datum(group);

        const labelsWithData = group.links
            .filter(l => l.label)
            .map((l, idx) => ({ link: l, idx }));

        const labels = container.selectAll("text")
            .data(labelsWithData)
            .join("text")
            .attr("class", "multi-edge-label")
            .classed("dimmed", d => hasPath && !d.link.onPath)
            .text(d => d.link.label)
            .attr("text-anchor", "middle")
            .on("click", function(event, d) {
                event.stopPropagation();
                if (highlightedEdgeIndex === d.link._index) {
                    highlightedEdgeIndex = null;
                } else {
                    highlightedEdgeIndex = d.link._index;
                }
                updateEdgeHighlight();

                const customEvent = new CustomEvent("edgeLabelClick", {
                    detail: {
                        source: typeof d.link.source === 'object' ? d.link.source.id : d.link.source,
                        target: typeof d.link.target === 'object' ? d.link.target.id : d.link.target,
                        label: d.link.label,
                        highlighted: highlightedEdgeIndex === d.link._index
                    },
                    bubbles: true
                });
                document.dispatchEvent(customEvent);
            });

        multiEdgeLabelContainers.push({ container, labels, group });
    });

    function updateEdgeHighlight() {
        // Update single-edge highlights
        link.classed("highlighted", d => d._index === highlightedEdgeIndex);
        linkLabel.classed("highlighted", d => d._index === highlightedEdgeIndex);

        // Update multi-edge highlights
        multiEdgeLabelContainers.forEach(({ labels }) => {
            labels.classed("highlighted", d => d.link._index === highlightedEdgeIndex);
        });

        // Show/hide curved edges (and their arrowheads)
        curvedEdges.forEach(({ link, path }) => {
            const isSelected = link._index === highlightedEdgeIndex;
            path.classed("visible", isSelected);
            path.classed("directed", isSelected && graphData.directed);
        });
    }

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
        // fillColor takes precedence, then color, then auto-generated
        const autoColor = colorScale(d.group || d.id);
        const fillColor = normalizeColor(d.fillColor) || normalizeColor(d.color) || autoColor;
        // stroke color: explicit color, or darker version of fill
        const strokeColor = normalizeColor(d.color) || safeColorDarker(fillColor, 0.5, '#666');

        if (shape === "box" || shape === "rect" || shape === "rectangle" || shape === "square") {
            el.append("rect")
                .attr("width", 50)
                .attr("height", 30)
                .attr("x", -25)
                .attr("y", -15)
                .attr("rx", 4)
                .attr("fill", fillColor)
                .attr("stroke", strokeColor)
                .attr("stroke-width", 1.5);
        } else if (shape === "diamond") {
            el.append("polygon")
                .attr("points", "0,-20 20,0 0,20 -20,0")
                .attr("fill", fillColor)
                .attr("stroke", strokeColor)
                .attr("stroke-width", 1.5);
        } else {
            // Default: ellipse/circle
            el.append("ellipse")
                .attr("rx", 25)
                .attr("ry", 18)
                .attr("fill", fillColor)
                .attr("stroke", strokeColor)
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
            if (!positionsLocked) {
                if (!event.active) simulation.alphaTarget(0.3).restart();
            }
            event.subject.fx = event.subject.x;
            event.subject.fy = event.subject.y;
        }

        function dragged(event) {
            event.subject.fx = event.x;
            event.subject.fy = event.y;
            // When locked, manually update the visual position since simulation isn't running
            if (positionsLocked) {
                event.subject.x = event.x;
                event.subject.y = event.y;
                // Update all edge positions
                updateEdgePositions();
                node.attr("transform", d => ` + "`" + `translate(${d.x},${d.y})` + "`" + `);
                // Update cluster hulls
                updateHulls();
            }
        }

        function dragended(event) {
            if (!positionsLocked) {
                if (!event.active) simulation.alphaTarget(0);
                event.subject.fx = null;
                event.subject.fy = null;
            }
            // When locked, keep the node fixed at its new position
        }

        return d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended);
    }

    // Helper to get node position by id
    const nodeById = new Map(graphData.nodes.map(n => [n.id, n]));

    function getNodePos(nodeIdOrObj) {
        if (typeof nodeIdOrObj === 'object') return { x: nodeIdOrObj.x, y: nodeIdOrObj.y };
        const node = nodeById.get(nodeIdOrObj);
        return node ? { x: node.x, y: node.y } : { x: 0, y: 0 };
    }

    // Helper to compute quadratic bezier curve path with shortened endpoints
    function computeCurvedPath(sourcePos, targetPos, curveDirection, curveOffset) {
        const dx = targetPos.x - sourcePos.x;
        const dy = targetPos.y - sourcePos.y;
        const len = Math.sqrt(dx * dx + dy * dy) || 1;

        // Unit vector along the line
        const ux = dx / len;
        const uy = dy / len;

        // Perpendicular unit vector
        const perpX = -uy;
        const perpY = ux;

        // Shorten endpoints to stop at node edge (node radius ~25px)
        const nodeRadius = 25;
        const startX = sourcePos.x + ux * nodeRadius;
        const startY = sourcePos.y + uy * nodeRadius;
        const endX = targetPos.x - ux * nodeRadius;
        const endY = targetPos.y - uy * nodeRadius;

        // Midpoint of shortened line
        const midX = (startX + endX) / 2;
        const midY = (startY + endY) / 2;

        // Control point offset perpendicular to the line
        const ctrlX = midX + perpX * curveOffset * curveDirection;
        const ctrlY = midY + perpY * curveOffset * curveDirection;

        return ` + "`" + `M${startX},${startY} Q${ctrlX},${ctrlY} ${endX},${endY}` + "`" + `;
    }

    // Function to update all edge positions
    function updateEdgePositions() {
        // Update single-edge links
        link
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        // Update unified links for multi-edge groups
        unifiedLinks.each(function(group) {
            const nodeA = getNodePos(group.nodeA);
            const nodeB = getNodePos(group.nodeB);
            d3.select(this)
                .attr("x1", nodeA.x)
                .attr("y1", nodeA.y)
                .attr("x2", nodeB.x)
                .attr("y2", nodeB.y);
        });

        // Update curved edges
        curvedEdges.forEach(({ link, path, curveDirection, curveOffset }) => {
            const sourcePos = getNodePos(link.source);
            const targetPos = getNodePos(link.target);
            path.attr("d", computeCurvedPath(sourcePos, targetPos, curveDirection, curveOffset));
        });

        // Position single-edge labels at midpoint
        linkLabel.attr("transform", d => {
            const midX = (d.source.x + d.target.x) / 2;
            const midY = (d.source.y + d.target.y) / 2;
            return ` + "`" + `translate(${midX},${midY})` + "`" + `;
        });

        // Position multi-edge label groups (stacked vertically at midpoint)
        multiEdgeLabelContainers.forEach(({ container, labels, group }) => {
            const nodeA = getNodePos(group.nodeA);
            const nodeB = getNodePos(group.nodeB);
            const midX = (nodeA.x + nodeB.x) / 2;
            const midY = (nodeA.y + nodeB.y) / 2;

            // Count labels with content
            const labelCount = labels.size();
            const lineHeight = 14;
            const startY = -(labelCount - 1) * lineHeight / 2;

            container.attr("transform", ` + "`" + `translate(${midX},${midY})` + "`" + `);
            labels.attr("y", (d, i) => startY + i * lineHeight);
        });
    }

    // Update positions on tick
    simulation.on("tick", () => {
        // Update cluster hulls first (so they're behind everything)
        updateHulls();

        // Update all edge positions
        updateEdgePositions();

        node.attr("transform", d => ` + "`" + `translate(${d.x},${d.y})` + "`" + `);
    });

    // Listen for events (example usage)
    document.addEventListener("nodeClick", function(e) {
        console.log("nodeClick event:", e.detail);
    });

    document.addEventListener("edgeLabelClick", function(e) {
        console.log("edgeLabelClick event:", e.detail);
    });

    document.addEventListener("edgeClick", function(e) {
        console.log("edgeClick event:", e.detail);
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
    </script>
</body>
</html>`
