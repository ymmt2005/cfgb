# Acceptance and fixture guide

The documents describe existing design work, subject to the human-approval
priority in [AGENTS.md](../../AGENTS.md). A document or fixture is not approval
for a new input-rejection policy. The example corpus is acceptance data, not a
CLI or renderer implementation. Standalone Schema checks are optional and
separate from typed YAML loading and build acceptance.
`tests/fixtures/validation/schema-cases.json` in the example repository holds
the optional standalone scenarios; the CLI cases use decoded values. Do not
run the standalone scenarios as negative CLI acceptance.

| Gate | Evidence / fixture | Implementing component |
| --- | --- | --- |
| Japanese/English Markdown authoring | Positive articles, home, about | CLI + renderer |
| Single-language and translated groups | `tests/expected/articles.json` | Discovery + routing |
| Different locale slugs / shared images | Protobuf group | Renderer |
| Full Markdown, alerts, code marks, footnotes | Markdown showcase | Renderer visual/browser tests |
| TOC and no-JS fallback | H2/H3, duplicate headings, Mermaid | Browser tests |
| Paste/drop colocates originals | `.vscode/settings.json` | VS Code and Cursor manual check |
| Image warnings and local-path isolation | Negative cases + asset inventory | CLI |
| Cached link card / missing cache fallback | Protobuf URL and IANA URL | CLI + renderer |
| Monthly ordering and timezone boundary | Fixed archive-timezone year/month, UTC machine timestamps, reader-local visible dates and UTC no-JS fallback | Go + Renderer + Browser |
| Topic IDs and labels | `src/data/topics.yaml`; schema-valid missing/empty locale labels fail semantic `E_TOPIC` | CLI + renderer |
| Static, runtime, fallback routes and aliases | `static-routes.json`, `worker-routes.json`, `fallbacks.json`, aliases | CFGB renderer + Worker + Static Assets |
| Local links and fragment resolution | Cross-article/HTML/reference links | Shared renderer manifest + CLI |
| Search query behavior | `tests/search/queries.yaml` | Actual Pagefind browser search |
| Locale isolation and filters | Each query has locale; topic/year cases | Pagefind |
| SEO, RSS, OG and sitemap | Locale pairs + explicit/fallback OG; sitemap index, numbered files and canonical sets in `sitemap.json` | Renderer integration tests |
| AI lifecycle and human edit preservation | `tests/ai/lifecycle.json` | CLI with fake provider |
| AI quality and provider selection | `tests/ai/summary-corpus.yaml` | Live evaluation + human review |
| No runtime/build-time AI/content fetching | Network-denied build after installation | CFGB build |
| Framework-free content repositories | No Astro/Worker/package files added; embedded toolchain extraction | CFGB build |
| Pinned Workers Builds bootstrap | Runtime contract; release/digest verification in separate command shells | Build integration |
| Retained toolchain and runtime validation | Delivery cases; Node/npm/Wrangler, the optional pnpm path, a fresh workspace per build, and basename session IDs that are not derived from the build identifier | CFGB build/deploy/preview |
| Diagnostic CI metadata | Detached HEAD records an available CI branch; a missing or differing commit, branch, or build ID stays successful | CFGB build |
| Separate build/upload stages | `tests/build-delivery/cases.json`; supplied-artifact upload, publication timestamps, and the current-branch gate | CFGB build/deploy/preview |
| Summary validation modes | Mode checks on decoded strings; decoder failures; optional standalone raw-type Schema scenarios | CLI / optional Schema tooling |
| Future-date validation modes | Default/authoring `W_FUTURE_DATE`, exit 0; publish `E_FUTURE_DATE`, exit 1 | CLI |
| Cross-locale alias rejection | Schema-valid alias in another locale; `E_URL_COLLISION`, exit 1 | CLI |
| Semantic alias uniqueness | Duplicate arrays in every mode and duplicates across variants; `E_ALIAS_DUPLICATE` on decoded aliases | CLI |
| Configuration defaults and semantics | `tests/fixtures/configuration/cases.json`; typed loading/defaults followed by checks only in the command that uses each setting | Configuration loader + relevant commands |
| Setup Action and immutable-release verification | [Action documentation](https://github.com/ymmt2005/cfgb-action#readme), verified installation/toolchain metadata, package-manager selection and subsequent direct CLI execution | `cfgb-action` |
| PR generated diff / latest-head validation | Delivery race and retry scenarios | GitHub CI |
| Private preview on every exposed host | Pre-upload `preview_worker` identity/policy check plus anonymous denied / authorized successful | Cloudflare integration |
| Configured production branch | Default `main`, custom `master`, wrong-branch deploy/preview rejection; the current invocation's branch | CLI + Cloudflare Builds |
| Configured-blog import and syntax inventory | Synthetic Atom exports from configured blogs | Importer |
| Deterministic migration pairing without AI | Candidate repeatability, absent AI credentials, zero model calls and explicit decisions | Importer |
| Release-pinned Worker runtime | Build-delivery cases; pinned `compatibility_date`, `workers_dev: false`, `preview_urls: true`, `previews: {}` and top-level assets | CFGB release + upload adapter |
| Complete map before link rewriting | Migration expected map and forward link | Importer |
| Pairing approval and asset deduplication | Migration pairing/category/asset decisions | Importer |
| Rerun and conflicts | Hatena cases and separate expected conflict reports; unchanged last-applied hash pair | Importer |
| Independent tool/site versions | Config schema + pinned corpus commit | CLI/release management |

Upload tests check required files, runtime compatibility, publication timestamps,
the current branch, and Access. A dirty checkout, an edited site artifact, or a
difference in recorded commit, branch, or build ID stays successful.
Transferred-artifact tests recreate only the upload toolchain and leave the
supplied bytes unchanged. CFGB executable release verification and PR/credential
isolation are separate security concerns.

## Test execution levels

1. Corpus preparation: inspect routes, links, asset references, target hashes
   and summary lifecycle expectations with temporary tooling. Optional standalone
   Schema checks report their own results; they are not CLI loading prerequisites
   or implementation acceptance.
2. During CLI implementation: copy each mutation onto a fresh positive tree, run
   the real `cfgb validate` in the specified mode, and require the expected error or warning
   code and exit status. Keep fixtures out of normal content discovery. Do not compare full English
   diagnostics; additional independent diagnostics are permitted.
3. During CFGB renderer implementation: render all positives, inspect desktop/mobile in
   both themes and with JS disabled. Assert no unexpected remote requests. Check
   code-copy keyboard behavior and accessible diagrams/TOC. Validate XML and HTML
   metadata and all emitted internal URLs/anchors.
   Parse the sitemap index and every referenced numbered file; compare canonical
   URL sets and real article alternates with the expected sitemap fixture, and
   check source-based lastmod values rather than the build clock.
4. Search: serve the actual built output, open `/<locale>/search/`, run each query
   through Pagefind and materialize result data. Assert expected canonical URLs
   appear within topK, unexpected locale URLs do not appear, and filters work.
   Token presence in source is not a search test. Add real-article Japanese
   queries after Hatena import; do not replace them with synthetic easy matches.
5. Delivery/import: execute integration tests in disposable branches/environments
   with controlled HTTP/fake-provider responses. Never use actual old-site writes
   to test idempotence.

Action acceptance covers mutable releases, absent/invalid attestations, wrong
release/asset identity, byte mismatch and corrupt-cache/version failures,
platform selection without caller digests, safe extraction,
PATH registration and setup without a checkout. A smoke workflow installs CFGB
and invokes it in a subsequent `run` step. Domain diagnostics and exit-code tests
remain CLI tests; setup adds no framework files to article repositories. These
are future integration gates, not performed checks at the documentation stage.

Required extra failure tests in the tool: DNS rebinding/mixed A+AAAA/redirect SSRF,
YAML duplicate keys, symlink escapes, network timeout/429/retry budgets, invalid AI
responses, stale PR heads, failures between paired file writes, public preview
alternate hostname, Atom pagination loops, duplicate IDs, XML external entities,
unsupported Hatena syntax and ambiguous heading fragments. Cross-locale alias
rejection is already locked by a validation mutation.

For acceptance, all original v1 outcomes remain required: Markdown-only authoring,
image paste, rich rendering/code/TOC, private PR previews, publication from the
configured production branch,
Japanese and locale-specific search, reciprocal translations, automatic lists/
archives/topics/feed/SEO, reviewed AI summaries, deterministic production content,
Hatena migration and manual-edit protection, and reusable independent tooling.
