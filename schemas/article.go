// Package schemas validates normalized inputs against the checked-in contracts.
package schemas

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed article.schema.json
var articleJSON []byte

var articleSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(articleJSON))
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource("article.schema.json", doc); err != nil {
		return nil, err
	}
	return compiler.Compile("article.schema.json")
})

// ValidateArticle runs structural validation, including date-time formats.
// Semantic requirements such as summary presence and alias ownership are separate.
func ValidateArticle(value any) error {
	schema, err := articleSchema()
	if err != nil {
		return fmt.Errorf("compile article schema: %w", err)
	}
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
