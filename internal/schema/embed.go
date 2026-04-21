// Package schema provides JSON Schema validation for entity types and a registry of valid relationship types.
package schema

import "embed"

//go:embed entities/*.json
var entitySchemasFS embed.FS
