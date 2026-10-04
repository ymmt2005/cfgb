# CFGB — Git-based Blog on Cloudflare

A Git-native publishing system for multilingual technical blogs on Cloudflare.
CFGB is an independent open-source project and is not affiliated with Cloudflare,
Inc.

**Status: design, visual mockup, and a `cfgb build` that renders a content repository with the embedded Astro renderer. Deploy and preview upload are not implemented.**

The [visual mockup](design/mockup/README.md) is a browsable HTML prototype of the example blog. It is the appearance to review before the Astro renderer is built. Search results there are labeled mock data.

The Go tool will manage Markdown, validation, migration, optional authoring-time
AI, and build/delivery. CFGB owns the renderer and Worker implementation and embeds
their sources/configuration/lockfile in its executable. Article repositories hold
content, assets and blog settings only. Node.js remains a build prerequisite.
Astro will render internally, Workers Static Assets will serve, and Pagefind will
search. Git branches/PRs hold drafts; reviewed content on `deploy.productionBranch`
(default `main`) is published.
Visitors never invoke an LLM.

The [cfgb-action repository](https://github.com/ymmt2005/cfgb-action) owns the
setup Action: install a verified CFGB release and register it on PATH. Workflows
run CLI commands directly after setup. Action and CLI versions are independently
pinned. The Action selects the runner asset and verifies the immutable release
and GitHub release attestation. It is also at the documentation-only stage.

## Implementation specifications

| Document | Scope |
| --- | --- |
| [Architecture](docs/spec/00-architecture.md) | Responsibilities, settled choices and implementation phases |
| [CLI](docs/spec/01-cli.md) | Authoring/build/delivery commands, validation modes, diagnostics and safe writes |
| [Configuration](docs/spec/02-configuration.md) | `cfgb.yaml`, defaults and schema versioning |
| [Content and rendering](docs/spec/03-content.md) | Frontmatter, identity, routes, Markdown, search and SEO |
| [Delivery](docs/spec/04-delivery.md) | GitHub PR preparation and Cloudflare production/private previews |
| [AI](docs/spec/05-ai.md) | Gateway adapter, hashes, manual-edit protection and evaluation |
| [Hatena migration](docs/spec/06-migration.md) | Inventory, pairing, links/assets, checkpoints and conflicts |
| [Acceptance](docs/spec/07-acceptance.md) | Traceable implementation gates and fixture semantics |
| [References](docs/spec/08-references.md) | Primary platform documentation reviewed for the design |
| [GitHub Action](docs/spec/09-github-action.md) | CLI setup Action, inputs/outputs, versions and installation contract |
| [Build runtime](docs/spec/10-build-runtime.md) | Workers Builds bootstrap, Node/npm/Wrangler lifecycle, and diagnostic build metadata |

[JSON Schemas](schemas/) are optional standalone/editor validation aids; YAML
loading uses typed Go decoding, without a Schema gate. Existing specification
rules are subject to the human-approval priority in [AGENTS.md](AGENTS.md).
See the [input-policy audit](docs/reviews/pr-3-policy-audit.md) for restrictions
whose explicit approval could not be verified. The [example repository](https://github.com/ymmt2005/cfgb-example)
contains synthetic Japanese/English content, original assets, cached link metadata,
and expected results for future tests. Fixture paths in the specifications refer
to that repository unless explicitly described as CFGB tool paths.

The personal reference site will use `https://ymmt2005.dev`. The reusable example
uses a reserved origin so its metadata cannot impersonate the real site. No
account resources, credentials or active deployment workflows are included.
The executable currently implements `version` and `build`; later commands are
documented design work.

## License

This project, including its documentation, schemas and prompt specifications,
is licensed under the [Apache License, Version 2.0](LICENSE).
