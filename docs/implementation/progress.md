# Implementation progress

Go error handling is checked by the pinned `errcheck` tool in CI, including
test code, explicit blank-identifier discards and fmt writes. Files/root handles
report close failures; file copying preserves delayed destination-close errors;
failed-build and output-staging cleanup return errors alongside the original
failure. CLI progress, result, help/version and diagnostic stream failures produce
a failure exit code. Configuration discovery distinguishes absence from probe
errors instead of silently selecting an ancestor after an I/O failure. Optional
Git diagnostics retain their documented fallback. Regression tests cover failing
writers/closers, simultaneous work/cleanup failures and broken discovery entries.
The root `AGENTS.md` records these rules and the settled design/review practices.

Baseline reviewed before this work:

| Repository   | Commit                                     |
| ------------ | ------------------------------------------ |
| cfgb         | `66daf2f26a80ee42232976f1c332afb100ce221d` |
| cfgb-example | `7f9f3553f259e86e1417d24f625eb97bbaefa025` |
| cfgb-action  | `e8150e2b5bc47388ef3dbecd7768687a4780500c` |

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

The command adapter uses Cobra `v1.10.2` (pflag `v1.0.9`) for command dispatch,
persistent flags, argument checks and help. This supports the specification's
future nested `import hatena` and `ai eval summary` commands without a separate
handwritten dispatcher. Only `version` and `build` are currently registered;
`--format` and `--no-color` remain later diagnostic-output work. `--config PATH`
works before/after `build`, including `--config=PATH`, and selects that exact
file. Its relative selector is resolved from the invocation directory, while
content and `--out` are resolved from the selected file's directory. Default
ancestor discovery and repository-rooted YAML decoding remain unchanged.
Help and version do not load configuration or probe the build toolchain. Parser
errors exit 2, domain errors retain their existing exit codes, and each invocation
creates a fresh command tree. Tests cover those parsing/selection boundaries,
in-repository and escaping config symlinks, and preservation of existing output
on an invalid selected config. Supported catalog/configuration/selection/cookie
language IDs remain case-sensitive; unknown regional tags are not implicitly
supported. Browser `Accept-Language` negotiation is unchanged.

`cfgb build` extracts the embedded Astro 7 renderer, installs dependencies with `npm ci`, renders the content repository, and runs Pagefind. `CFGB_PACKAGE_MANAGER=pnpm` uses the embedded pnpm lockfile instead. The artifact is `site/`, `worker/index.js`, and `build-manifest.json`. Article repositories still contain no framework files.

The renderer covers locale homes, articles, lists, archives, topics, about, search, feeds, sitemap, robots, alias `_redirects`, and localized 404 pages. Markdown includes GFM footnotes, GitHub alerts, Expressive Code, Mermaid, and cached link cards. Front matter and `topics.yaml` are parsed in Go with `goccy/go-yaml`. Only the isolated front-matter block is decoded into typed Go fields, time values serialize as RFC3339, and the Markdown body is passed through unchanged. The renderer reads that normalized metadata. The Worker negotiates `/` and `/__locale`, and the language links go through that endpoint so the `cfgb_locale` cookie is set. Visitors do not get a palette switch; theme is still system, light, or dark. `--out` is a disposable entry. The build removes that path at the start, without following a symlink, creates `<out>/.tmp`, and promotes `site/`, `worker/index.js`, and `build-manifest.json` after rendering. Each build creates a new `cfgb-build-*` workspace with `os.MkdirTemp` and records its basename as `toolchainSessionId`. The same build identifier still gets a new workspace. A failed build removes that workspace; a successful build retains it for upload. `buildUUID` is stored separately and is not a path. `cfgb.yaml` is read through the repository root. A `git status` failure is recorded as a dirty source. That flag, the publication snapshot, and any recorded source commit, branch, or build ID are the manifest's source and publication records. Upload accepts a dirty tree, a missing diagnostic, a difference from the current checkout, and site bytes edited after the build. The manifest carries no configuration, input, or output hashes. Production deploy still rejects future dates from the publication snapshot, and it still requires the current invocation's branch to be the configured production branch. Raw HTML attribute updates replace the value in the original tag, so an inline anchor keeps its label. The selected `--out` path is omitted from the source-dirty check. Article pages record the site-local year and publication timestamp for Pagefind. The search UI selects the document language as the locale filter. Pages that are not articles, and generated 404 pages, are marked ignored for Pagefind. Links and raw HTML images to `./assets/` are rewritten to the published media path. Home, about, and aside use `/media/home/`, `/media/about/`, and `/media/aside/`. Markdown images, including reference-style images in articles and prose, stay on Astro's image pipeline. Aside ids are prefixed so footnotes and headings do not collide with the page. Article heading anchors stay unchanged. The publication timestamp is Pagefind metadata from the time element's datetime. An untranslated article's language switch goes to the other locale home even when another group uses the same slug. That choice comes from the article group's locale-to-URL map. Build checks decoded settings for the release language catalog and timezone through ValidateSite; loading itself performs no such checks. The renderer does not treat an unknown locale as English. Existing percent-escapes are not encoded again. Local asset references identify files and reject query/fragment suffixes; relative article links preserve heading fragments. Social metadata is still incomplete. The layout emits OpenGraph title, description, and URL, and it emits neither `og:image` nor any `twitter:*` tag. A configured article `ogImage` is parsed and then ignored. Publishing that image, rasterizing an SVG to a crawler-compatible PNG or JPEG, and generating the title and branding fallback when no image is set remain Phase 1 work. The renderer metadata contract is not complete until those tags identify that asset. Full semantic validation, deploy, and preview upload are not implemented yet. CSP allows inline styles because Expressive Code and the footnote markup need them, `img-src` includes `https:` so remote article images still load, and `script-src` includes `'wasm-unsafe-eval'` for Pagefind. Mermaid redraws when the system color scheme changes while the page is in system mode. Observed Node, npm, and pnpm versions must be valid semantic versions; a prerelease does not pass a stable floor, including one whose numeric version is above that floor. `Accept-Language` excludes `q=0` regardless of parameter case. A Mermaid fence keeps one source element, which is both the no-JS fallback and the render target. Search initialization is an external same-origin script.

