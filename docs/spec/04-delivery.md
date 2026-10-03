# GitHub and Cloudflare delivery contract

This is the intended deployment specification, not an active workflow. Neither
repository contains active CI/deployment implementation at this stage. No
Cloudflare resource or secret is created.

## Production

In the personal repository, pin Node/pnpm, Astro, Expressive Code, Pagefind Extended,
Mermaid and Wrangler to tested versions, and pin the CFGB binary release/checksum.
Commit the pnpm lockfile. The production branch is `main`:

```sh
pnpm install --frozen-lockfile
cfgb validate --publish
pnpm exec astro build
pnpm exec pagefind --site dist
pnpm run test:integration
pnpm exec wrangler deploy
```

`test:integration` is a required implementation script, not provided by this
content-only repository. Network content generation is forbidden after dependency
installation. Rendering must succeed with network denied, including remote image
handling and OG generation. Summaries and link cards must already be committed.
Cloudflare Workers Builds owns deployment; avoid a second competing deploy job
in GitHub Actions. Pin the same toolchain in both check and deploy environments.

The apex is canonical. Configure the `www` redirect at the host/zone layer while
preserving path/query. Disable or redirect the alternate production workers.dev
origin. Do not redirect previews to production. Emit artifact metadata with the
source commit; check it after deployment and retain rollback to the last good
Worker version. Runtime has no AI credentials or Hatena credentials.

## PR preparation and final checks

1. Read-only `pull_request` checks run with `contents: read`, no secrets, and
   credentials not persisted in checkout. Run `validate --authoring` first.
2. For trusted same-repository content branches, run the pinned CFGB binary to
   prepare missing metadata/summaries. Treat article text as data, and use trusted
   base-branch configuration/prompts in the credentialed job. Config/workflow/tool
   changes require review; never execute PR scripts in this job.
3. Re-read the PR head SHA before writing. Only allow generated changes to selected
   variants' `summary`, `.cfgb.json`, and link-card cache files. Commit only if
   changed and push normally; never force-push over author edits. Concurrent head
   changes cancel preparation and rerun it against the new head.
4. Use a dedicated GitHub App installation token for the bot push, scoped to the
   repository with contents write. This ensures follow-up checks can run on the
   generated head. Do not assume `GITHUB_TOKEN` pushes automatically rerun CI.
5. Final `content-validation` runs regular `cfgb validate` on that latest head,
   builds Astro/Pagefind, and executes route/render/search tests without secrets.
   A missing summary therefore fails the final gate until prepared by bot or human.
6. Workers Builds creates a Preview for the branch. Configure build validation
   before `pnpm exec wrangler preview`, and report the preview URL for the exact
   head. Older builds may fail while summary generation is pending; only the
   latest head's successful checks permit merge.

Use concurrency groups per PR and cancel stale runs. Preparation is idempotent;
unchanged bot output must end the loop. A provider failure leaves files unchanged
and reports retryable failure. Do not trigger generation solely on bot identity:
eligibility is determined from hashes and content diff.

Fork PRs get no secrets, no privileged preparation and no automatic credentialed
deployment. A missing summary must be supplied by the contributor or prepared on
a maintainer-owned reviewed branch. Never use `pull_request_target` to execute
fork code with secrets. Require trusted build approval for untrusted configuration
changes, including same-repo branches; membership alone is not a security boundary.

Branch protection requires `content-validation` and the actual observed Cloudflare
preview check context (do not guess its display name). Both must refer to the
latest head. Require review after generated diffs as appropriate. There is no
independent time scheduler to publish future dates.

## Private preview requirements

Cloudflare's current Worker Previews use `wrangler preview`; existing setups may
still use version URLs and need a deliberate migration. Choose the new Preview
model for a new site. Preview and production settings are separate.

Previews are public by default. `security.previewAccess: true` is an assertion
checked by deployment acceptance, not a magical YAML access-control switch.
Configure Cloudflare Access before the first draft preview. Cover the branch URL,
deployment-specific URL and any alternate workers.dev/custom host. Check that an
anonymous request cannot fetch HTML, images, feeds or Pagefind chunks through any
host. A service-token authenticated check must succeed. Fail preview readiness if
an alternate public hostname bypasses Access. Noindex is not authentication.
Keep production public and separate from the preview policy. Clean up closed PR
previews; do not rely on URL obscurity. Public Git still makes PR source readable.

## Request handling

Use Workers Static Assets with a minimal Worker, `ASSETS` binding and selective
Worker-first handling of `/` and `/__locale`. Asset misses invoke the Worker for
localized 404s. Do not use SPA fallback. Implementation must verify this routing
against its pinned Wrangler/compatibility date.

At `/`, choose supported locale by valid preference cookie, then parsed weighted
`Accept-Language` (primary language matching, q=0 excluded), then default. Return
302 to `/<locale>/` with `Cache-Control: private, no-store` and appropriate `Vary`.
No cookie is set merely by negotiation. The header selector uses a GET link to
`/__locale?lang=...&next=...`; accept only an enabled locale and a local route from
the route registry in that locale (otherwise its home). Set `cfgb_locale` with
Path=/, Secure, HttpOnly, SameSite=Lax, Max-Age=31536000, return 303/no-store.
This supports no-JS use and prevents open redirects. Locale URLs are never
redirected based on browser preference.

For a missing `/ja/...` or `/en/...`, fetch the matching prebuilt fallback HTML
internally and return it with status 404, not 200. Other missing paths return a
bilingual 404. Preserve HEAD semantics, and never serve article HTML for missing
assets. Fallback pages contain localized search and home links.

`_headers` sets nosniff, strict-origin-when-cross-origin, DENY framing and disabled
camera/microphone/geolocation for assets. The Worker must set the same applicable
headers on its own redirects/errors; `_headers` does not cover those responses.
`_redirects` implements validated article aliases. Security headers/CSP on Worker
responses and alias behavior need explicit integration tests.

Finalize CSP after enumerating Astro/Expressive Code/Mermaid inline scripts/styles
and migrated embeds. Prefer build-generated script hashes and local sources; do
not assume a strict policy works without browser testing. No external JS/font CDN,
tracking cookies or analytics by default. Analytics is an explicit future opt-in.
