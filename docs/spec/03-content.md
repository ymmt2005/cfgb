# Content, renderer and URL contract

## Identity and frontmatter

Discover only `content.root/posts/<YYYY>/<article-key>/<locale>.md`.
The pair `<YYYY>/<article-key>` is the logical article identity. An article key
is the literal directory name, without an additional character pattern. It is
independent of the public slug; spaces, Unicode and punctuation are preserved.
Encode each literal group component when constructing public media URLs.
The initial `YYYY-MM-DD-` prefix is a creation
convention, not a source of publication dates. Do not rename published groups.
The filename supplies locale. An article group has each enabled locale that was
authored, and shared `assets/`; `.cfgb.json` is optional. A stray `fr.md`, nested variant,
or duplicate locale is an error. Directories inside a group other than `assets/`
fail with `E_TRANSLATION_GROUP`, exit 1. Subdirectories
inside `assets/` remain shared assets, not article variants. Discovery excludes
`tests`, docs and examples.

`build` isolates each article's front matter with a line reader and decodes only
that block with `goccy/go-yaml`. The Markdown body, including a later `---` or
fenced code, is unchanged. Home, about, and aside files have no front matter, so
their entire contents stay the body. `topics.yaml` decodes into its typed map.
Article YAML unmarshals directly into the Go metadata struct; fields are strings,
string slices and Go time values. Consumers use those fields, not generic maps.
The YAML library supplies conversions and decoding errors; loaders add no AST,
custom-tag/document restrictions or mandatory JSON Schema validation. Article
body bytes are preserved. The renderer consumes serialized typed records and
does not parse YAML itself.

