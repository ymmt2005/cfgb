# GitHub and Cloudflare delivery

This is the v1 implementation contract, not an implemented deployment workflow.
Git is the source of truth; AI runs only during authoring. The content repository
contains Markdown, data, assets and CFGB configuration. CFGB owns and embeds the
Astro renderer, Worker, dependency lockfile and tool configuration. Builds extract
these into a disposable workspace; they never scaffold framework files into the
content repository. Node.js is a documented build prerequisite.

## Workers Builds command boundaries

Install a reviewed, pinned CFGB release in the trusted build environment. CFGB
pins its renderer dependencies and Wrangler in its embedded lockfile; Worker
Previews require Wrangler **4.135.0 or newer**. Verify platform compatibility when
implementation starts and when upgrading the pinned toolchain.

| Workers Builds setting | Command | Responsibility |
| --- | --- | --- |
| Build command | `cfgb build --out dist` | Validate, render with Astro, build Pagefind, run integration checks, seal artifact |
| Deploy command | `cfgb deploy --from dist` | Verify artifact and publication gate; upload production Worker/assets |
| Preview command | `cfgb preview --from dist` | Verify artifact and private-preview gate; upload branch preview |

The build command never uploads or invokes a deployment command. Deploy and
preview consume the same verified build artifact without rebuilding. Neither
command generates summaries, fetches link cards, or imports Hatena content.
After dependency installation, rendering and indexing must work with network
access denied. Assets, content and link metadata all come from Git.

The artifact contains `site/`, `worker/index.js` and `build-manifest.json`. The
manifest records source commit and branch, CFGB/schema/toolchain versions,
configuration and input/output hashes, successful build checks and publication
metadata. Uploads require a clean source checkout matching the artifact; ignored
build output is not a source edit. Reject stale, incomplete or altered artifacts.

Build uses default validation, which permits future publication timestamps; it
must still reject missing summaries. Production deploy additionally checks
publication timestamps from the verified snapshot against the current time and
requires `main`. A future-dated change can have a private preview while its final
`validate --publish` merge check fails. Preview requires a non-production branch.
Use the trusted environment variables defined in the CLI specification for the
Cloudflare account, Worker target and upload token. Do not commit credentials or
account-specific deployment configuration. CFGB generates temporary Wrangler
configuration for the reviewed deployment target; authors do not run Astro,
Pagefind or Wrangler directly.

The configured production origin is canonical. For an apex deployment, redirect
`www` at the host/zone layer while preserving path/query, and disable or redirect
the alternate production workers.dev origin. Do not redirect previews to
production. Check deployed source-commit metadata and retain platform rollback
to the last good Worker version. Runtime has no AI or Hatena credentials.

## Trust boundary and PR lifecycle

Direct pushes to any same-repository branch are restricted to trusted maintainers
and narrowly scoped automation acting for them. This is a security boundary:
Workers Builds executes the pushed branch before any later PR approval. Review
configuration, workflow and tool-version changes before pushing them to an
automatically built branch. PR branch protection alone is not a deployment gate.

External contributors use fork PRs. Fork jobs are read-only, receive no privileged
credentials and create no automatic preview. A maintainer can review and transfer
a change to a trusted same-repository branch before previewing it. If untrusted
same-repository push access is needed later, change this trust model first and
use a gated deployment pipeline, such as GitHub Actions with a protected
Environment; do not assume the automatic Workers Builds setup enforces that gate.

1. Run read-only `validate --authoring` checks on the PR head with `contents: read`,
   no secrets and no persisted checkout credentials. Non-string summaries
   remain schema errors; missing/empty summaries are warnings in this mode.
2. For trusted authoring automation, use a reviewed pinned CFGB release and
   trusted configuration. Generate only eligible summaries; protect human edits.
   Use trusted base-branch configuration/prompts, never PR scripts. Restrict writes
   to selected variants' summaries, sidecars and link-card caches. Re-read the head
   before writing, cancel on concurrent changes, and push normally without force.
   Commit the generated diff using a scoped GitHub App token. Do not assume
   `GITHUB_TOKEN`-generated activity automatically reruns checks.
3. Guard against stale heads and bot loops. A generated commit changes the head,
   so final required checks must run on that new commit. Never validate one head
   and upload artifacts from another.
4. Final required checks include default validation, `validate --publish` and
   build integration checks. Main publication uses the exact successful main
   artifact. Non-main Workers Builds uses the Preview command and Access gate.

