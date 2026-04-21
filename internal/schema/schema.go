package schema

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var entitySchemas map[string]*jsonschema.Schema

func init() {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	entitySchemas = make(map[string]*jsonschema.Schema)

	entries, err := fs.ReadDir(entitySchemasFS, "entities")
	if err != nil {
		panic(fmt.Sprintf("schema: read entities dir: %v", err))
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := entitySchemasFS.ReadFile("entities/" + e.Name())
		if err != nil {
			panic(fmt.Sprintf("schema: read %s: %v", e.Name(), err))
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			panic(fmt.Sprintf("schema: parse %s: %v", e.Name(), err))
		}
		if err := c.AddResource("schema:"+name, doc); err != nil {
			panic(fmt.Sprintf("schema: add resource %s: %v", name, err))
		}
		compiled, err := c.Compile("schema:" + name)
		if err != nil {
			panic(fmt.Sprintf("schema: compile %s: %v", name, err))
		}
		entitySchemas[name] = compiled
	}
}

// ValidateEntity validates data against the JSONSchema for the given entity type.
func ValidateEntity(entityType string, data json.RawMessage) error {
	s, ok := entitySchemas[entityType]
	if !ok {
		return fmt.Errorf("unknown entity type %q", entityType)
	}
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if err := s.Validate(v); err != nil {
		return fmt.Errorf("entity %q schema validation failed: %w", entityType, err)
	}
	return nil
}

// KnownEntityTypes returns all registered entity type names in sorted order.
func KnownEntityTypes() []string {
	types := make([]string, 0, len(entitySchemas))
	for t := range entitySchemas {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
