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
- Schemas and prompts are embedded. The renderer package manifest, its lockfiles, and the Worker source are the toolchain pins. `cfgb.yaml` is parsed with `github.com/goccy/go-yaml` `v1.19.2`. Unknown fields still fail.
- Renderer dependency pin is Astro `7.3.5`, `@astrojs/markdown-remark` `7.3.1`, `@astrojs/sitemap` `3.7.4`, `astro-expressive-code` `0.44.2`, Mermaid `12.1.0`, Pagefind `1.5.2`, Wrangler `4.147.0`. The default installer is npm >= 12, tested at `12.2.0`. `allowScripts` permits install scripts for esbuild and workerd. sharp `0.35.4` and `0.35.5` have no install script. pnpm >= 11 remains optional through `CFGB_PACKAGE_MANAGER=pnpm`, tested at `12.8.1`.
- `workerCompatibilityDate` is `2026-09-22`, pinned with the Worker source. It has not been deployed.
- The example commit above is the corpus pin. CI checks out that commit rather than the moving default branch.
- Node `24.21.0` (Krypton, Active LTS) is the tested runtime. `nodeRange` is `>=24.15.0 <25 || >=26.0.0`. Node 26 and newer are accepted; the example corpus also built on Node `26.10.0`. The Go job runs `go build ./...`, `go vet ./...`, `go test ./...`, and `go tool staticcheck ./...`, in that order, then checks `gofmt`. staticcheck `2026.2.1` (`v0.8.1`) is pinned with a `tool` directive in `go.mod`. It skips `TestExampleCorpus`. The TypeScript jobs run that corpus on Node `24.21.0` and Node `26.10.0`, with npm `12.2.0` and pnpm `12.8.1`. The `required` job is the status check to require: it is skipped when those jobs pass and fails when one of them fails or is cancelled. Node 25 is not, because npm 12 does not run there. Node 24 and Node 26 bundle npm 11.

## Phase 1 — renderer and build, started

`cfgb build` extracts the embedded Astro 7 renderer, installs dependencies with `npm ci`, renders the content repository, and runs Pagefind. `CFGB_PACKAGE_MANAGER=pnpm` uses the embedded pnpm lockfile instead. The artifact is `site/`, `worker/index.js`, and `build-manifest.json`. Article repositories still contain no framework files.

The renderer covers locale homes, articles, lists, archives, topics, about, search, feeds, sitemap, robots, alias `_redirects`, and localized 404 pages. Markdown includes GFM footnotes, GitHub alerts, Expressive Code, Mermaid, and cached link cards. Front matter and `topics.yaml` are parsed in Go with `goccy/go-yaml`. Only the isolated front-matter block is decoded, timestamps stay strings, and the Markdown body is passed through unchanged. The renderer reads that normalized metadata. The Worker negotiates `/` and `/__locale`, and the language links go through that endpoint so the `cfgb_locale` cookie is set. Visitors do not get a palette switch; theme is still system, light, or dark. `--out` is a disposable entry. The build removes that path at the start, without following a symlink, creates `<out>/.tmp`, and promotes `site/`, `worker/index.js`, and `build-manifest.json` after rendering. Each build creates a new `cfgb-build-*` workspace with `os.MkdirTemp` and records its basename as `toolchainSessionId`. The same build identifier still gets a new workspace. A failed build removes that workspace; a successful build retains it for upload. `buildUUID` is stored separately and is not a path. `cfgb.yaml` is read through the repository root. A `git status` failure is recorded as a dirty source. That flag, the publication snapshot, and any recorded source commit, branch, or build ID are the manifest's source and publication records. Upload accepts a dirty tree, a missing diagnostic, a difference from the current checkout, and site bytes edited after the build. The manifest carries no configuration, input, or output hashes. Production deploy still rejects future dates from the publication snapshot, and it still requires the current invocation's branch to be the configured production branch. Raw HTML attribute updates replace the value in the original tag, so an inline anchor keeps its label. The selected `--out` path is omitted from the source-dirty check. Article pages record the site-local year and publication timestamp for Pagefind. The search UI selects the document language as the locale filter. Pages that are not articles, and generated 404 pages, are marked ignored for Pagefind. Links and raw HTML images to `./assets/` are rewritten to the published media path. Home, about, and aside use `/media/home/`, `/media/about/`, and `/media/aside/`. Markdown images, including reference-style images in articles and prose, stay on Astro's image pipeline. Aside ids are prefixed so footnotes and headings do not collide with the page. Article heading anchors stay unchanged. The publication timestamp is Pagefind metadata from the time element's datetime. An untranslated article's language switch goes to the other locale home even when another group uses the same slug. That choice comes from the article group's locale-to-URL map. Configuration rejects an unsafe locale identifier, an empty locale map, a blank label, a default locale that is not configured, and a path-safe tag outside the shared Japanese and English catalog. The renderer does not treat an unknown locale as English. Existing percent-escapes are not encoded again, and any query or fragment stays on the URL. Social metadata is still incomplete. The layout emits OpenGraph title, description, and URL, and it emits neither `og:image` nor any `twitter:*` tag. A configured article `ogImage` is parsed and then ignored. Publishing that image, rasterizing an SVG to a crawler-compatible PNG or JPEG, and generating the title and branding fallback when no image is set remain Phase 1 work. The renderer metadata contract is not complete until those tags identify that asset. Full semantic validation, deploy, and preview upload are not implemented yet. CSP allows inline styles because Expressive Code and the footnote markup need them, `img-src` includes `https:` so remote article images still load, and `script-src` includes `'wasm-unsafe-eval'` for Pagefind. Mermaid redraws when the system color scheme changes while the page is in system mode. Observed Node, npm, and pnpm versions must be valid semantic versions; a prerelease does not pass a stable floor, including one whose numeric version is above that floor. `Accept-Language` excludes `q=0` regardless of parameter case. A Mermaid fence keeps one source element, which is both the no-JS fallback and the render target. Search initialization is an external same-origin script.

The Markdown URL regression suite now builds a matrix of image/link syntaxes,
shared reference definitions, URL suffixes, encoded filenames, and prose scopes.
Image imports are staged under the build workspace's `.astro/cfgb-images/` with
safe names, then handled by Astro. A final HTML pass restores authored suffixes
and rejects unresolved image markers. This workspace data is not a deployable
artifact. Relative article links decode URL path components before route lookup.
Aside namespacing preserves HTML and ARIA ID associations. Config loading rejects
unknown AI summary entry fields and structurally incomplete entries; article
normalization rejects unknown front-matter fields; discovery rejects unconfigured
article Markdown variants. These checks do not make full semantic validation
complete.

## Not done

Validation, deploy/preview upload, and every later milestone. Live Cloudflare, model, and release gates remain open.
