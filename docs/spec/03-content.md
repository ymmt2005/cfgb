# Content, renderer and URL contract

## Identity and frontmatter

Discover only `content.root/posts/<YYYY>/<article-key>/<locale>.md`.
The pair `<YYYY>/<article-key>` is the logical article identity. Article keys
match `[a-z0-9]+(-[a-z0-9]+)*`; the initial `YYYY-MM-DD-` prefix is a creation
convention, not a source of publication dates. Do not rename published groups.
The filename supplies locale. An article group has one or both enabled locales
and shared `assets/`; `.cfgb.json` is optional. A stray `fr.md`, nested variant,
or duplicate locale is an error. Discovery excludes `tests`, docs and examples.

`build` isolates each article's front matter with a line reader and decodes only
that block with `goccy/go-yaml`. The Markdown body, including a later `---` or
fenced code, is unchanged. Home, about, and aside files have no front matter, so
their entire contents stay the body. `topics.yaml` uses the same decoder. The
renderer consumes those normalized records and does not parse YAML itself.

Frontmatter structure requires `title`, `slug`, `publishedAt`, `topics`.
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
then ascending article key as a deterministic tie break. `updatedAt` never
reorders publication feeds. Archive year/month and visible dates use site timezone.

An article's URL never depends on directory year, title, or date. Build a single
route registry before rendering. Aliases are origin-relative paths with a trailing
slash; reject queries/fragments, encoded separators, dot segments, backslashes,
`//`, external URLs and wildcard placeholders. V1 permits ASCII path segments
using letters/digits/hyphens/underscores. Reject alias loops, duplicate aliases,
canonical collisions, reserved routes and cross-locale aliases. Emit direct 301s
to current canonical routes (no chains). Hatena URLs are provenance, not aliases.
Alias array/item structure is checked by JSON Schema. Duplicate aliases within
one variant or across variants are semantic errors: `E_ALIAS_DUPLICATE`, exit 1,
in default, authoring and publish validation. Do not reject duplicates as
`E_SCHEMA`; structural alias type/path failures remain schema errors.
A schema-valid alias must begin with its owning variant's `/<locale>/` prefix.
A cross-locale alias is `E_URL_COLLISION`, exit 1, in every validation mode,
even when its path does not otherwise exist in the route registry.

Global language links target the same article's counterpart if available;
otherwise target the other locale home. Only real pairs receive an article-level
translation notice and reciprocal `hreflang`. Each pair member uses its own
canonical and its own summary/dates. Do not pretend untranslated content has an
alternate. Other translated page pairs have reciprocal locale links. Root may
use `x-default`; unpaired articles must not invent a language alternate.

## Markdown and images

Use GFM (tables, task lists, strikeout, autolinks) plus footnotes and GitHub alerts
NOTE/TIP/IMPORTANT/WARNING/CAUTION. Footnotes use `[^id]` references and
`[^id]: definition` lines, as provided by the pinned `remark-gfm` footnote
extension. Keep that extension enabled. Place the definitions at the end of the
article body, numbered by first reference. Each reference links to its
definition, and each definition links back to every reference that uses it. The
same id keeps one number. Render inline Markdown inside a definition. Show the
list without JavaScript. A reference with no definition is `E_LINK_BROKEN`.
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
Do not rewrite examples inside code fences. Markdown H1 is reserved for the
layout title; article body headings begin at H2, TOC includes H2/H3.

Raw HTML is allowed for trusted, reviewed authors. It does not imply arbitrary
third-party scripts are acceptable; inventory and review migration embeds. Reject
javascript URLs. No MDX/code execution. Imported inline event handlers, iframes
and scripts are surfaced as review blockers before publication. Any approved
embed must be consistent with the final CSP/privacy policy.

Original PNG/JPEG/SVG/etc. remain in Git. Astro processes local Markdown images
and supplies dimensions/responsive delivery. External images remain external
without build-time downloads and appear in migration/privacy reports. A Markdown
or raw HTML link or image to `./assets/...` is rewritten to the published
`/media/<year>/<article>/` path. Existing percent-escapes are not encoded again,
and any query or fragment stays on that URL. Local SVG
is used as an image, not blindly injected as trusted inline markup. The example
includes SVG as a precise diagram and PNG as an original lossless raster fixture.
OG fallback is a build-time PNG with title/branding and a bundled licensed font
supporting Japanese; browser web fonts are still unnecessary. OG references must
resolve to a crawler-compatible PNG/JPEG, rasterizing SVG source if necessary.
For local assets, including `ogImage`, normalize the path and check article-group
containment before testing file existence; then check resolved symlink containment.
An escaping path is `E_LOCAL_PATH`, exit 1, even if the target does not exist.

## Code, diagrams, theme and TOC

Expressive Code + Shiki handles syntax, `title="main.go"`, line/text marks, copy
buttons and accessible filename frames. One light/dark representation follows
page theme. Selectors are `[data-theme="light"]` and `[data-theme="dark"]`, so
system mode, which has no `data-theme` attribute, keeps the library's
`prefers-color-scheme` rules. Mermaid fences are extracted before code highlighting. Lazy-load a
local Mermaid bundle only when a page contains diagrams; use `securityLevel:
strict`. Keep the original source in that render target as the no-JS fallback;
hide it only after successful rendering by replacing the element with the diagram.
Do not emit a second copy of the source. Invalid Mermaid keeps source with a useful error, not blank content.
Re-render from original source on theme change, including a system color-scheme
change while the page remains in system mode; serialize renders to avoid races.

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
Use `data-pagefind-body` only on article content, with metadata for title, summary,
publication date and topics; filters use stable topic IDs and site-local year.
Pages that are not articles, including generated 404 pages, carry
`data-pagefind-ignore="all"`. The search UI selects the document language as the
`locale` filter. Navigation, the right-hand column, TOC, footer, and copy labels are excluded. Search title must remain searchable
even if metadata is set outside the body. URL results must be canonical locale
paths. The checked-in query corpus specifies top-k inclusion, not brittle ranking.

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
