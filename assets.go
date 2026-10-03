// Package cfgb embeds the specification assets shipped inside the executable.
package cfgb

import "embed"

// FS holds schemas, prompts, and the release toolchain requirements.
// Renderer and Worker sources join this set when cfgb build learns to extract
// them. Patterns are relative to the module root.
//
//go:embed schemas prompts toolchain-requirements.json
var FS embed.FS
