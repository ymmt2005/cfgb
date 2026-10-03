# CLI contract

The executable is `cfgb`. Global flags: `--config PATH` (default: nearest ancestor
`cfgb.yaml`, stopping at repository root), `--format text|json`, `--no-color`,
`--version`. Paths are relative to the configuration directory, never the current
working directory. Reject unknown keys, duplicate YAML keys, custom YAML tags,
out-of-root paths and symlink traversal. YAML is parsed as YAML 1.2 with timestamp
values preserved as strings. Accept UTF-8 without BOM. Hash normalization follows
the domain contract: [AI summary input](05-ai.md#input-and-output-hashes) normalizes
body newlines; [migration source/target hashes](06-migration.md#assets-and-restart-safety)
use exact content bytes as specified there. Do not normalize artifact output bytes.

## Commands

| Command | Inputs/defaults | Effects |
| --- | --- | --- |
| `new --lang ja --slug SLUG TITLE` | Locale required; explicit ASCII slug when title cannot be slugified | Require clean Git worktree, create `post/<article-key>` branch and variant skeleton |
| `translate ARTICLE --to en --slug SLUG` | Article key, not ambiguous title | Create missing variant in same group; copy topics/title as placeholders; empty body and summary; never overwrite or translate with AI |
| `prepare [ARTICLE...] [--offline] [--refresh-links] [--dry-run]` | All variants if no selection | Report assets; fetch missing link cards unless offline; no LLM or commits |
| `validate [--authoring\|--publish] [--now RFC3339]` | Regular validation by default | Read-only structural and semantic checks |
| `build [--out DIR]` | Default `dist`; regular validation, embedded fixed toolchain | Generate verified artifact; no upload |
| `deploy --from DIR` | Verified artifact and production target from environment | Check publication conditions and deploy the same artifact; no rebuild |
| `preview --from DIR` | Verified artifact and current non-production branch | Create/update a private Worker Preview; no rebuild |
| `summarize [ARTICLE...] [--changed-since REF] [--dry-run]` | Explicit selection or changed variants | Generate eligible summaries and sidecars; never automatically commit |
| `summarize ARTICLE --lang LOCALE --replace-manual` | One explicit variant | Deliberately replace a manual summary; flag never used in CI |
| `ai eval summary --corpus PATH --candidates PATH --out DIR` | Candidate config and review corpus | Generate anonymized review bundle; do not modify articles |
| `import hatena inventory --out DIR` | Blogs from config; authenticated online read | Fetch paginated inventory, source and rendered snapshots |
| `import hatena plan --input DIR --out DIR` | Captured source, category and pairing decisions | Produce complete URL/asset plan; no live writes |
| `import hatena apply --plan PATH [--dry-run]` | Reviewed plan, hash preconditions | Apply local changes atomically with manifest checkpoints |
| `hatena mark-moved --plan PATH --apply` | Separate reviewed remote edit plan | Explicit old-site update; never invoked by import |
| `migrate schema --to N [--dry-run]` | Supported schema migration | Require clean worktree, show diff; no silent migration |

`ARTICLE` is the POSIX path relative to `content.root/posts`, e.g.
`2026/2026-09-19-protobuf-guide`. Selection by article key includes every variant
unless `--lang` restricts it. Empty selection is a successful no-op, not “all”.
`summarize` requires selection or `--changed-since`; it does not surprise authors
by sending the entire repository. Deleted variants are omitted from generation
but still matter for link validation. Renames are detected by Git and reported.

`new` derives `<year>/<YYYY-MM-DD>-<slug>` in `site.timezone`, checks collisions,
and sets `publishedAt` to the creation instant with an offset. No future scheduling.
It prints the created branch and path, but makes no commit. The author must review
the date at publication. `translate` uses the current branch, sets a new locale
publication date, does not copy source aliases, ogImage or generated ownership,
and requires author review of title and summary. Translation tooling is v1
scaffolding only; `--ai` is reserved, not accepted.
Both commands write `summary: ""`, not a bare `summary:` key (parsed as null).
Empty content is scaffolding, not input for an AI summary; refuse generation from
an empty/whitespace-only body without writing a summary or ownership record.

## Validation modes

| Rule | `--authoring` | default | `--publish` |
| --- | --- | --- | --- |
| Invalid schema/topic/path/date/URL/asset/link | Error | Error | Error |
| Missing, empty or whitespace-only summary | Warning | Error | Error |
| Summary null, numeric or another non-string value | Error | Error | Error |
| Valid future `publishedAt` | Warning | Warning | Error |
| Summary outside recommended length | Warning | Warning | Warning |
| Unknown generated sidecar version | Error | Error | Error |
| Missing link-card cache | Warning | Warning | Warning |
| Missing alt/unused/duplicate/large image | Warning | Warning | Warning |

`--now` is a test clock, never read from untrusted frontmatter. Production CI
does not override the clock. `updatedAt >= publishedAt`; publication requires
both timestamps not later than now. Warnings do not become errors implicitly.
Default `validate` is the final PR content gate after summary generation.

The article Schema permits omitted/empty summary structurally in all modes; its
newline constraint still applies to present strings. After structure checks, a
summary is missing when absent or empty after trimming Unicode White_Space
characters. Emit `W_SUMMARY_REQUIRED` in authoring and `E_SUMMARY_REQUIRED`
otherwise, without a second length warning for that field. Non-string values
fail with `E_SCHEMA` in every mode. Other required fields are not relaxed.

## GitHub Action setup

`ymmt2005/cfgb-action` installs a verified, exact CFGB release and registers it on
PATH. Caller workflow `run` steps execute the CLI directly, retaining its argument,
diagnostic and exit-code contract. Setup inputs, outputs and release installation
are specified in the [Action contract](09-github-action.md).

## Build and delivery

CFGB owns the embedded renderer, Worker, lockfile and artifact integration checks.
Content repositories require no package.json, Astro files, Wrangler files or
custom executable build scripts. `build` extracts implementation files to a
disposable workspace, stages configured content and installs pinned dependencies;
rendering, Pagefind and artifact checks then run offline. No AI, metadata refresh,
remote image fetch or source mutation occurs. Node.js remains required in v1.
The [build runtime contract](10-build-runtime.md) defines bootstrap, runtime
requirements, retained toolchain sessions and source-provenance resolution. Build
checks the actual Node range and exact pnpm from release requirements before
installing frozen dependencies. Its installed Wrangler survives for the same
build's upload command, outside the content repository and deployable artifact.

`--out` must resolve inside the repository, outside input content and protected
Git/configuration paths. Refuse a nonempty directory unless recognized as a prior
CFGB artifact; never recursively delete an unowned directory. Stage a complete
replacement before publishing new output. Default build validation allows future
publication dates for previews but requires existing valid summaries.

Artifact layout: `site/` (static assets), `worker/index.js` (bundled Worker) and
`build-manifest.json` (source commit and branch, CFGB/renderer versions,
config/input/output hashes, publication metadata snapshot, completed checks, provenance provider,
optional opaque build identifier (`buildUUID`), toolchain session ID and required/observed runtime versions).
Publication metadata includes article key, locale, slug, summary and timestamps for each variant. Local
builds may record dirty worktrees; remote uploads require a clean checkout matching
the recorded commit and input hashes. Generated/ignored output does not count as
a source edit. Never include credentials or raw private source exports.

`deploy`/`preview` verify artifact bytes against the recorded hashes, completed
checks, supported versions and source identity before upload. These are
deployment-correctness checks: they detect stale, inconsistent, accidentally
modified or wrong-source artifacts. They are not a cryptographic authentication
boundary against a malicious artifact store, transfer channel or compromised
deployment environment. Production also validates the publication snapshot with
current time, including future-date rejection. No source mutation, regeneration
or re-rendering occurs. `deploy` requires both the recorded and current source
branch to equal `deploy.productionBranch` (default `main`); `preview` rejects
that branch. A wrong branch is `E_DEPLOY_TARGET`, exit 1. The recorded commit
must match the current checkout; no comparison with the production branch tip
is required. These checks supplement branch protection.

`CFGB_CF_WORKER_NAME` supplies the target Worker; `CLOUDFLARE_ACCOUNT_ID` and
`CLOUDFLARE_API_TOKEN` configure the Wrangler adapter. Generate Wrangler config
in a disposable upload workspace from the artifact and trusted target settings.
Its top-level `compatibility_date` comes from release-pinned
`workerCompatibilityDate`, recorded in the artifact; v1 always generates
`workers_dev: false`, `preview_urls: true`, `previews: {}` and top-level assets.
Preview/Version URLs require Access coverage; the production custom domain stays
public. See the runtime contract.
Use the retained toolchain session's Wrangler; a temporary upload config does not
reinstall or lose that toolchain. Workers Builds source commit/branch/build UUID
come from `WORKERS_CI_COMMIT_SHA`, `WORKERS_CI_BRANCH`, `WORKERS_CI_BUILD_UUID`
and must match the original checkout and artifact as defined in the runtime
contract. Read-only Access verification uses `CFGB_CF_ACCESS_API_TOKEN`.
Preserve the original branch when creating a Preview; the temporary workspace
must not change its identity. Use CFGB's pinned Wrangler dependency, never an
unpinned npx download. None of these credentials becomes a visitor-runtime secret.

Diagnostic codes: `E_ARTIFACT` for missing checks/corrupt or unsupported artifacts,
`E_BUILD_SOURCE` for stale/dirty source, `E_DEPLOY_TARGET` for branch/target errors,
and `E_PREVIEW_ACCESS` for absent/unverifiable Access coverage. `E_TOOLCHAIN`
(exit 2) identifies missing/incompatible Node/pnpm/Wrangler or toolchain sessions.
Publication content failures retain the same `E_*` codes as `validate --publish`. Missing build/upload prerequisites
are configuration failures (exit 2); remote upload failures use exit 3. Gate
violations use exit 1. Do not promote/upload a failed artifact.

## Errors and output

Exit codes: `0` success (including warnings), `1` validation failure, `2` invalid
usage/configuration, `3` IO/network/provider failure, `4` conflict/precondition
failure. JSON diagnostics use one object with `schemaVersion: 1`, `errors`,
`warnings` arrays and stable diagnostic objects `{code,path,line,message}`;
`line` may be null. Human messages may evolve; codes are the test contract.

Core codes: `E_SCHEMA`, `E_TOPIC`, `E_SLUG_DUPLICATE`, `E_DATE`, `E_FUTURE_DATE`,
`E_SUMMARY_REQUIRED`, `E_ASSET_MISSING`, `E_LOCAL_PATH`, `E_LINK_BROKEN`,
`E_TRANSLATION_GROUP`, `E_ALIAS_DUPLICATE`, `E_URL_COLLISION`,
`E_SIDECAR_VERSION`, `E_CONFLICT`, `E_PROVIDER_CONFIG`, `E_FETCH_UNSAFE`.
Warnings include `W_SUMMARY_LENGTH`, `W_SUMMARY_REQUIRED`, `W_FUTURE_DATE`,
`W_ALT_EMPTY`, `W_ASSET_UNUSED`, `W_ASSET_DUPLICATE`, `W_ASSET_LARGE`,
`W_LINKCARD_MISSING`. Collect independent errors; don't invent cascading results
from a file that cannot be parsed.

Alias uniqueness is semantic validation after structural Schema validation.
Duplicates within an alias array or across variants use `E_ALIAS_DUPLICATE`,
exit 1, in every validation mode; malformed alias types/paths use `E_SCHEMA`.

## Safe writes and network

Mutations use sibling temporary files + rename; multi-file operations stage all
outputs and journal completion. Validate pre-write hashes to detect concurrent
edits. On conflict, leave current files untouched and emit reviewable proposals
outside content. Preserve YAML comments/key order with node-based updates.
Do not silently rewrite the whole document. Never execute Markdown, raw HTML,
article instructions, shell interpolation, or repository scripts to generate text.

Link-card extraction operates on the Markdown AST: only a top-level paragraph
whose single meaningful child is an external HTTP(S) autolink/bare URL is a card.
Labeled links, code, blockquotes and list items remain ordinary content. Use the
exact trimmed parsed URL (without fragment) as cache key; no query stripping or
canonical-URL rekeying. Cache filename is lowercase SHA-256 hex of those UTF-8
bytes. An absent cache falls back to a link. Metadata is text and HTML-escaped.

Fetcher contract: HTTP(S), ports 80/443 only, no userinfo; max 5 redirects,
10 seconds total, 2 MiB decompressed response. Disable environment proxy
inheritance. Resolve all A/AAAA records, reject any non-public address (including
loopback, RFC1918, link-local, mapped IPv4, multicast/reserved and metadata
endpoints), pin the approved address for connection while preserving Host/SNI,
and repeat validation per redirect. Block HTTPS downgrade. No ambient cookies or
Authorization on link metadata fetches. Tests must cover DNS rebinding, compressed
bombs, mixed DNS answers, and redirect to private hosts. Respect timeouts and
rate limits; failed refresh preserves the previous valid cache.

Assets: warn above 10 MiB, compare SHA-256 for duplicates, and never remove or
recompress originals automatically. A file is unused only after checking both
locale variants, raw HTML references, and ogImage. Remote images are permitted
but reported; they are not fetched by production builds.