The Markdown pipeline expands reference consumers first, normalizes URLs next,
and performs block transforms last. Reference-form and inline links therefore
use the same cached-card behavior. Public media scopes and article identities
come from the Go metadata index. HTML URL and ID updates share one source-span
attribute editor, preserving split opening tags, quote styles, HTML entities,
and namespaced attributes. Image imports are staged under the build workspace's
`.astro/cfgb-images/` with safe names, then handled entirely by Astro; there is no
post-build image URL rewrite. Local asset query/fragment suffixes are rejected.
The regression matrix builds actual HTML, checks file and fragment targets,
and indexes the output with Pagefind. It covers image/link syntaxes, shared
reference definitions, encoded filenames, prose scopes, cached cards, and alias
exclusion. Title and summary are searchable even for image-only article bodies.
Layout-owned IDs use a `cfgb-` prefix; aside IDs use `aside-` and keep their HTML
and ARIA associations. See [renderer design](renderer.md).

Configuration loads directly into the complete Go struct with goccy/go-yaml's
unknown-field rejection. Defaults are seeded before decoding so explicit values
are preserved; Hatena blog entries are typed. There is no configuration AST
policy, custom-tag/document restriction or runtime JSON Schema gate. The
configuration schema remains available as an optional standalone/editor aid.
Language support is exact, case-sensitive catalog membership without a separate
language-tag syntax gate. Build calls the separate `ValidateSite` method for its supported-language and
bundled-timezone requirements; decoding itself does not perform these checks.
Decode and build-setting errors precede toolchain probes and output removal.

