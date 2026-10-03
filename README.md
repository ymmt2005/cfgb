# CFGB — Cloudflare Git-based Blog

A Git-native publishing system for multilingual technical blogs on Cloudflare.
CFGB is an independent open-source project and is not affiliated with Cloudflare,
Inc.

**Status: design and specifications only. The CLI and site are not implemented.**

The Go authoring tool will manage Markdown, validation, migration and optional
authoring-time AI. Astro will render the site, Workers Static Assets will serve
it, and Pagefind will search it. Git branches/PRs hold drafts; reviewed `main`
content is published. Visitors never invoke an LLM.

## Implementation specifications

| Document | Scope |
| --- | --- |
| [Architecture](docs/spec/00-architecture.md) | Responsibilities, settled choices and implementation phases |
| [CLI](docs/spec/01-cli.md) | Commands, flags, validation modes, diagnostics and safe writes |
| [Configuration](docs/spec/02-configuration.md) | `cfgb.yaml`, defaults and schema versioning |
| [Content and rendering](docs/spec/03-content.md) | Frontmatter, identity, routes, Markdown, search and SEO |
| [Delivery](docs/spec/04-delivery.md) | GitHub PR preparation and Cloudflare production/private previews |
| [AI](docs/spec/05-ai.md) | Gateway adapter, hashes, manual-edit protection and evaluation |
| [Hatena migration](docs/spec/06-migration.md) | Inventory, pairing, links/assets, checkpoints and conflicts |
| [Acceptance](docs/spec/07-acceptance.md) | Traceable implementation gates and fixture semantics |
| [References](docs/spec/08-references.md) | Primary platform documentation reviewed for the design |

[JSON Schemas](schemas/) define machine-readable structural contracts. Semantic
validation rules in the specifications also apply. The [example repository](https://github.com/ymmt2005/cfgb-example)
contains synthetic Japanese/English content, original assets, cached link metadata,
and expected results for future tests. Fixture paths in the specifications refer
to that repository unless explicitly described as CFGB tool paths.

The personal reference site will use `https://ymmt2005.dev`. The reusable example
uses a reserved origin so its metadata cannot impersonate the real site. No
account resources, credentials, active deployment workflows or executable CLI
implementation are included at this stage.