Workers Builds owns the timing of deployment, using the separate commands above.
GitHub checks protect merging; they do not silently gate every branch build.
Require `content-validation` and the actually observed Cloudflare preview check
context on the latest head; do not guess its display name. Use per-PR concurrency
and cancel stale jobs. Provider failures preserve files; unchanged bot output ends
the loop. Require review of generated diffs as appropriate. No independent time
scheduler publishes future timestamps. Never execute fork code with secrets via
`pull_request_target`.
Cloudflare upload credentials and AI credentials belong to separate, least-scope
trusted jobs. Content files and PR-controlled configuration cannot choose tokens,
provider endpoints or deployment targets outside the trusted allowlist.

## Mandatory private preview

`security.previewAccess` is `true` in v1, including when omitted. `false` is a
configuration error, not an option for a public preview. The setting asserts a
requirement; it does not automatically provision Cloudflare Access policies.

Before uploading, verify that the reviewed Access configuration covers every
preview hostname the operation can expose, including branch, deployment and
alternate Worker preview URLs. An Access policy on the production domain alone
is insufficient. Missing coverage or inability to verify it fails the preview
command before upload. Do not print an unprotected URL as a ready preview.

After upload, check that anonymous requests are denied and authorized requests
succeed for HTML, images, feeds and Pagefind assets on every exposed hostname.
Report readiness only after these checks pass. If post-upload verification fails,
attempt to disable/remove the new preview where supported, return failure and
report cleanup failure separately. The pre-upload gate remains mandatory.
Production is public; preview visibility never changes canonical URLs or makes
content committed to public Git private. Clean up previews for closed PRs;
noindex and URL obscurity are not access control. Credentials and private drafts must not
be committed to a public content repository.

## Locale Worker and Static Assets

CFGB's embedded Worker handles only locale negotiation. Generated Wrangler
configuration uses selective `run_worker_first` for `/` and `/__locale`, binds
`ASSETS`, sets `html_handling = "auto-trailing-slash"` and
`not_found_handling = "404-page"`. All other requests use Static Assets routing;
if a miss reaches Worker code it delegates to `ASSETS.fetch(request)`.

- `/` is a runtime route, with no generated root `index.html`. It returns a **302**
  redirect to `/<locale>/`: valid locale cookie first, then supported
  `Accept-Language` ranges by descending quality (exclude `q=0`), then configured
  default locale. Preserve deterministic tie handling. Emit `Cache-Control:
  private, no-store` and `Vary: Cookie, Accept-Language` so negotiation is not
  shared-cached. No cookie is set merely by negotiation. Locale URLs are never
  redirected according to browser preference.
- `GET /__locale?lang=ja|en&next=<local-path>` validates a supported locale, sets
  the `cfgb_locale` cookie and returns **303**. Cookie attributes are `Secure`,
  `HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age=31536000`. Reject unsupported
  methods/locales; invalid or absent `next` falls back to the selected locale
  home. Accept only a same-site absolute path beginning with a single `/`;
  reject schemes, network-path URLs, backslashes and control characters. Require
  the path to resolve to a local route registered in the selected locale;
  otherwise use its home. The header selector uses GET links and works without JS.
  These responses also use `Cache-Control: no-store`.
- Static Assets resolves localized 404 pages using the nearest generated
  `/ja/404.html`, `/en/404.html` or bilingual `/404.html`, returning **404**.
  Fallback pages contain localized home and search links. Do not implement a
  second Worker locale-404 algorithm or SPA fallback.

Test browser navigation separately from fetch/curl requests. Static Assets'
non-navigation miss behavior can reach the Worker; `ASSETS.fetch` must preserve
its response and status. Missing images or scripts must not become successful
HTML pages. Verify HEAD returns the correct status and headers without a body.

`tests/expected/static-routes.json` lists emitted public routes, including feeds,
search and sitemap resources. `worker-routes.json` describes runtime endpoints;
`fallbacks.json` describes generated 404 assets and miss behavior. Alias redirects
are separate. Runtime endpoints, error pages, aliases and search pages are not
article canonical URLs and are excluded from the sitemap as specified in the
content contract. Preview metadata always uses configured production canonical
origin, never the temporary preview origin.

## Headers and browser behavior

CFGB generates `_headers` for static responses and `_redirects` for approved
same-site aliases. Worker redirects/errors set their own headers; static
`_headers` does not apply to them. Set nosniff, strict-origin-when-cross-origin,
DENY framing and disabled camera/microphone/geolocation. Apply a CSP consistent with local fonts,
assets, code-copy, Pagefind and Mermaid behavior; no content-time remote scripts
or fonts. Specify permitted inline-script/style handling during renderer
implementation, prefer generated script hashes, and test both themes, keyboard
access and no-JS fallbacks. No tracking cookies or analytics by default; analytics
is a future explicit opt-in.

Cloudflare Access enforcement and ordinary cache/security headers are separate
checks. Validate all exposed preview hosts rather than assuming asset, feed or
search URLs inherit protection from the first HTML request.
