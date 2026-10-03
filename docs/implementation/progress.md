# Implementation progress

Baseline reviewed before this work:

| Repository | Commit |
| --- | --- |
| cfgb | `66daf2f26a80ee42232976f1c332afb100ce221d` |
| cfgb-example | `7f9f3553f259e86e1417d24f625eb97bbaefa025` |
| cfgb-action | `e8150e2b5bc47388ef3dbecd7768687a4780500c` |

## M0-A — visual mockup

Browsable static prototype in `design/mockup/`. Search results are labeled mock data. The owner accepted this direction. The production renderer now follows it.

The header controls are dropdowns labeled only テーマ / Theme and 配色 / Palette. The open menu follows the page colors, including a dark menu in the dark theme. Palette choices are Classic, Cyber, Dope, Forest, and Dusk. Wide articles put the table of contents on the left. It opens expanded and can be collapsed; the article uses that width while the contents are closed. The right column renders optional `aside/<locale>.md` from the content repository and adds no structure of its own. The mockup palette menu is for appearance review only. In the finished site the palette is specified only in `cfgb.yaml`. The production visitor theme contract remains system, light, and dark. The palette field is not in the schema yet.

## M0-B — token handoff

`docs/operator/cloudflare-tokens.md` is a documentation-confirmed creation recipe. It is not live-tested. No credential has been used.

## M0-C — foundation started

- Go module `github.com/ymmt2005/cfgb`, toolchain Go 1.27.1. `cfgb version` works. Other commands exit 2.
- Schemas, prompts, and `toolchain-requirements.json` are embedded.
- Renderer dependency pin is Astro `6.4.8` (Astro 7.3.5 is published and is not the v1 baseline), `@astrojs/sitemap` `3.7.4`, `astro-expressive-code` `0.44.2`, Mermaid `12.1.0`, Pagefind `1.5.2`, Wrangler `4.147.0`. The default installer is npm >= 12, tested at `12.2.0`. `allowScripts` permits install scripts for esbuild, sharp, and workerd. pnpm `10.33.3` remains optional through `CFGB_PACKAGE_MANAGER=pnpm`.
- `workerCompatibilityDate` is `2026-09-22`. It has not been deployed.
- Corpus tests are not wired yet. The example commit above is the pin to use.
- Node `24.21.0` (Krypton, Active LTS) is the tested runtime. `nodeRange` is `>=24.15.0 <25 || >=26.0.0`. Node 26 and newer are accepted; the example corpus also built on Node `26.10.0`. CI runs that build on Node `24.21.0` and Node `26.10.0`. Node 25 is not, because npm 12 does not run there. Node 24 and Node 26 bundle npm 11.

## Phase 1 — renderer and build, started

`cfgb build` extracts the embedded Astro 6 renderer, installs dependencies with `npm ci`, renders the content repository, and runs Pagefind. `CFGB_PACKAGE_MANAGER=pnpm` uses the embedded pnpm lockfile instead. The artifact is `site/`, `worker/index.js`, and `build-manifest.json`. Article repositories still contain no framework files.

The renderer covers locale homes, articles, lists, archives, topics, about, search, feeds, sitemap, robots, alias `_redirects`, and localized 404 pages. Markdown includes GFM footnotes, GitHub alerts, Expressive Code, Mermaid, and cached link cards. The Worker negotiates `/` and `/__locale`. Visitors do not get a palette switch; theme is still system, light, or dark. Full semantic validation, deploy, and preview upload are not implemented yet. CSP still allows inline styles because Expressive Code and the footnote markup need them.

## Not done

Validation, deploy/preview upload, and every later milestone. Live Cloudflare, model, and release gates remain open.
