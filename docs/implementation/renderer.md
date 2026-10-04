# Renderer design and review boundaries

CFGB embeds the renderer and supplies it with a staged content snapshot and a
normalized metadata index. Public media is copied from that same staged content
root, so later edits/deletions in the live repository cannot change one renderer
input independently. Content repositories contain Markdown and assets.
The renderer does not parse front matter again or infer article identity from a
collection ID or an absolute filesystem path.

Astro's content/image cache and Vite's dependency cache both live under the
invocation's `.astro/` directory. Vite has its own `cacheDir`; setting only
Astro's cache does not isolate it. Test workspaces may share installed dependencies
through symlinks, but they must not write a shared `node_modules/.vite` cache.
Actual renderer builds verify workspace-local Vite cache creation while running
in parallel.

## Input boundary

Go isolates the front-matter block with a line reader and unmarshals it directly
into `frontmatter.Metadata`. Its fields are strings, string slices and Go time
values; all Go metadata consumers use those typed fields. Topics decode directly
into a typed topic/language/label map. The Markdown body goes to Astro after UTF-8 validation.
There is no YAML AST/map projection, document/tag policy or mandatory JSON Schema
validation in either loader. The optional article schema is a standalone/editor
aid, not the runtime definition of what a YAML loader may accept. The Astro shape
supplies collection types without adding nonempty/minimum-length gates.

