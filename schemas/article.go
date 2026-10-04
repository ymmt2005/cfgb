// Package schemas provides optional standalone validation against bundled schemas.
// Configuration and front-matter loading do not call this package.
package schemas

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed *.schema.json
var files embed.FS

var articleSchema = sync.OnceValues(func() (*jsonschema.Schema, error) { return compile("article.schema.json") })
var configSchema = sync.OnceValues(func() (*jsonschema.Schema, error) { return compile("cfgb.schema.json") })

func compile(name string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	// Register bundled references locally; compilation never fetches schemas.
	for _, entry := range entries {
		raw, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if err := compiler.AddResource(entry.Name(), doc); err != nil {
			return nil, err
		}
	}
	return compiler.Compile(name)
}

// ValidateArticle runs structural validation, including date-time formats.
// Semantic requirements such as summary presence and alias ownership are separate.
func ValidateArticle(value any) error {
	schema, err := articleSchema()
	if err != nil {
		return fmt.Errorf("compile article schema: %w", err)
	}
	return validate(schema, value)
}

// ValidateConfig optionally checks a caller-supplied value, including URI formats.
// Loading/defaults use typed Go decoding; command-specific checks are separate.
func ValidateConfig(value any) error {
	schema, err := configSchema()
	if err != nil {
		return fmt.Errorf("compile configuration schema: %w", err)
	}
	return validate(schema, value)
}

func validate(schema *jsonschema.Schema, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	return schema.Validate(doc)
}
