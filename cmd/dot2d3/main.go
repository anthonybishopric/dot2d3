// Command dot2d3 converts DOT files to interactive D3.js visualizations.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/anthonybishopric/dot2d3/pkg/dot"
)

var (
	outputFile = flag.String("o", "", "Output file (default: stdout)")
	title      = flag.String("t", "", "HTML page title (default: graph ID or 'Graph Visualization')")
	jsonOnly   = flag.Bool("json", false, "Output only JSON data (no HTML)")
	serve      = flag.String("serve", "", "Start HTTP server on specified address (e.g., ':8080' or 'localhost:8080')")
	help       = flag.Bool("h", false, "Show help")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `dot2d3 - Convert DOT files to interactive D3.js visualizations

Usage:
  dot2d3 [options] [input.dot]

If no input file is specified, reads from stdin.

Options:
`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  dot2d3 graph.dot > output.html
  dot2d3 -o output.html graph.dot
  dot2d3 -t "My Graph" -o output.html graph.dot
  dot2d3 --json graph.dot > graph.json
  echo 'digraph { A -> B -> C }' | dot2d3 > quick.html

Server mode:
  dot2d3 -serve :8080
  curl -X POST -d 'digraph { A -> B }' http://localhost:8080/convert > graph.html
  curl -X POST -d 'digraph { A -> B }' http://localhost:8080/convert?format=json

Features:
  - Clickable nodes (emits 'nodeClick' JavaScript events)
  - Draggable nodes
  - Zoomable/pannable graph (mouse wheel to zoom, drag to pan)
  - Double-click to reset zoom
  - Hover tooltips showing node attributes
  - Degree-of-separation filter slider
`)
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Server mode
	if *serve != "" {
		runServer(*serve)
		return
	}

	// CLI mode
	runCLI()
}

func runServer(addr string) {
	mux := http.NewServeMux()

	// POST /convert - accepts DOT in body, returns HTML (or JSON with ?format=json)
	mux.HandleFunc("POST /convert", handleConvert)

	// GET / - simple health/info endpoint
	mux.HandleFunc("GET /", handleIndex)

	log.Printf("Starting dot2d3 server on %s", addr)
	log.Printf("POST DOT content to http://%s/convert to get D3 HTML", addr)
	log.Printf("Add ?format=json for JSON output, ?title=MyTitle for custom title")

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>dot2d3 Server</title>
<style>
    body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        margin: 0;
        padding: 20px;
        background: #f5f5f5;
    }
    .container {
        display: flex;
        gap: 20px;
        height: calc(100vh - 40px);
    }
    .left-panel {
        width: 450px;
        flex-shrink: 0;
        overflow-y: auto;
    }
    .right-panel {
        flex: 1;
        display: flex;
        flex-direction: column;
    }
    h1 { margin-top: 0; }
    label {
        display: block;
        margin-top: 12px;
        margin-bottom: 4px;
        font-weight: 500;
        font-size: 14px;
    }
    label:first-of-type { margin-top: 0; }
    textarea {
        width: 100%;
        box-sizing: border-box;
        font-family: monospace;
        font-size: 14px;
        padding: 10px;
        border: 1px solid #ccc;
        border-radius: 4px;
    }
    button {
        margin-top: 12px;
        padding: 10px 20px;
        font-size: 14px;
        background: #4a90d9;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }
    button:hover { background: #357abd; }
    details {
        margin-top: 15px;
        font-size: 13px;
        color: #666;
    }
    summary { cursor: pointer; }
    pre {
        background: #fff;
        padding: 10px;
        border-radius: 4px;
        overflow-x: auto;
        font-size: 12px;
    }
    #preview {
        flex: 1;
        border: 1px solid #ccc;
        border-radius: 4px;
        background: white;
    }
    .placeholder {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        color: #999;
        font-size: 18px;
    }
    .error-display {
        padding: 20px;
        background: #fff3f3;
        border: 1px solid #f44336;
        border-radius: 4px;
        color: #c62828;
        font-family: monospace;
        font-size: 14px;
    }
    .hint {
        font-size: 12px;
        color: #888;
        margin-top: 4px;
    }
</style>
</head>
<body>
<div class="container">
    <div class="left-panel">
        <h1>dot2d3</h1>
        <p>Convert Graphviz DOT to interactive D3.js visualizations.</p>
        <form>
            <label for="graph">Graph (DOT format)</label>
            <textarea name="graph" id="graph" rows="12" placeholder="digraph G {
    A -> B -> C -> D
    B -> E [color=red]
}"></textarea>
            <label for="path">Path to Highlight (optional)</label>
            <textarea name="path" id="path" rows="4" placeholder="digraph { A -> B -> C }"></textarea>
            <div class="hint">Path edges highlighted in orange. Rest of graph is dimmed. Invalid nodes show in red.</div>
            <button type="submit">Convert</button>
        </form>
        <details>
            <summary>API Usage</summary>
            <pre>
POST /convert
  JSON body: {"graph": "...", "path": "..."}
  Query params:
    format=json  - Return JSON instead of HTML
    title=...    - Set the page title

Examples:
  curl -X POST -H "Content-Type: application/json" \
    -d '{"graph":"digraph{A->B->C}","path":"digraph{A->B}"}' \
    localhost:8080/convert

  # Plain text (backward compatible, no path):
  curl -X POST -d 'digraph { A -> B }' localhost:8080/convert
            </pre>
        </details>
    </div>
    <div class="right-panel">
        <iframe id="preview" frameborder="0">
            <div class="placeholder">Enter DOT and click Convert</div>
        </iframe>
    </div>
</div>
<script>
document.querySelector('form').addEventListener('submit', function(e) {
    e.preventDefault();
    const graphDOT = document.querySelector('textarea[name="graph"]').value;
    const pathDOT = document.querySelector('textarea[name="path"]').value;

    const body = JSON.stringify({
        graph: graphDOT,
        path: pathDOT || undefined
    });

    fetch('/convert', {
        method: 'POST',
        body: body,
        headers: {'Content-Type': 'application/json'}
    })
    .then(r => {
        if (r.headers.get('Content-Type')?.includes('application/json')) {
            return r.json().then(data => ({ isError: true, data }));
        }
        return r.text().then(html => ({ isError: false, html }));
    })
    .then(result => {
        const iframe = document.getElementById('preview');
        if (result.isError) {
            const err = result.data;
            iframe.srcdoc = '<div style="padding:20px;font-family:sans-serif;">' +
                '<h2 style="color:#c62828;margin-top:0;">Path Validation Error</h2>' +
                '<p><strong>Error:</strong> ' + err.error + '</p>' +
                (err.lastValidNode ? '<p><strong>Last valid node:</strong> ' + err.lastValidNode + '</p>' : '') +
                '</div>';
        } else {
            iframe.srcdoc = result.html;
        }
    })
    .catch(err => {
        const iframe = document.getElementById('preview');
        iframe.srcdoc = '<div style="padding:20px;color:#c62828;">' + err.message + '</div>';
    });
});
</script>
</body>
</html>`)
}