The opening/closing `---` lines identify the front-matter block, including a BOM
before the opening delimiter. The YAML library handles decoding, conversions
and syntax errors. A shared text-input boundary rejects malformed UTF-8 before
YAML decoding or JSON serialization. A file-start UTF-8 BOM is removed; interior
U+FEFF characters and line endings remain. This explicitly requested encoding
check applies to configuration, topics, article front matter/body and all prose. Content failures carry a typed diagnostic (`E_SCHEMA`,
`E_SLUG_DUPLICATE`, or `E_TRANSLATION_GROUP`) and make `build` exit 1;
reader/filesystem errors remain I/O failures with exit 3. Other unverified restrictions are listed in the
[PR #3 policy audit](../reviews/pr-3-policy-audit.md); they are not evidence of
human approval. New correctness rules require explicit human confirmation under
`AGENTS.md`.

Go derives each post's `archive: {year, month}` with `site.timezone` and normalizes
publication/update timestamps to UTC. This keeps historical offsets containing
seconds out of machine timestamp serialization. Archive routes use those derived
fields, independently of visible dates. Static HTML renders a UTC fallback in
`time[data-cfgb-date]`; the shared browser/server date formatter then updates only
its text in the reader's timezone. Machine `datetime`, JSON-LD, Pagefind metadata
and RSS retain UTC instants. JavaScript never receives the site timezone.

The normalized index owns group identity, locale, source filename, and article
route. `content.ts` joins collection entries to that index. `content-urls.mjs`
uses the same index for source-to-route and source-to-media mappings. Home,
about, and aside have explicit media scopes.
Article keys are literal filesystem directory names, not public slugs. Media
URLs and Astro's stored importer paths encode each component so spaces, Unicode,
`#` and `%` retain their filename meaning. The source index and logical IDs keep
the original names.

## Markdown transformations

`remark-cfgb.mjs` has three ordered stages:

1. Expand image and link references into independent consumers. CommonMark
   identifier matching and first-definition precedence apply. Preserve titles,
   children, author-supplied node data, and source positions.
2. Normalize consumer URLs. Article links resolve through source metadata.
   Asset links and raw HTML use public media URLs. Markdown images use safe
   workspace imports and Astro's image pipeline. Definitions themselves do not
   need URL rewriting once their consumers have been expanded.
3. Transform blocks: alerts, Mermaid fences, and cached cards. These transforms
   see already-resolved children, including reference-form links. Traversal
   follows replacement nodes, never detached subtrees.

Local `./assets/...` references are file paths and reject query/fragment
suffixes. Decode filename components once, check containment, and encode the
public path once. Percent-encoded filename punctuation is supported. Heading
fragments on relative article links, and remote or public URLs, retain their
normal URL meaning. Unused definitions do not cause image imports.

Raw HTML `srcset` and preload `imagesrcset` use the same media URL policy for
every URL token as `src`, including local suffix/path rejection. A small URL-span
tokenizer follows the HTML srcset splitting rules, preserving descriptors,
spacing and comma-containing URLs (including data URLs). Descriptor validation
remains with the browser; the rewrite does not add or repair descriptors.
The existing HTML attribute editor handles entity decoding/re-escaping and
quote styles. Article, home, about and aside scopes share source metadata.
Tests inspect the actual built candidate URLs/files and Chromium's selected
images at different device pixel ratios and picture media-query breakpoints.

`image-paths.mjs` isolates a necessary Astro/Vite import adaptation: some valid
asset filenames cannot be used directly as Vite import specifiers. Copy image
bytes to safe names under the build-owned workspace. Astro then supplies
processed URLs and dimensions. These temporary imports are outside the content
repository and outside the deployable artifact. No final HTML rewrite is needed.
This adapter does not maintain an image-extension allowlist. Format support and
format errors belong to the installed Astro image pipeline; removing the CFGB
list does not promise additional formats beyond that pipeline.

## HTML and page assembly

`html-attributes.mjs` parses attributes with parse5 and edits only their source
value spans. It is shared by URL rewriting and aside ID namespacing. Avoid
serializing raw Markdown HTML fragments: an opening tag may be represented
separately from its visible text and closing tag. Preserve comments and literal
examples, quote styles, entities, namespaced attributes, and template contents.

Aside is rendered independently and its IDs and internal ID references receive
`aside-`. Article anchors remain stable. Layout-owned controls use `cfgb-` IDs
so ordinary heading IDs do not shadow the skip link or theme controls. Authored
HTML is trusted; this transformation is not a sanitizer. Full URL and ID
validation is still part of the later semantic validation milestone.

Astro's sitemap integration owns sitemap XML. The renderer supplies canonical
route membership and translation groups. Alias redirects exist only in
`_redirects`; no alias HTML is generated. Pagefind indexes title, summary, and
Markdown content on canonical articles. Navigation, TOC, aside, non-article
pages, and fallback pages are excluded. Publication metadata comes from the
`time` element's `datetime`, not a literal in Pagefind's attribute syntax.

## Browser behavior

Persisted theme choices are members of an explicit `Set`; inherited JavaScript
object properties cannot become theme values. The theme listbox is named by its
button and supports focus movement, selection, and Escape back to the button.
Storage failures do not prevent a visitor from changing the current page theme.
Pointer clicks outside the menu and keyboard focus leaving it dismiss the popup
without moving focus back to its trigger. Native fragment navigation has scroll
padding below the sticky masthead; mobile pages with a static masthead omit it.

The scroll observer groups desktop and mobile TOC links by heading ID in a
`Map`. It updates the stored elements directly instead of constructing a CSS
selector from authored anchors. Ordinary heading IDs such as `constructor` and
`__proto__` do not collide with JavaScript object prototypes.

## Tests and scope

Fast tests cover URL resolution and rejection, image staging, HTML attribute
edits, ID associations, locale mapping, and theme/search helpers. Parser tests exercise the actual Go struct decoder, tagged/scalar/list values,
reader errors and byte-preserved bodies. Optional schemas are separate from
loading; tests must not turn their constraints into implicit runtime rules.

Astro build tests inspect generated DOM, image dimensions and emitted files,
article and asset destinations, heading targets, cached-card behavior, layout
and aside composition, translations, and literal examples. The URL matrix also
builds a real Pagefind index and checks canonical article membership and
searchable title/summary text. The pinned example corpus goes through the actual
Go CLI, embedded extraction, dependency installation, Astro, and Pagefind.

`renderer/tests/browser/` builds a small actual site and serves its generated
HTML and scripts with the generated security headers, then builds the real
Pagefind index. Playwright covers theme, storage and scroll-observer behavior,
pre-interaction theme paint, real Mermaid SVG rendering and native system-theme
changes, invalid-diagram recovery, native
Unicode clipboard copy, Japanese/English WASM search and result/fragment
navigation, empty-result recovery, skip-link keyboard navigation, native TOC
collapse/reflow, footnote round trips and menu focus departure. Desktop and
mobile viewports, reduced motion and JavaScript-disabled article navigation are
covered. Native popover and dialog command targets in authored aside HTML are
also exercised with JavaScript disabled; `itemref`, `popovertarget` and
`commandfor` follow namespaced IDs. Tests await observable state rather than fixed sleeps, and new cases
fail on runtime/resource errors or CSP violations. Browser APIs and libraries
are not replaced with test doubles. CI runs this Chromium suite on the Node
24/npm job; a mobile viewport is not a Safari/WebKit compatibility claim. The
browser-test package has its own pinned npm
lockfile and is not embedded or installed by `cfgb build`.

Run the browser suite after installing renderer dependencies:

```sh
cd renderer/tests/browser
npm ci
npx --no-install playwright install --with-deps chromium
node --test *.test.mjs
```

Supported scenarios are based on the authoring contract. Local asset request
suffixes and an artificial Windows path supplied to a Linux renderer are not
positive conformance cases. Native paths continue to use Node's platform-aware
`path` implementation.

This PR remains the renderer/build foundation. Full semantic validation,
deploy/preview upload, and complete social image metadata are documented in
the remaining delivery/CLI specifications as unfinished. They are not claims made by the current renderer.

## Generated-site link checks

CI builds the pinned example with the CLI and runs lychee over every generated
HTML file. `aqua.yaml` pins the registry and lychee versions; committed
`aqua-checksums.json` locks their downloaded bytes on supported platforms.
Checksum enforcement is enabled in the workflow, and aqua-installer is pinned
to a full commit SHA. The link check is a dependency of the `required` status.

`scripts/check-links.mjs` supplies CFGB routing to lychee. It maps same-origin
absolute URLs to the local output, resolves aliases through generated
`_redirects`, and preserves fragments at their canonical targets. Directory
links require an actual `index.html`. Only the two Worker endpoints `/` and
`/__locale` are excluded from filesystem checks; Worker tests cover them.
The routing test uses the real pinned lychee and verifies that missing pages,
images, fragment IDs, index pages, and alias targets are rejected.

After building the corpus, run:

```sh
aqua install
node scripts/check-links.mjs path/to/dist/site https://example.invalid --config .github/lychee.toml
```

Add `--online --output external-links.md` to also check external HTTP links.
This enables lychee's cache; the config limits concurrency and supplies retry
and timeout settings. External availability is reported separately from the
required offline renderer check. `cfgb-example` owns that scheduled/manual
check against its current content. Its negative input fixtures are not scanned.

To update the tool, review the exact version/ref changes and run
`aqua update-checksum -prune`. Commit the config and generated checksums together;
normal CI must not regenerate the lock.
