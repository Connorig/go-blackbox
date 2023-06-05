package static

import "embed"

//go:embed index.html
var StaticFile embed.FS
