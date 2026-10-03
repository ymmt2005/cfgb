// Package cfgb embeds the specification assets shipped inside the executable.
package cfgb

import "embed"

// FS holds schemas, prompts, and the release toolchain requirements.
// Renderer and Worker sources join this set when cfgb build learns to extract
// them. Patterns are relative to the module root.
//
//go:embed schemas prompts toolchain-requirements.json
//go:embed all:renderer/src
//go:embed all:renderer/public
//go:embed renderer/package.json renderer/package-lock.json renderer/pnpm-lock.yaml renderer/astro.config.mjs renderer/tsconfig.json
var FS embed.FS
