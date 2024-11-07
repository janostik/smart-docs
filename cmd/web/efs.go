package web

import "embed"

//go:embed "templates/*" "assets/*"
var Files embed.FS
