package web

import "embed"

//go:embed "assets"
//go:embed "templates"
var Files embed.FS
