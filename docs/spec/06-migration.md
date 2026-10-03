# Hatena migration algorithm

Migration sources are the configured `hatena.blogs` entries and their locales.
Support one or multiple configured blogs; no personal origin is hard-coded.
Test exports in `cfgb-example` are synthetic
and use reserved `.invalid` origins. No live import or remote modification occurs
when checking this corpus.

All migration commands, including inventory, plan, apply and mark-moved, make no
model calls and neither require nor read AI credentials. Pairing candidates use
deterministic heuristics; approved human decisions are the only pairing authority.

## Inventory and extraction

Authenticate to Hatena AtomPub with credentials from `CFGB_HATENA_USER` and
`CFGB_HATENA_API_KEY`, kept outside Git. Resolve API collection URLs through the
authenticated service document/configuration and follow paginated `rel=next`
links, recording source IDs, edit/alternate URLs and content type. Rate-limit and
retry bounded transient failures; checkpoint each page. Disable XML external
entities/DTD resolution. Never leak credentials to arbitrary asset hosts.

Store raw Atom, original content and formatted HTML with SHA-256 checksums under
local ignored `.cfgb-work/hatena/`. `cfgb import hatena inventory` captures the
snapshot first; `cfgb import hatena plan` works offline from that snapshot.
Reject duplicate source IDs with different bodies, malformed timestamps and
pagination loops. Report source drafts/preview/paid-content flags and exclude
them from the publishable plan unless deliberately reviewed; do not accidentally
publish private material obtained through an authenticated export.

For `text/x-markdown`, preserve original Markdown and only perform specified AST
rewrites. For `text/html` or `text/x-hatena-syntax`, retain source and formatted HTML,
generate a proposed conversion, and require review. Inventory Hatena-specific
embeds, footnotes, heading anchors and scripts; never assert lossless conversion
without comparing rendered output. Raw snapshots are not committed by default.

## Planning before writes

1. Inventory all configured blogs completely and compute source-body hashes over exact UTF-8
   content bytes as returned by XML decoding (no newline normalization).
2. Propose topics from categories one-to-one, then review multilingual label
   merges. Store approved category-to-topic mappings; don't invent topics using AI.
3. Produce translation-pair candidates from explicit links, dates, titles, shared
   outbound links/assets using deterministic heuristics only. Confidence is advisory.
   Use stable source-ID ordering for ties; the same snapshot and decisions under
   the same CFGB version must produce the same candidate report. No LLM comparison.
4. Read explicit pair decisions. No automatic pairing, even for high confidence;
   reject a decision assigning one source to multiple groups or two same-locale
   sources to one group. Unapproved candidates remain separate groups.
5. Assign stable article keys and independent locale slugs. Freeze the entire
   old-URL to article/locale/new-URL map before rewriting any article.
6. Build an asset plan with source URL, ownership decision, SHA-256, MIME, byte
   count and local target. Download only operator-approved owned assets.
7. Build fragment mappings by comparing captured HTML anchors to the new rendered
   heading IDs. Preserve explicit IDs where feasible; unresolved/ambiguous mappings
   are blockers, never silently drop fragments.
8. Stage proposed content, expected routes and conflict reports. Preserve any
   explicitly supplied human summary; absent summaries remain absent/empty and
   require authoring-mode validation at this stage. Review before applying.
   Generate missing summaries later through the separate `cfgb summarize` command,
   then review and validate for publication. Migration never calls a provider.

URL matching resolves relative URLs against each original article URL, maps exact
known original HTTP/HTTPS aliases, and preserves query/fragment semantics. Do not
strip arbitrary query parameters or treat every same-host URL as an article.
Rewrite inline/reference Markdown links and raw HTML `a[href]`; image `src`,
`srcset`, and linked images need asset mapping. Never touch code examples or
unrelated external URLs. Removed/unmapped articles are reported for a decision.

## Assets and restart safety

“Owned” means uploaded/controlled by the author, not just hosted on a Hatena domain.
Review the host/ownership allowlist. Store original bytes; no lossy recompression.
Deduplicate by SHA-256 within a logical article group so translations can share
assets; do not deduplicate across groups in v1. Derive safe filenames from hashes
and sniffed types; reject path traversal. Third-party images remain remote and
are reported. Apply the fetcher's public-address/redirect/size protections with a
separate explicit large-asset limit (25 MiB); report oversized sources for review.

`migration/hatena/manifest.json` is committed provenance. Each entry records
`sourceId`, `sourceUrl`, `locale`, `sourceHash`, `articleKey`, `targetUrl`,
and `targetHash`. Only successfully applied entries are recorded; there is no
status/conflict entry. `sourceHash` is the source body used in the last successful
application; `targetHash` hashes the exact target Markdown written by that same
application, including frontmatter. Update the pair together only after success.
Never replace either hash with a newly observed source/target during a conflict.
Per-variant sidecars repeat import identity/hash,
not private raw source snapshots. Missing source entries never imply deletion.

Conflicts are written to a separate report under `.cfgb-work/hatena/`, conforming
to `migration-conflicts.schema.json`. New unowned target collisions create no
manifest entry. Conflicts on previously applied entries preserve their complete
last-success record and sidecar. Report `observedSourceHash`, optional
`observedTargetHash` (absent if the target is missing), optional
`proposedTargetHash`, and the prior `lastAppliedSourceHash`/`lastAppliedTargetHash`
pair when available. Omit both last-applied fields for unowned targets. These
observations/proposals never establish ownership or authorize replacement.
Record a reason and identity for each conflict. Partial progress can record
unrelated successful entries but must preserve every conflicting baseline.

| Rerun condition | Behavior |
| --- | --- |
| Source unchanged and target matches last target hash | Skip, byte-identical no-op |
| Source unchanged but target edited by human | Preserve target; report local edit |
| Source changed and target still matches last write | Produce replacement plan, apply after review |
| Source changed and target also edited | Conflict; preserve target and prior manifest/hash pair; emit separate report/proposal |
| New source but target path occupied without matching provenance | Conflict, never overwrite or create a manifest/ownership entry |
| Partial prior apply | Resume from journal; verify every existing target before continuing |

Perform changes in a clean worktree, stage/journal each transaction, then atomic
rename and manifest update. Keep enough preconditions to recover between a target
write and a manifest write. Recovery must verify journal preconditions before
recording a successful paired baseline; otherwise preserve the prior record and
report a conflict. A fresh manifest alone must never claim old target
ownership. Applying the same reviewed plan twice must not alter bytes or dates.

## Old-site notices and cutover

`cfgb hatena mark-moved` is separate and defaults to plan-only without `--apply`.
Its plan lists each remote edit and checks the fetched content hash/ETag where
supported immediately before writing. Back up originals; prepend one recognizable
notice without duplicate insertion, preserve original syntax/body, and refuse
concurrent remote changes. It must not delete articles or bulk redirect domains.

Keep old blogs as read-only archives initially. New-site alias files cannot
redirect a domain the site does not control. Review cross-domain duplicate-content
and canonical options separately. Before cutover: real Japanese search corpus,
rendered comparison, anchor/link/asset report, topic/pair decisions, feeds/sitemap,
404s, alias redirects, Access, security headers and canonical apex verified.
