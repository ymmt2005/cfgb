# CLI contract

The executable is `cfgb`. Global flags: `--config PATH` (default: nearest ancestor
`cfgb.yaml`, stopping at repository root), `--format text|json`, `--no-color`,
`--version`. Paths are relative to the configuration directory, never the current
working directory. Reject unknown keys, duplicate YAML keys, custom YAML tags,
out-of-root paths and symlink escapes. YAML is parsed as YAML 1.2 with timestamp
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
| `build [--out DIR]` | Default `dist`; regular validation, embedded fixed toolchain | Generate the site artifact; no upload |
| `deploy --from DIR` | Supplied artifact and the current invocation's production target | Check publication conditions and deploy that artifact; no rebuild |
| `preview --from DIR` | Supplied artifact and the current invocation's non-production branch | Create/update a private Worker Preview; no rebuild |
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
custom executable build scripts. `build` creates a fresh workspace with
`os.MkdirTemp("", "cfgb-build-*")`, extracts implementation files there, stages
configured content, parses article front matter and
`topics.yaml` with `goccy/go-yaml`, and installs pinned dependencies. The
manifest records `filepath.Base` of that directory as `toolchainSessionId`.
`buildUUID` stays separate diagnostic metadata and does not name the workspace. A failed
build removes the workspace it created. A successful build retains it until
upload completion, upload failure, or disposal of the build environment. Deploy
and preview, which are not implemented here, resolve it as
`filepath.Join(os.TempDir(), toolchainSessionId)` when they share that temp
directory. Rendering, Pagefind and artifact checks then run offline. No AI, metadata refresh,
remote image fetch or source mutation occurs. Node.js remains required in v1.
The [build runtime contract](10-build-runtime.md) defines bootstrap, runtime
requirements, retained toolchain sessions and diagnostic source metadata. Build
checks the actual Node range and requires npm >= 12 before
installing frozen dependencies. `CFGB_PACKAGE_MANAGER=pnpm` selects optional pnpm >= 11
instead. That choice is an environment variable, not a field in
the content repository. Its installed Wrangler survives for the same
build's upload command, outside the content repository and deployable artifact.

`--out` is a path inside the repository. The check is lexical: refuse a
location that contains, or sits inside, source content, `cfgb.yaml`, topic and
link-card inputs, or `.git`. That includes an output directory that is an
ancestor of one of those inputs. Do not accept or reject the path by resolving
its symlink target, and do not delete that target. An existing directory, file,
symlink, or dangling symlink at the selected path is removed as that entry.
`build` does this removal at the start and stops if it fails, then creates
`<out>/.tmp` before rendering. It does not keep an earlier artifact. The
complete `site/`, `worker/index.js`, and `build-manifest.json` are written
there, then moved into `<out>`. Success is reported only after that complete
artifact is in place. A failed build removes the incomplete directory. This
invocation deletes only the staging directory it created; a sibling `<out>.tmp`
is left alone. A missing link-card directory is an empty cache. Repository-relative inputs are read through `os.Root` on the
repository, including `cfgb.yaml`. Article assets are read through a child root of that article's
`assets` directory. Symlinks and `..` that stay inside the applicable root are
followed. Escapes fail at the rooted open, which keeps the build inside its
inputs. Default build validation allows future
publication dates for previews but requires existing valid summaries.

Artifact layout: `site/` (static assets), `worker/index.js` (bundled Worker) and
`build-manifest.json`. The manifest records CFGB and renderer versions, the
toolchain workspace basename, required and observed runtime versions, completed
checks, and the publication snapshot. That snapshot lists each variant's article
key, locale, slug, summary, and timestamps, so production deploy can apply the
current-time future-date rule. Source commit, source branch, the opaque build
identifier (`buildUUID`), provenance provider, and a dirty-worktree flag are
optional diagnostics. Build records each value the checkout or CI environment
supplies, including a Workers Builds branch on a detached checkout. A missing
value, a dirty worktree, or a difference between a diagnostic and the current
checkout still produces a successful build. Generated output stays out of the
dirty record. The selected `--out` directory is omitted as a literal path,
including when its name contains a Git pathspec metacharacter and when it is
unignored and already present before the build. A real article or
configuration edit is still recorded as dirty. A `git status` query that fails is recorded as
dirty while a commit and branch that were already read stay in the manifest.
The manifest carries no configuration, input, or output hashes.
Never include credentials or raw private source exports.

`deploy` and `preview` check required files, artifact-format compatibility,
completed checks, and the runtime versions needed to run the supported Wrangler.
They upload the supplied artifact, including bytes an operator edited after the
build. CFGB does not rebuild the site and does not change those bytes.
Production also validates the publication snapshot against the current time,
including future-date rejection. `deploy` requires the current invocation's
branch to equal `deploy.productionBranch` (default `main`); `preview` rejects
that branch. A wrong branch is `E_DEPLOY_TARGET`, exit 1. The guard uses this
invocation's branch. A diagnostic commit, branch, or build identifier in the
artifact may differ from the current checkout and from the production branch
tip. These checks supplement branch protection. CFGB treats CI artifact storage,
transfer, and the deployment environment as operator-trusted. Verification of
the CFGB executable remains the release and setup-Action contract.

`CFGB_CF_WORKER_NAME` supplies the target Worker; `CLOUDFLARE_ACCOUNT_ID` and
`CLOUDFLARE_API_TOKEN` configure the Wrangler adapter. Generate Wrangler config
in a disposable upload workspace from the artifact and trusted target settings.
Its top-level `compatibility_date` comes from release-pinned
`workerCompatibilityDate`, recorded in the artifact; v1 always generates
`workers_dev: false`, `preview_urls: true`, `previews: {}` and top-level assets.
Preview/Version URLs require Access coverage; the production custom domain stays
public. See the runtime contract.
Use the retained toolchain session's Wrangler; a temporary upload config does not
reinstall or lose that toolchain. When `WORKERS_CI_COMMIT_SHA`,
`WORKERS_CI_BRANCH`, or `WORKERS_CI_BUILD_UUID` is available, record it as
diagnostic metadata, including a branch name on a detached checkout. A missing
or differing value leaves the build and the upload successful. Read-only Access
verification uses `CFGB_CF_ACCESS_API_TOKEN`.
Preserve the original branch when creating a Preview; the temporary workspace
must not change its identity. Use CFGB's pinned Wrangler dependency, never an
unpinned npx download. None of these credentials becomes a visitor-runtime secret.

Diagnostic codes: `E_ARTIFACT` for a missing required file, a corrupt artifact, or
an unsupported runtime requirement such as `workerCompatibilityDate`,
`E_DEPLOY_TARGET` for branch/target errors,
and `E_PREVIEW_ACCESS` for absent/unverifiable Access coverage. `E_TOOLCHAIN`
(exit 2) identifies missing/incompatible Node, npm, optional pnpm, Wrangler or toolchain sessions.
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
