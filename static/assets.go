package static

import "embed"

// Files contains the compiled web interface served by management.
//
//go:embed index.html app.js lookup.js response.js trace.js graph.js physics.js style.css vendor
var Files embed.FS
