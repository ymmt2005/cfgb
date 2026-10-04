// Package yamlutil checks YAML syntax contracts before value conversion.
package yamlutil

import (
	"fmt"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"
)

// ValidateTags rejects custom/unsupported tags before NodeToValue can erase
// them. Keep the decoder's built-in tag vocabulary; a directive must not
// redefine that vocabulary and pass as a reserved tag merely by its spelling.
func ValidateTags(body ast.Node) error {
	for _, node := range ast.Filter(ast.TagType, body) {
		tag := node.(*ast.TagNode)
		tk := tag.GetToken()
		_, reserved := token.ReservedTagKeywordMap[token.ReservedTagKeyword(tk.Value)]
		if !reserved || tag.Directive != nil {
			return fmt.Errorf("custom or unsupported YAML tag %q at line %d, column %d", tk.Value, tk.Position.Line, tk.Position.Column)
		}
	}
	return nil
}
