package web

import "embed"

//go:embed static/index.html static/app.js static/style.css
var staticFS embed.FS
