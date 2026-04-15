//go:build tools

// Package tools pins tool dependencies so they appear in go.mod/go.sum.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
