// Package assets embeds the static files and HTML templates.
package assets

import "embed"

// FS holds the entire assets tree (templates/ and static/).
//
//go:embed templates static
var FS embed.FS
