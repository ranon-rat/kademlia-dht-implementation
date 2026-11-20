package static

import "embed"

// Files contains the compiled web interface served by management.
//
//go:embed index.html app.js lookup.js response.js style.css
var Files embed.FS
