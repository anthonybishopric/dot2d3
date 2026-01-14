// Command dot2d3 converts DOT files to interactive D3.js visualizations.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/anthonybishopric/gographviz/pkg/dot"
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
        width: 400px;
        flex-shrink: 0;
    }
    .right-panel {
        flex: 1;
        display: flex;
        flex-direction: column;
    }
    h1 { margin-top: 0; }
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
        margin-top: 10px;
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
</style>
</head>
<body>
<div class="container">
    <div class="left-panel">
        <h1>dot2d3</h1>
        <p>Convert Graphviz DOT to interactive D3.js visualizations.</p>
        <form>
            <textarea name="dot" rows="15" placeholder="digraph G {
    A -> B -> C
    B -> D [color=red]
}"></textarea>
            <button type="submit">Convert</button>
        </form>
        <details>
            <summary>API Usage</summary>
            <pre>
POST /convert
  Body: DOT file content
  Query params:
    format=json  - Return JSON instead of HTML
    title=...    - Set the page title

Examples:
  curl -X POST -d 'digraph { A -> B }' localhost:8080/convert
  curl -X POST -d @graph.dot localhost:8080/convert?title=MyGraph
  curl -X POST -d 'digraph { A -> B }' localhost:8080/convert?format=json
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
    const dot = document.querySelector('textarea[name="dot"]').value;
    fetch('/convert', {
        method: 'POST',
        body: dot,
        headers: {'Content-Type': 'text/plain'}
    })
    .then(r => r.text())
    .then(html => {
        const iframe = document.getElementById('preview');
        iframe.srcdoc = html;
    });
});
</script>
</body>
</html>`)
}

func handleConvert(w http.ResponseWriter, r *http.Request) {
	// Read DOT content from request body
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

	// Parse DOT
	graph, err := dot.Parse("request", body)
	if err != nil {
		http.Error(w, "Failed to parse DOT: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Check query params
	format := r.URL.Query().Get("format")
	pageTitle := r.URL.Query().Get("title")

	// Generate output
	var output []byte
	var contentType string

	if format == "json" {
		output, err = dot.ToJSON(graph)
		contentType = "application/json"
	} else {
		opts := dot.RenderOptions{
			Title: pageTitle,
		}
		output, err = dot.ToHTML(graph, opts)
		contentType = "text/html; charset=utf-8"
	}

	if err != nil {
		http.Error(w, "Failed to generate output: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
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