Article front matter now unmarshals directly into `frontmatter.Metadata`, with
string/slice/time fields and unknown-field rejection. Go consumers and the
publication snapshot use typed fields rather than input maps/type assertions.
Topics decode into their typed map. The article schema is optional standalone/
editor guidance; loaders do not apply it or add YAML AST/document/tag policies.
The user subsequently requested encoding checks: malformed UTF-8 is rejected
before YAML decoding or JSON serialization, and a file-start BOM is removed.
Interior U+FEFF characters and line endings remain unchanged. The Astro collection
shape provides types without extra nonempty/minimum-length constraints.
Discovery still uses four-digit years and checks disabled variants and group directories;
other unverified input policies are inventoried in the
[PR #3 policy audit](../reviews/pr-3-policy-audit.md). Their presence in an
AI-written specification/test is not evidence of human approval.

Article keys now use literal filesystem names without an ASCII pattern. Media
URLs and Astro's stored importer paths encode the original components, including
spaces, Unicode, hash signs and percent signs. `--out` may be outside the
repository while existing input-overlap checks and exact symlink-entry removal
remain. Both copy paths detect actual ancestor directory cycles with
`os.SameFile` rather than a depth limit, allowing deep finite trees and independent
aliases. The image adapter delegates format support to Astro without a separate
extension allowlist. The example integration includes a copied corpus with an
arbitrary article key, external output and 80-level media, alongside cycle and
failure-cleanup tests. The renderer unit/integration suite contains 33 tests.

Decoder/content errors retain typed diagnostic codes and exit 1 from `build`;
reader/filesystem failures remain exit 3. Tests cover direct decoding, body and
reader preservation, metadata serialization and command failure cleanup.
The theme listbox is named by its trigger. Theme values use an explicit set,
and both TOCs use a heading-ID map shared by desktop/mobile link elements.
Browser interaction tests build real pages and run Chromium with the generated
CSP in the Node 24/npm CI job; their separate test dependencies are not part of
the embedded renderer toolchain.

The browser suites now run 16 tests, including real Pagefind WASM searches with
a locale differing from the browser language, article/heading navigation and
empty-result recovery; real Mermaid redraws on native color-scheme changes and
rapid theme selections; malformed-diagram source/error preservation; actual
multiline Unicode clipboard content; skip-link/TOC/footnote navigation, native
desktop TOC reflow, mobile JavaScript-disabled navigation and reduced motion.
The fixtures share build/server setup and enforce the generated headers.
These tests exposed two UI defects: the theme popup stayed open after Tab moved
focus outside it, and native fragments could land behind the sticky masthead.
Focus departure now dismisses the popup without trapping focus, and desktop
fragment scroll padding clears the masthead. Static-masthead mobile pages omit
that padding. Runtime/resource failures and CSP violations fail the new cases.

Responsive raw HTML images now rewrite every `srcset` / `imagesrcset` URL through
the same media contract as `src`: local request suffixes and path escapes are
rejected, remote/data candidates are preserved, and density/width descriptors
are unchanged. Source-span tokenization follows HTML splitting rules rather than
splitting on every comma. Built-HTML tests cover articles and all prose scopes;
the two additional browser tests verify native density/width candidate selection,
data URLs, successful image decoding and picture breakpoint changes at DPR 1/2.

Static `_headers` and all Worker-created responses use the same baseline policy
from `renderer/src/lib/security-headers.json`. Worker tests cover redirects,
GET/HEAD/method/locale errors and the missing-binding fallback while preserving
cache, cookie, Vary, Location and Allow headers; asset responses pass through.
Astro content and image caches live under the workspace's `.astro/cache`. Vite
dependency optimization uses `.astro/vite` in that same workspace, so builds
that share a dependency installation keep separate caches.

A mandatory `links` CI job builds the pinned example and checks all generated
HTML with lychee 0.24.2, installed by aqua 2.63.0. Both registry/tool versions
and their checksums are committed; the installer Action uses a full commit SHA.
Routing adapters cover self-origin canonical URLs and alias redirects, while
Worker endpoints remain under the Worker tests. External links are an optional
online pass, reported by cfgb-example's scheduled/manual workflow.

## Not done

Validation, deploy/preview upload, and every later milestone. Live Cloudflare, model, and release gates remain open.

Configuration discovery/read failures are distinguished from decoder/usage
errors: real I/O (including a deleted working directory and temporary workspace
creation) exits 3, while invalid configuration exits 2. Public article and prose
media are copied from the staged content snapshot, rather than rereading the live
repository. An empty timezone uses Go's native UTC interpretation and is sent to
Intl as `UTC`. Equal publication times sort by article key, then the complete
year/group identity; generated homes and feeds exercise that ordering. Aside ID
references include microdata and native popover/dialog targets, with a browser
regression test that runs without JavaScript.

Parallel renderer CI exposed a separate Vite cache race when test workspaces
shared `node_modules`: Astro's cache setting alone did not isolate Vite's
`node_modules/.vite` writes. Vite now has its own workspace-local
`.astro/vite/` cache. Both actual Astro integration tests verify that cache is
created in their own workspace; parallel tests remain enabled.


Go now converts publication/update timestamps to the site location before passing
metadata to Astro. Archive dates and visible labels use the RFC3339 calendar
fields without JavaScript interpreting the site timezone. Tests cover Tokyo
month rollover, New York year/DST boundaries, fractional offsets, empty-name UTC
and a Go-only timezone, including actual HTML/RSS generation. CI triggers only
on pull requests and pushes to `main`.