// ConvertRequest is the JSON request body for /convert endpoint.
type ConvertRequest struct {
	Graph string `json:"graph"`
	Path  string `json:"path,omitempty"`
}

// ConvertError is the JSON error response for path validation failures.
type ConvertError struct {
	Error         string                    `json:"error"`
	InvalidEdge   *dot.PathValidationResult `json:"invalidEdge,omitempty"`
	LastValidNode string                    `json:"lastValidNode,omitempty"`
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		http.Error(w, "Request body is empty. Please provide DOT content.", http.StatusBadRequest)
		return
	}

	// Determine if body is JSON or plain text DOT
	var graphDOT, pathDOT string
	contentType := r.Header.Get("Content-Type")
	isJSON := strings.Contains(contentType, "application/json") ||
		(len(body) > 0 && body[0] == '{')

	if isJSON {
		var req ConvertRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "Failed to parse JSON request: "+err.Error(), http.StatusBadRequest)
			return
		}
		graphDOT = req.Graph
		pathDOT = req.Path
	} else {
		// Plain text body is the graph DOT (backward compatible)
		graphDOT = string(body)
	}

	if graphDOT == "" {
		http.Error(w, "Graph DOT content is empty.", http.StatusBadRequest)
		return
	}

	// Parse main graph DOT
	graph, err := dot.Parse("request", []byte(graphDOT))
	if err != nil {
		http.Error(w, "Failed to parse graph DOT: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Build render options
	opts := dot.RenderOptions{
		Title: r.URL.Query().Get("title"),
	}

	if pathDOT != "" {
		pathAST, err := dot.Parse("path", []byte(pathDOT))
		if err != nil {
			http.Error(w, "Failed to parse path DOT: "+err.Error(), http.StatusBadRequest)
			return
		}
		opts.PathAST = pathAST
	}

	// Check query params for output format
	format := r.URL.Query().Get("format")

	// Generate output
	var output []byte
	var outputContentType string

	if format == "json" {
		output, err = dot.ToJSON(graph)
		outputContentType = "application/json"
		if err != nil {
			http.Error(w, "Failed to generate JSON: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Generate HTML with path validation
		var pathResult *dot.PathValidationResult
		output, pathResult, err = dot.ToHTMLWithValidation(graph, opts)
		outputContentType = "text/html; charset=utf-8"

		if err != nil {
			http.Error(w, "Failed to generate HTML: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// If path validation failed, return JSON error
		if pathResult != nil && !pathResult.Valid {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(pathResult)
			return
		}
	}

	w.Header().Set("Content-Type", outputContentType)
	w.Write(output)
}

func runCLI() {
	var input []byte
	var filename string
	var err error

	args := flag.Args()
	if len(args) == 0 || args[0] == "-" {
		// Read from stdin
		input, err = io.ReadAll(os.Stdin)
		filename = "<stdin>"
	} else {
		filename = args[0]
		input, err = os.ReadFile(filename)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Parse DOT
	graph, err := dot.Parse(filename, input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing DOT: %v\n", err)
		os.Exit(1)
	}

	// Generate output
	var output []byte
	if *jsonOnly {
		output, err = dot.ToJSON(graph)
	} else {
		opts := dot.RenderOptions{
			Title: *title,
		}
		output, err = dot.ToHTML(graph, opts)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating output: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if *outputFile == "" {
		fmt.Print(string(output))
	} else {
		if err := os.WriteFile(*outputFile, output, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Written to %s\n", *outputFile)
	}
}
