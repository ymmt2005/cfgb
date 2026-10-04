# Renderer design and review boundaries

CFGB embeds the renderer and supplies it with a staged content snapshot and a
normalized metadata index. Content repositories contain Markdown and assets.
The renderer does not parse front matter again or infer article identity from a
collection ID or an absolute filesystem path.

## Input boundary

Go isolates the front-matter block, decodes that single YAML document, and
validates the original mapping against the embedded `article.schema.json`,
including asserted date-time formats. Only then does it project string arrays
and pass the untouched Markdown body to Astro. The schema is the source of
structural constraints; Astro's collection shape supplies types to templates.
Semantic validation remains a separate CLI milestone.

The normalized index owns group identity, locale, source filename, and article
route. `content.ts` joins collection entries to that index. `content-urls.mjs`
uses the same index for source-to-route and source-to-media mappings. Home,
about, and aside have explicit media scopes.

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

`image-paths.mjs` isolates a necessary Astro/Vite import adaptation: some valid
asset filenames cannot be used directly as Vite import specifiers. Copy image
bytes to safe names under the build-owned workspace. Astro then supplies
processed URLs and dimensions. These temporary imports are outside the content
repository and outside the deployable artifact. No final HTML rewrite is needed.

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

## Tests and scope

Fast tests cover URL resolution and rejection, image staging, HTML attribute
edits, ID associations, locale mapping, and theme/search helpers. Parser tests
separate YAML fidelity from article structural validity. Schema regression
cases cover both rejected inputs and authoring/semantic allowances.

Astro build tests inspect generated DOM, image dimensions and emitted files,
article and asset destinations, heading targets, cached-card behavior, layout
and aside composition, translations, and literal examples. The URL matrix also
builds a real Pagefind index and checks canonical article membership and
searchable title/summary text. The pinned example corpus goes through the actual
Go CLI, embedded extraction, dependency installation, Astro, and Pagefind.

Supported scenarios are based on the authoring contract. Local asset request
suffixes and an artificial Windows path supplied to a Linux renderer are not
positive conformance cases. Native paths continue to use Node's platform-aware
`path` implementation.

This PR remains the renderer/build foundation. Full semantic validation,
deploy/preview upload, and complete social image metadata are documented in
`progress.md` as unfinished. They are not claims made by the current renderer.
