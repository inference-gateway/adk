// Command wrap-a2a-schema turns the raw A2A JSON Schema (draft-07, top-level
// `definitions`) into a minimal OpenAPI 3.1 document that oapi-codegen can
// consume: `definitions` becomes `components.schemas` and every
// `#/definitions/...` ref is rewritten to `#/components/schemas/...`. It is a
// deterministic transform with no new dependency (gopkg.in/yaml.v3 is already
// a direct module dependency).
//
// Usage: wrap-a2a-schema <input schema.yaml> <output openapi.yaml>
package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: wrap-a2a-schema <input> <output>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(input, output string) error {
	data, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("reading A2A schema %s: %w", input, err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing A2A schema: %w", err)
	}

	schemas, ok := doc["definitions"].(map[string]any)
	if !ok {
		return fmt.Errorf("A2A schema %s: definitions must be a mapping", input)
	}

	dropMultiTypeArrays(schemas)
	annotateLooseObjects(schemas)
	hoisted := map[string]any{}
	hoistTitledEnums(hoisted, schemas)
	for name, schema := range hoisted {
		schemas[name] = schema
	}
	// Struct is a bare `type: object`; pin it to a map alias so it stays
	// assignable to/from plain maps, matching the previous generator output.
	if s, ok := schemas["Struct"].(map[string]any); ok {
		s["x-go-type"] = "map[string]interface{}"
	}

	wrapped := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "A2A JSON-RPC Schema",
			"version": "1.0.0",
		},
		"paths": map[string]any{},
		"components": map[string]any{
			"schemas": schemas,
		},
	}

	out, err := yaml.Marshal(wrapped)
	if err != nil {
		return fmt.Errorf("marshalling wrapped schema: %w", err)
	}
	out = bytes.ReplaceAll(out, []byte("#/definitions/"), []byte("#/components/schemas/"))

	if err := os.WriteFile(output, out, 0o644); err != nil {
		return fmt.Errorf("writing wrapped schema %s: %w", output, err)
	}
	return nil
}

// hoistTitledEnums lifts every inline `enum` that carries a `title` into a
// named top-level schema keyed by the PascalCase title (e.g. "Task State" ->
// TaskState) and replaces the inline definition with a $ref to it. It also pins
// each constant name via x-enum-varnames so oapi-codegen reproduces the
// previous generator's `<Type><Value>` idiom (RoleAgent, TaskStateWorking, ...)
// instead of its default `TYPEVALUE` shouting form. `root` is the schemas map
// the hoisted entries are added to; `node` is the subtree being walked.
func hoistTitledEnums(root map[string]any, node any) {
	switch v := node.(type) {
	case map[string]any:
		title, hasTitle := v["title"].(string)
		enum, hasEnum := v["enum"].([]any)
		if hasTitle && hasEnum {
			name := pascal(title)
			if _, exists := root[name]; !exists {
				root[name] = map[string]any{
					"type":            "string",
					"enum":            enum,
					"title":           title,
					"description":     v["description"],
					"x-enum-varnames": enumVarnames(name, enum),
				}
			}
			// Replace the inline enum with a reference to the hoisted schema.
			for k := range v {
				delete(v, k)
			}
			v["$ref"] = "#/components/schemas/" + name
			return
		}
		for _, child := range v {
			hoistTitledEnums(root, child)
		}
	case []any:
		for _, child := range v {
			hoistTitledEnums(root, child)
		}
	}
}

// pascal turns a title like "Task State" into "TaskState".
func pascal(s string) string {
	var b strings.Builder
	for _, word := range strings.Fields(s) {
		b.WriteString(strings.ToUpper(word[:1]))
		b.WriteString(word[1:])
	}
	return b.String()
}

// enumVarnames maps each SCREAMING_SNAKE enum value to `<Type><CamelSuffix>`,
// stripping the type-derived prefix (ROLE_ from ROLE_AGENT, TASK_STATE_ from
// TASK_STATE_WORKING) to match the previous generator's constant names.
func enumVarnames(typeName string, enum []any) []any {
	prefix := screaming(typeName) + "_"
	names := make([]any, 0, len(enum))
	for _, e := range enum {
		val, _ := e.(string)
		suffix := strings.TrimPrefix(val, prefix)
		names = append(names, typeName+camel(suffix))
	}
	return names
}

// screaming turns "TaskState" into "TASK_STATE".
func screaming(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

// camel turns "AUTH_REQUIRED" into "AuthRequired".
func camel(s string) string {
	var b strings.Builder
	for _, word := range strings.Split(s, "_") {
		if word == "" {
			continue
		}
		b.WriteString(strings.ToUpper(word[:1]))
		b.WriteString(strings.ToLower(word[1:]))
	}
	return b.String()
}

// annotateLooseObjects pins every object whose additionalProperties is the
// empty schema (`{}`) to map[string]interface{}, reproducing the previous
// generator's output.
func annotateLooseObjects(node any) {
	switch v := node.(type) {
	case map[string]any:
		if v["type"] == "object" {
			if ap, ok := v["additionalProperties"].(map[string]any); ok {
				// A map type (`map[string]T`) is already nilable; keep it
				// unpointered on optional fields, matching the old generator.
				v["x-go-type-skip-optional-pointer"] = true
				if len(ap) == 0 {
					v["x-go-type"] = "map[string]interface{}"
				}
			}
		}
		// Optional slices are already nilable; keep them as `[]T` (not `*[]T`)
		// to match the previous generator's output.
		if v["type"] == "array" {
			v["x-go-type-skip-optional-pointer"] = true
		}
		for _, child := range v {
			annotateLooseObjects(child)
		}
	case []any:
		for _, child := range v {
			annotateLooseObjects(child)
		}
	}
}

// dropMultiTypeArrays removes any `type` key whose value is a JSON Schema
// multi-type list, which oapi-codegen cannot render.
func dropMultiTypeArrays(node any) {
	switch v := node.(type) {
	case map[string]any:
		if _, isList := v["type"].([]any); isList {
			delete(v, "type")
		}
		for _, child := range v {
			dropMultiTypeArrays(child)
		}
	case []any:
		for _, child := range v {
			dropMultiTypeArrays(child)
		}
	}
}