The following authoring/validation conventions describe the existing corpus and
optional standalone/editor schema. They are not YAML-loader prerequisites.
New runtime rejection rules require explicit human approval under `AGENTS.md`;
see the [PR #3 policy audit](../reviews/pr-3-policy-audit.md) for unverified rules.

The documented article structure includes `title`, `slug`, `publishedAt`, `topics`.
`summary` is structurally optional and may be empty during authoring. Default
validation and publication require a nonempty summary as a semantic rule.
Optionally `updatedAt`, `ogImage`, `aliases`. No other keys.
`slug` is lowercase ASCII letters/digits separated by single hyphens, unique
within locale. `publishedAt`/`updatedAt` are RFC3339 strings with explicit zone;
quote them in YAML. `topics` is a nonempty unique array. `summary` is plain text,
trimmed, without markup or newlines after YAML folding. Titles/summaries are
escaped in HTML, JSON-LD, XML and attributes. `ogImage` is a local `./assets/`
image; no remote OG dependency. All paths must remain within the article group.

```yaml
---
title: "Protocol Buffers のスキーマを読む"
slug: protobuf-schema-guide
publishedAt: "2026-09-19T13:12:40+09:00"
topics: [protobuf, oss]
summary: "A human-reviewed summary belongs here."
ogImage: ./assets/schema.svg
aliases: [/ja/posts/old-protobuf-guide/]
---
```

`home/<locale>.md` and `pages/about/<locale>.md` have no frontmatter in v1;
their title and routing come from localized layout labels. They are excluded
from article feeds/search. Their relative assets are scoped to their directory.

`aside/<locale>.md` is optional. It is ordinary Markdown with no frontmatter and
no prescribed structure. The renderer places its HTML in the right-hand column
of every page for that locale and does not give the file a URL. A missing file
omits the column. The column is excluded from feeds, search, the sitemap, and
route fixtures.

## URLs and ordering

Serve `/` as a runtime locale-negotiation route; do not generate an index page.
Generate static content for each enabled locale:
`/<locale>/`, `/posts/`, `/posts/<slug>/`, `/archive/`,
`/archive/<YYYY>/<MM>/`, `/topics/<topic>/`, `/search/`, `/about/`, `/feed.xml`
(all after the locale prefix). Generate `/sitemap-index.xml`, numbered
`/sitemap-<n>.xml` files beginning at zero, and `/robots.txt`.
Generate localized `/<locale>/404.html` and bilingual `/404.html` as internal
fallback assets, not indexable articles. Static Assets chooses the nearest
directory fallback with status 404. `/__locale` is a runtime preference route.
Fixtures separate `static-routes.json` (ordinary static content),
`worker-routes.json` (runtime behavior), `fallbacks.json` (404 assets/misses) and
`aliases.json` (redirects). Fallback assets and runtime preference endpoints are
excluded from canonical lists/search/feeds/sitemap. Root can be an x-default
alternate but is not an indexable generated page. Empty locale archives/home lists remain
valid. Monthly/topic pages exist only where that locale has matching articles.
Default lists are unpaginated in v1. Latest = descending publication instant,
then ascending article key and full year/group identity as deterministic tie
breaks. `updatedAt` never
reorders publication feeds. `site.timezone` is used only for monthly archive
classification: Go derives each article's archive year/month from its publication
instant and passes these fields separately from UTC publication/update timestamps.
Article lists, homes and article pages initially show the UTC date. JavaScript
uses the reader's browser timezone to update date text in the existing language
format; without JavaScript the UTC fallback remains. Generated `datetime`,
JSON-LD, Pagefind publication metadata and RSS retain UTC instants. Archive page
membership and links remain fixed even when a reader's visible date falls in a
different month. Directory names do not determine archive membership.

An article's URL never depends on directory year, title, or date. Build a single
route registry before rendering. Aliases are origin-relative paths with a trailing
slash; reject queries/fragments, encoded separators, dot segments, backslashes,
`//`, external URLs and wildcard placeholders. V1 permits ASCII path segments
using letters/digits/hyphens/underscores. Reject alias loops, duplicate aliases,
canonical collisions, reserved routes and cross-locale aliases. Emit direct 301s
to current canonical routes (no chains). Hatena URLs are provenance, not aliases.
Alias array/item structure is documented by the optional standalone JSON Schema;
the YAML loader decodes the Go slice directly. Duplicate aliases within
one variant or across variants are semantic errors: `E_ALIAS_DUPLICATE`, exit 1,
in default, authoring and publish validation. Do not reject duplicates as
`E_SCHEMA`; values that cannot decode into the Go slice remain decoder errors.
Optional Schema path checks are separate from YAML loading. The existing
semantic alias-ownership rule requires the owning variant's `/<locale>/` prefix.
A cross-locale alias is `E_URL_COLLISION`, exit 1, in every validation mode,
even when its path does not otherwise exist in the route registry.

Global language links use the article group's locale-to-URL map. The requested
locale resolves to that group's counterpart, or to that locale's home when the
group has no translation. A different group that publishes the same slug is not
a counterpart. Alternate-language metadata is the collection of actual group
members and is omitted when the group has only one. With one configured language,
the header shows a non-interactive label. With two, it shows a compact segmented
toggle with language links. With more, it shows a native disclosure listing every
configured language. Both controls mark the current language and work with keyboard
navigation and without JavaScript. JavaScript additionally closes the disclosure
on Escape, outside click or focus leaving.
Missing article translations are labeled as home destinations in the selector.
The article notice links every real translation by its configured language label;
it reports no translations only when no other group member exists.
Only a real counterpart receives the article-level translation
notice and reciprocal `hreflang`. Each member uses its own canonical and its
own summary/dates. Do not pretend untranslated content has an alternate. Home
and about use the same map over the configured locales. Root may use
`x-default`; unpaired articles must not invent a language alternate.

## Markdown and images

Generated GFM tables use the mockup's scrolling table wrapper. Generated footnote
references and backlinks use its reference/backlink classes while retaining the
Markdown processor's IDs and accessible labels. These presentation transforms
retain authored raw HTML structure.

Use GFM (tables, task lists, strikeout, autolinks) plus footnotes and GitHub alerts
NOTE/TIP/IMPORTANT/WARNING/CAUTION. Footnotes use `[^id]` references and
`[^id]: definition` lines, as provided by the pinned `remark-gfm` footnote
extension. Keep that extension enabled. Place the definitions at the end of the
article body, numbered by first reference. Each reference links to its
definition, and each definition links back to every reference that uses it. The
same id keeps one number. Render inline Markdown inside a definition. Show the
list without JavaScript. Aside is rendered on its own and then placed beside the
page. Prefix that fragment's generated ids, fragment links, footnote
backreferences, and accessibility references with `aside-` so they do not collide
with the page. Renamed authored ids also require their HTML `for`, `list`, `form`,
and `headers` references and ARIA id references to be updated. Rewrite only
targets inside the fragment; references outside it and links to other pages stay
unchanged. Article heading anchors stay unchanged. A reference with no definition is `E_LINK_BROKEN`.
Omit a definition that nothing references. Leave footnote syntax inside code
fences unchanged. Process Markdown via AST, including reference
links and raw HTML attributes. Parse those attributes with an HTML syntax tree,
including unquoted values, and do not rewrite text inside HTML comments. Replace
the attribute value in the original tag text. An opening tag stays an opening tag,
so the visible label remains inside the anchor when Markdown splits the tag from
its text. Local links to `../other-key/ja.md#heading` resolve
through the route registry; root-relative internal URLs must also resolve.
Fragment checks use the renderer's actual heading IDs, including duplicates and
non-Latin headings. Go validation uses a shared heading-manifest adapter or the
same algorithm, never an independent guess. Explicit HTML `id` values count too.
Do not rewrite examples inside code fences. Layout-owned targets use the
`cfgb-` prefix (for example `cfgb-content`) so ordinary authored headings such
as Content do not shadow the skip link. Markdown H1 is reserved for the layout
title; article body headings begin at H2, TOC includes H2/H3.

Raw HTML is allowed for trusted, reviewed authors. It does not imply arbitrary
third-party scripts are acceptable; inventory and review migration embeds. Reject
javascript URLs. No MDX/code execution. Imported inline event handlers, iframes
and scripts are surfaced as review blockers before publication. Any approved
embed must be consistent with the final CSP/privacy policy.

Original PNG/JPEG/SVG/etc. remain in Git. Astro processes local Markdown images,
including reference-style images, in articles and in home, about, and aside.
It supplies dimensions and emits the processed file. A definition used by a
Markdown image stays on that pipeline, even when a link uses the same definition.
Resolve image and link consumers separately, preserving the definition's title
and CommonMark identifier matching and first-definition precedence. Decode
local asset path components once; encoded filename characters such as `%26`,
`%23`, and `%3F` denote filename characters, not URL delimiters. A `./assets/...`
reference identifies a file and must not have a query or fragment. Reject such
suffixes consistently in Markdown images, asset links, and raw HTML attributes.
This restriction does not apply to published root-relative or remote URLs, or
relative article links such as `../other-key/ja.md#heading`. Relative article
links split their URL suffix before decoding and route lookup. External images
remain external without build-time downloads and appear in migration/privacy
reports. Raw HTML images and links to `./assets/...` are published under
`/media/<year>/<article>/`. Home, about, and aside use `/media/home/`,
`/media/about/`, and `/media/aside/`. Media scopes come from the normalized source
metadata, not guesses based on the file's absolute path. Existing percent-escapes
are not encoded again. Local SVG
is used as an image, not blindly injected as trusted inline markup. The example
includes SVG as a precise diagram and PNG as an original lossless raster fixture.
OG fallback is a build-time PNG with title/branding and a bundled licensed font
supporting Japanese; browser web fonts are still unnecessary. OG references must
resolve to a crawler-compatible PNG/JPEG, rasterizing SVG source if necessary.
The renderer emits article-specific PNGs under `/og/`, using `ogImage` when
provided and otherwise a 1200×630 title card. The fallback includes optional
`site.image` branding. Explicit images are decoded and converted to PNG, with
their oriented dimensions preserved. Each article/locale has a separate image
URL based on source identity. OpenGraph and Twitter metadata use the same
absolute production image URL, including any hosting prefix, actual dimensions
and article-title alternative text. BlogPosting JSON-LD includes that URL too.
These image assets are excluded from sitemap entries and article search.
For local assets, including `ogImage`, normalize the path and check article-group
containment before testing file existence; then check resolved symlink containment.
An escaping path is `E_LOCAL_PATH`, exit 1, even if the target does not exist.

## Code, diagrams, theme and TOC

Expressive Code + Shiki handles syntax, `title="main.go"`, line/text marks, copy
buttons, line numbers by default, and accessible filename frames. A fence may
opt out with `showLineNumbers=false`; copied text excludes the gutter. Frame
spacing and colors use the shared page palette tokens. One light/dark
representation follows page theme. Selectors are `[data-theme="light"]` and `[data-theme="dark"]`, so
system mode, which has no `data-theme` attribute, keeps the library's
`prefers-color-scheme` rules. Mermaid fences are extracted before code highlighting. Lazy-load a
local Mermaid bundle only when a page contains diagrams; use `securityLevel:
strict`. Keep the original source in that render target as the no-JS fallback;
hide it only after successful rendering by replacing the element with the diagram.
Keep Mermaid's default look and drop shadows. Use the page's resolved palette
colors and prose font for diagram labels, nodes and edges. Place diagrams in the mockup's bordered frame; wide flowcharts and
sequence diagrams scroll within it rather than shrinking their labels. Diagram
layout follows the authored Mermaid source, not the hand-drawn mockup geometry.
Do not emit a second copy of the source. Invalid Mermaid keeps source with a useful error, not blank content.
Re-render from original source on theme change, including a system color-scheme
change while the page remains in system mode; serialize renders to avoid races.

Successfully rendered diagrams offer a localized **Expand diagram** control.
Open a native modal filling the viewport, with zoom, fit-to-window, and actual-size
controls and a scrollable diagram area.
Open at no less than the diagram's natural size so large labels stay readable;
fit-to-window is an explicit overview that may shrink the diagram.
Where the browser permits it, a separate full-screen control hides browser chrome
through the Fullscreen API; otherwise
the expanded modal remains usable. Keep a single live render target and SVG,
preserving its IDs, original source, and theme redraws while expanded. Escape
and the close control dismiss the viewer, restore the diagram to its article,
and return keyboard focus to its expand control. Closing also exits full screen
when the viewer owns it. Native modal focus containment, mobile layout, reduced
motion, and labels in every supported language apply. Invalid diagrams and the
no-JavaScript source fallback do not show unusable expansion controls.

Theme states are system/light/dark. The color palette is specified only in
`cfgb.yaml`, not by a visitor control and not by a content file. Store explicit choice locally, handle storage
failure, listen for system changes only in system mode, and set the initial theme
before paint. TOC is an anchor list the reader can collapse and expand. An article
opens with the list expanded. Without JavaScript it remains that expanded
disclosure: a sticky column on a wide page and above the body on a narrow page.
IntersectionObserver highlights the current section. Reduced motion,
keyboard focus, skip link and WCAG AA contrast are required. Static list pages
may load the small global theme controller; no Mermaid/Pagefind there.

## Search, metadata and feeds

Run Pagefind Extended on built output. Each page has correct `<html lang>`;
load search only at `/<locale>/search/`, and reinitialize when locale changes.
Use `data-pagefind-body` on the article title, summary, and Markdown body, with
metadata for title, summary, publication date and topics; filters use stable topic IDs and the archive-timezone publication year.
The publication timestamp is the `datetime` attribute of its `time` element,
recorded as `published[datetime]`, not as a bracketed literal value.
Pages that are not articles, including generated 404 pages, carry
`data-pagefind-ignore="all"`. The search UI selects the document language as the
`locale` filter. Navigation, the right-hand column, TOC, footer, and copy labels are excluded. Search title must remain searchable
even if metadata is set outside the body. URL results must be canonical locale
paths. Aliases are emitted only in `_redirects`; static alias HTML is unnecessary
for delivery. The checked-in query corpus specifies top-k inclusion, not brittle
ranking.

Generate canonical, reciprocal alternates, OpenGraph, Twitter cards, BlogPosting
JSON-LD, sitemap and RSS. Feed descriptions reuse summaries; no full-body feed is
required in v1. RSS dates are correctly formatted instants; sitemap includes
canonical public pages only, excluding search/fallback/alias/preview URLs.
Use `updatedAt` when present for modification metadata. Escape `</script>` in
JSON-LD and validate XML. Preview robots deny crawling and emit noindex headers.

## Sitemap generation

Use the pinned `@astrojs/sitemap` integration embedded with CFGB's renderer;
CFGB does not implement its own XML writer. Set Astro `site` to configured
`site.baseUrl` even in preview builds. Use filename base `sitemap`, an explicit
`entryLimit: 45000` and no locale-specific chunks. `/sitemap-index.xml` references
every emitted `/sitemap-<n>.xml` by absolute production URL. The small example
corpus emits exactly `/sitemap-0.xml`; larger sites may emit further numbered
files. `/robots.txt` advertises the absolute sitemap-index URL in production;
preview retains its deny-crawling behavior.

Feed the integration the canonical HTML pages from the route registry and filter
out runtime routes (including `/`), search, 404 fallbacks, aliases, feeds, robots,
sitemap resources and asset URLs. Never include a preview hostname. The union
of numbered sitemap entries must equal the canonical HTML route set, with each
URL appearing once. XML order/formatting is not an acceptance contract.

Use the integration's `serialize` hook to supply article `lastmod` from `updatedAt`
when present, otherwise `publishedAt`; never use the build clock. Pages without
source modification metadata omit `lastmod`. Supply language links from actual
translation groups and confirmed translated page counterparts, including each
paired page itself. Home and about routes are generated for every configured
locale, so their alternates come from that locale list rather than from
optional prose files. Unpaired articles have no language alternates. Do not use
automatic pathname-based i18n matching: paired locale articles may have different
slugs. The integration still owns XML serialization and file splitting.

Acceptance parses the generated index and all referenced numbered files, verifies
that each file exists, checks canonical entries and article alternates against
`tests/expected/sitemap.json`, and rejects duplicate/missing/unexpected URLs.
Both the index and numbered files are included in `static-routes.json`.
