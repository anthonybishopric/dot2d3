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
	// Shared links now always show the form with pre-populated content
	// The JavaScript will decode URL params and fill in the form fields
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
    .button-row {
        display: flex;
        gap: 10px;
        margin-top: 12px;
    }
    .button-row button {
        margin-top: 0;
    }
    .copy-link-btn {
        background: #5cb85c;
        display: flex;
        align-items: center;
        gap: 6px;
    }
    .copy-link-btn:hover { background: #4cae4c; }
    .copy-link-btn.copied { background: #337ab7; }
    .copy-link-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }
    .copy-feedback {
        font-size: 12px;
        color: #3c763d;
        margin-top: 6px;
        min-height: 18px;
    }
    .copy-feedback.error { color: #c9302c; }
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
            <div class="button-row">
                <button type="submit">Convert</button>
                <button type="button" class="copy-link-btn" id="copy-link-btn">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
                        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
                    </svg>
                    Copy Link
                </button>
            </div>
            <div class="copy-feedback" id="copy-feedback"></div>
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
// LZ-String v1.5.0 (MIT License) - https://github.com/pieroxy/lz-string
var LZString=function(){var r=String.fromCharCode,o="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=",n="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+-$",e={};function t(r,o){if(!e[r]){e[r]={};for(var n=0;n<r.length;n++)e[r][r.charAt(n)]=n}return e[r][o]}var i={compressToBase64:function(r){if(null==r)return"";var n=i._compress(r,6,function(r){return o.charAt(r)});switch(n.length%4){default:case 0:return n;case 1:return n+"===";case 2:return n+"==";case 3:return n+"="}},decompressFromBase64:function(r){return null==r?"":""==r?null:i._decompress(r.length,32,function(n){return t(o,r.charAt(n))})},compressToUTF16:function(o){return null==o?"":i._compress(o,15,function(o){return r(o+32)})+" "},decompressFromUTF16:function(r){return null==r?"":""==r?null:i._decompress(r.length,16384,function(o){return r.charCodeAt(o)-32})},compressToUint8Array:function(r){for(var o=i.compress(r),n=new Uint8Array(2*o.length),e=0,t=o.length;e<t;e++){var s=o.charCodeAt(e);n[2*e]=s>>>8,n[2*e+1]=s%256}return n},decompressFromUint8Array:function(o){if(null==o)return i.decompress(o);for(var n=new Array(o.length/2),e=0,t=n.length;e<t;e++)n[e]=256*o[2*e]+o[2*e+1];var s=[];return n.forEach(function(o){s.push(r(o))}),i.decompress(s.join(""))},compressToEncodedURIComponent:function(r){return null==r?"":i._compress(r,6,function(r){return n.charAt(r)})},decompressFromEncodedURIComponent:function(r){return null==r?"":""==r?null:(r=r.replace(/ /g,"+"),i._decompress(r.length,32,function(o){return t(n,r.charAt(o))}))},compress:function(o){return i._compress(o,16,function(o){return r(o)})},_compress:function(r,o,n){if(null==r)return"";var e,t,i,s={},u={},a="",p="",c="",l=2,f=3,h=2,d=[],m=0,v=0;for(i=0;i<r.length;i+=1)if(a=r.charAt(i),Object.prototype.hasOwnProperty.call(s,a)||(s[a]=f++,u[a]=!0),p=c+a,Object.prototype.hasOwnProperty.call(s,p))c=p;else{if(Object.prototype.hasOwnProperty.call(u,c)){if(c.charCodeAt(0)<256){for(e=0;e<h;e++)m<<=1,v==o-1?(v=0,d.push(n(m)),m=0):v++;for(t=c.charCodeAt(0),e=0;e<8;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1}else{for(t=1,e=0;e<h;e++)m=m<<1|t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t=0;for(t=c.charCodeAt(0),e=0;e<16;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1}0==--l&&(l=Math.pow(2,h),h++),delete u[c]}else for(t=s[c],e=0;e<h;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1;0==--l&&(l=Math.pow(2,h),h++),s[p]=f++,c=String(a)}if(""!==c){if(Object.prototype.hasOwnProperty.call(u,c)){if(c.charCodeAt(0)<256){for(e=0;e<h;e++)m<<=1,v==o-1?(v=0,d.push(n(m)),m=0):v++;for(t=c.charCodeAt(0),e=0;e<8;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1}else{for(t=1,e=0;e<h;e++)m=m<<1|t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t=0;for(t=c.charCodeAt(0),e=0;e<16;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1}0==--l&&(l=Math.pow(2,h),h++),delete u[c]}else for(t=s[c],e=0;e<h;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1;0==--l&&(l=Math.pow(2,h),h++)}for(t=2,e=0;e<h;e++)m=m<<1|1&t,v==o-1?(v=0,d.push(n(m)),m=0):v++,t>>=1;for(;;){if(m<<=1,v==o-1){d.push(n(m));break}v++}return d.join("")},decompress:function(r){return null==r?"":""==r?null:i._decompress(r.length,32768,function(o){return r.charCodeAt(o)})},_decompress:function(o,n,e){var t,i,s,u,a,p,c,l=[],f=4,h=4,d=3,m="",v=[],g={val:e(0),position:n,index:1};for(t=0;t<3;t+=1)l[t]=t;for(s=0,a=Math.pow(2,2),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;switch(s){case 0:for(s=0,a=Math.pow(2,8),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;c=r(s);break;case 1:for(s=0,a=Math.pow(2,16),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;c=r(s);break;case 2:return""}for(l[3]=c,i=c,v.push(c);;){if(g.index>o)return"";for(s=0,a=Math.pow(2,d),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;switch(c=s){case 0:for(s=0,a=Math.pow(2,8),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;l[h++]=r(s),c=h-1,f--;break;case 1:for(s=0,a=Math.pow(2,16),p=1;p!=a;)u=g.val&g.position,g.position>>=1,0==g.position&&(g.position=n,g.val=e(g.index++)),s|=(u>0?1:0)*p,p<<=1;l[h++]=r(s),c=h-1,f--;break;case 2:return v.join("")}if(0==f&&(f=Math.pow(2,d),d++),l[c])m=l[c];else{if(c!==h)return null;m=i+i.charAt(0)}v.push(m),l[h++]=i+m.charAt(0),i=m,0==--f&&(f=Math.pow(2,d),d++)}}};return i}();

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

// Pre-populate form from URL params (for when viewing shared link editor)
// Supports both old base64 format and new LZ-String compressed format
function fromUrlSafeBase64(str) {
    if (!str) return '';
    try {
        // Restore standard base64 characters
        let s = str.replace(/-/g, '+').replace(/_/g, '/');
        // Add padding if needed
        while (s.length % 4) s += '=';
        return decodeURIComponent(escape(atob(s)));
    } catch (e) {
        return '';
    }
}

function decompress(str) {
    if (!str) return '';
    // Try LZ-String decompression first (new format)
    try {
        const result = LZString.decompressFromEncodedURIComponent(str);
        if (result && result.length > 0) return result;
    } catch (e) {}
    // Fall back to base64 decoding (old format for backward compatibility)
    return fromUrlSafeBase64(str);
}

(function() {
    const params = new URLSearchParams(window.location.search);
    const graphParam = params.get('g');
    const pathParam = params.get('p');

    if (graphParam) {
        const graphDOT = decompress(graphParam);
        if (graphDOT) {
            document.querySelector('textarea[name="graph"]').value = graphDOT;
        }
    }
    if (pathParam) {
        const pathDOT = decompress(pathParam);
        if (pathDOT) {
            document.querySelector('textarea[name="path"]').value = pathDOT;
        }
    }
})();

// Copy Link functionality
const copyLinkBtn = document.getElementById('copy-link-btn');
const copyFeedback = document.getElementById('copy-feedback');

function compress(str) {
    // Use LZ-String compression for URL-safe output
    return LZString.compressToEncodedURIComponent(str);
}

copyLinkBtn.addEventListener('click', async function() {
    const graphDOT = document.querySelector('textarea[name="graph"]').value;
    const pathDOT = document.querySelector('textarea[name="path"]').value;

    if (!graphDOT.trim()) {
        copyFeedback.textContent = 'Please enter a graph first';
        copyFeedback.className = 'copy-feedback error';
        return;
    }

    // Build shareable URL with LZ-String compression
    const params = new URLSearchParams();
    params.set('g', compress(graphDOT));
    if (pathDOT.trim()) {
        params.set('p', compress(pathDOT));
    }

    const shareURL = window.location.origin + '/?' + params.toString();

    try {
        await navigator.clipboard.writeText(shareURL);
        copyLinkBtn.classList.add('copied');
        copyLinkBtn.innerHTML = ` + "`" + `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="20 6 9 17 4 12"></polyline>
            </svg>
            Copied!
        ` + "`" + `;
        copyFeedback.textContent = 'Link copied to clipboard';
        copyFeedback.className = 'copy-feedback';

        setTimeout(() => {
            copyLinkBtn.classList.remove('copied');
            copyLinkBtn.innerHTML = ` + "`" + `
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path>
                    <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path>
                </svg>
                Copy Link
            ` + "`" + `;
            copyFeedback.textContent = '';
        }, 2000);
    } catch (err) {
        // Fallback - show the URL
        copyFeedback.innerHTML = '<a href="' + shareURL + '" target="_blank" style="word-break:break-all;">Open link</a>';
        copyFeedback.className = 'copy-feedback';
    }
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
