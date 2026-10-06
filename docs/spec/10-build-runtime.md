# Build runtime, bootstrap and provenance

Release binaries and the setup Action implement the installation part of this
contract. Production upload through the retained toolchain is implemented.
Workers Builds bootstrap integration and private previews remain later work.
Operators provision the Cloudflare account/zone and Access policies separately.
This contract complements [CLI](01-cli.md), [delivery](04-delivery.md) and the
[setup Action documentation](https://github.com/ymmt2005/cfgb-action#readme).

## Release and Workers Builds bootstrap

CFGB publishes a standalone executable for each supported OS/architecture, with
release checksums and toolchain requirements. For Workers Builds the v1 asset
contract is `cfgb-linux-amd64` under the exact release tag in `ymmt2005/cfgb`.
The checksum is SHA-256 over executable bytes, not an archive. Other targets have
separate assets/checksums; never substitute a different architecture or version.
`ymmt2005/cfgb` will use immutable releases: published release artifacts cannot
be replaced. The setup Action verifies the exact release and runner-specific
asset through GitHub release attestation. Workers Builds retains the independently
reviewed `CFGB_SHA256` pin for its fixed Linux/amd64 executable. Both paths verify
bytes before execution, including reused downloads/installations.

| Environment | Installation verification |
| --- | --- |
| GitHub setup Action | Exact version, immutable release and verified release attestation for the selected runner asset |
| Workers Builds | Exact version and independently reviewed `CFGB_SHA256` for `cfgb-linux-amd64` |
| Manual local installation | Exact version; GitHub release-attestation verification is recommended |

Publish all platform assets, checksums and toolchain requirements in the draft
before publishing the immutable release. Published assets cannot be filled in
later. Release notes or latest/pre-release labels do not determine CLI selection.

A trusted maintainer configures the following Workers Builds variables; they are
not read from articles or PR-controlled scripts:

| Variable | Contract |
| --- | --- |
| `CFGB_VERSION` | Exact reviewed release tag, e.g. `vX.Y.Z`; no latest/range |
| `CFGB_SHA256` | Reviewed 64-character lowercase SHA-256 for that executable |
| `NODE_VERSION` | Exact Node release inside `>=24.15.0 <25` or `>=26.0.0`. Node 24.21.0 is the tested Active LTS. Install npm >= 12 separately; these lines bundle npm 11 |
| `PNPM_VERSION` | Optional. A pnpm release >= 11, used only when `CFGB_PACKAGE_MANAGER=pnpm`. The tested release is recorded in `pnpmVersion` |
| `SKIP_DEPENDENCY_INSTALL` | `1`; CFGB owns dependency installation outside the content checkout |

The CFGB executable embeds the renderer `package.json`, `package-lock.json`, and
`pnpm-lock.yaml`, and the build copies those files into the workspace. `engines.node`
is the Node range. npm >= 12.0.0 and optional pnpm >= 11.0.0 are the installer
floors. The tested Node, npm, and pnpm releases are `24.21.0`, `12.2.0`, and
`12.8.1`; the manifest records them. The exact `wrangler` dependency is the
Wrangler pin, and it must be at least 4.135.0. `workerCompatibilityDate` is the
Workers runtime date pinned with the Worker source, in `YYYY-MM-DD` form. The
lockfiles are the dependency pins. A compatible Node range such as `24.x` is
illustrative, not an already selected runtime. The actual versions are verified
when releasing CFGB.

The Build command first downloads the pinned binary into a temporary file,
verifies the independently configured digest, installs it under
`$HOME/.local/bin/cfgb`, then calls it. Version/digest values must be validated
before download. Use only the fixed release origin and exact asset contract;
never download/execute an unverified installer script. This illustrative command
assumes reviewed, correctly formatted variables and a future published release:

```sh
set -eu
: "${CFGB_VERSION:?required}" "${CFGB_SHA256:?required}"
cfgb_download="$(mktemp)"
trap 'rm -f "$cfgb_download"' EXIT
curl --fail --silent --show-error --location \
  --proto '=https' --proto-redir '=https' \
  "https://github.com/ymmt2005/cfgb/releases/download/${CFGB_VERSION}/cfgb-linux-amd64" \
  --output "$cfgb_download"
printf '%s  %s\n' "$CFGB_SHA256" "$cfgb_download" | sha256sum --check --status
mkdir -p "$HOME/.local/bin"
install -m 0755 "$cfgb_download" "$HOME/.local/bin/cfgb"
"$HOME/.local/bin/cfgb" build --force --out dist
```

The configured Build command must fail before build on download/checksum/install
failure. Same-build Deploy and Preview commands use this installed executable,
rechecking its configured SHA-256 before invocation. For production:

```sh
set -eu
: "${CFGB_SHA256:?required}"
printf '%s  %s\n' "$CFGB_SHA256" "$HOME/.local/bin/cfgb" | sha256sum --check --status
"$HOME/.local/bin/cfgb" deploy --from dist
```

Preview uses the same verification prefix followed by
`"$HOME/.local/bin/cfgb" preview --from dist`. Do not rely on a PATH export from a
previous command shell or repeat an unpinned install. Retain the installation
through the current Workers Build; the next build bootstraps and verifies its own
selected release. In Workers Builds the CLI also checks its embedded release
version against `CFGB_VERSION` before build/upload and rejects a mismatch with
`E_TOOLCHAIN`, exit 2. These are command-setting examples, not files to add to content
repositories. The GitHub setup Action has its own installer implementation and
uses the same release asset/requirements contract with the attestation verification
defined in its setup specification. Match Action `cfgb-version` to `CFGB_VERSION`.
For the same Linux/amd64 target, the independently reviewed Workers Builds
`CFGB_SHA256` must match the asset digest in the verified release attestation;
both environments then verify the same executable bytes. Action callers need no
platform-specific digest configuration. Other runner targets use different
executable bytes from the same exact immutable CFGB release.

## Node, npm and toolchain workspace

Node.js is not pinned by a JavaScript package lockfile. npm >= 12 is the default
installer because that release makes dependency install scripts opt-in. Before
dependency install, `build` checks the actual Node runtime against the renderer's
`engines.node` and requires the actual npm to be >= 12.0.0. npm 11 and older still
run those scripts by default, so they fail `E_TOOLCHAIN`. The check does not
require an exact npm patch, and it does not trust environment-variable values as
proof of the installed versions. Observed Node, npm, and pnpm strings must be
valid semantic versions. Comparison uses semantic version precedence, so a
prerelease is lower than the same numbered release and does not satisfy a
stable range, including a prerelease whose numeric version is above the floor. Malformed output, including a numeric prefix with trailing junk,
is rejected before the range is applied. Workers Builds uses `NODE_VERSION` to provision
Node 24.15.0 or newer on the Node 24 line, or Node 26.0.0 or newer. Node 25 is
outside npm 12's supported engines. Node 24 (Krypton) is the tested Active LTS
release. Node 26 and newer are accepted. These releases bundle npm 11, so the
environment also installs npm >= 12. Other environments do the same.

`CFGB_PACKAGE_MANAGER` selects the installer. An empty value or `npm` runs
`npm ci` and `npm exec`. `pnpm` requires pnpm >= 11.0.0, then runs
`pnpm install --frozen-lockfile`. pnpm 10 and older are rejected. Any other value fails
with `E_TOOLCHAIN`, exit 2. The variable is a build-environment setting. It is
not a field in the content repository's `cfgb.yaml`. Workers Builds sets
`PNPM_VERSION` only for that optional path. Unsupported or missing runtimes fail
with `E_TOOLCHAIN`, exit 2, before rendering or upload.

`build` creates a fresh toolchain workspace outside the content repository:

```go
workspace, err := os.MkdirTemp("", "cfgb-build-*")
```

`MkdirTemp` creates that directory with mode `0700` before umask. The returned
directory is this invocation's workspace. Open it as an `os.Root` when an operation
must stay inside it. Every build gets a new directory, including two builds that
carry the same `buildUUID`. Do not reuse a user-cache path, and do not derive the
directory name from `sha256(buildUUID)` or from the raw build identifier.

`build-manifest.json` records `toolchainSessionId` as `filepath.Base(workspace)`.
A workspace `/tmp/cfgb-build-1234567890` records `cfgb-build-1234567890`. That
field is the generated basename, not an absolute path and not a caller-supplied
relative path. The manifest also records toolchain requirements and observed
versions. It contains no absolute workspace path, `node_modules`, or source
snapshot. `buildUUID` remains the original opaque `WORKERS_CI_BUILD_UUID` when
that variable is present. It is diagnostic metadata, and it does not name the workspace.

In the same build environment, deploy and preview resolve the workspace as
`filepath.Join(os.TempDir(), toolchainSessionId)`. Those commands must share the
temporary-directory setting and filesystem with the build that created it. Reject
an ID that is absolute, contains a path separator, or is `.` or `..` before
joining it. Do not recompute the ID from `buildUUID`.

A failed build removes the workspace that invocation created and does not remove
another build's workspace. A successful build retains the workspace for the
subsequent deploy or preview command. Do not remove it when build returns
successfully. After upload completion or failure, remove that workspace. Disposal
of the build environment also ends its lifetime. Production deploy is implemented;
private previews remain later work.
Preflight failures and `deploy --dry-run` retain the build session; an actual
upload attempt consumes it whether it succeeds or fails. The workspace contains
extracted package and lockfile sources, `npm ci` dependencies
by default (or the frozen pnpm install when selected), including the pinned
Wrangler, and private session metadata. The renderer `package.json` `allowScripts`
field permits install scripts for `esbuild` and `workerd`. The pnpm
path permits the same two packages through `allowBuilds` in
`pnpm-workspace.yaml`, and sets `fsevents` to false. sharp 0.35 has no
install script. Every other dependency install script stays blocked.
Dependency installation is allowed network access;
content rendering, indexing and integration checks subsequently run offline.

The workspace is not part of the deployable artifact and does not belong in Git,
static assets, artifact transfer or visitor runtime. No credentials are stored in
workspace state.

Session metadata records the workspace basename, the original raw build
identifier when present, and the installed tool versions. Upload resolves the
recorded basename under the shared temp directory and invokes Wrangler by its
workspace path. A difference in source commit, branch, or build identifier
leaves that lookup unchanged. Never use a global Wrangler or unpinned npx resolution.

Same-build uploads require that workspace. Missing or corrupt workspace state fails
with `E_TOOLCHAIN`, exit 2; no silent different-version fallback. The
recorded basename selects that invocation's workspace.
Upload-toolchain recreation is not implemented yet. For an artifact moved
outside the original build environment, a future adapter may recreate only the
identical embedded upload toolchain using the same CFGB
release, after the required-file and runtime checks. It must not render, generate content,
or alter artifact bytes. This recreation is dependency installation, not a site
rebuild. Record a fresh local session for the recreated toolchain. The supplied
artifact, including its diagnostic metadata, stays unchanged.

Upload rechecks the actual Node version against the recorded supported range
and verifies the exact installed Wrangler. Build observations remain diagnostic;
another compatible Node version can run the retained upload toolchain. Upload
invokes Wrangler directly through Node and does not invoke or require npm/pnpm.
A future recreated upload session would need the appropriate installer to recreate
its frozen dependencies, while recording that session's runtime separately. The
manifest's `toolchain` records `nodeRange`, `testedNodeVersion`, `packageManager`
(the installer this build used), `npmVersion`, `pnpmVersion`, `wranglerVersion`,
`workerCompatibilityDate`, `rendererVersion`, and observed `nodeVersion`,
`observedNpmVersion` and `observedPnpmVersion`. `nodeRange` is the renderer's
`engines.node`. `wranglerVersion` is its exact `wrangler` dependency.
`workerCompatibilityDate` is the date pinned with the Worker source.
An npm build records the observed npm version and requires npm >= 12.0.0.
A pnpm build records the observed pnpm version and requires pnpm >= 11.0.0.
Observed Wrangler must match its exact required
version. The artifact's `workerCompatibilityDate` must match the release pin
used to generate Wrangler configuration. A mismatch fails with `E_ARTIFACT`, exit 1.

## Generated Wrangler runtime configuration

The CFGB release pins `workerCompatibilityDate` with its Worker source and pins
Wrangler in the renderer package manifest. The artifact records both. Release
testing covers that exact runtime date for production and preview.
Build/deploy/preview must not derive the date from their execution date, the source
commit date or the current platform default. Changing it requires a reviewed,
tested CFGB release. A missing `engines.node`, or a Wrangler dependency that is
not an exact version of at least 4.135.0, fails `E_TOOLCHAIN`, exit 2. An
artifact whose `workerCompatibilityDate` differs from the release pin fails
`E_ARTIFACT`, exit 1.

Generate temporary Wrangler configuration for both upload commands with top-level
`compatibility_date` equal to that pinned value, `workers_dev: false`,
`preview_urls: true` and an explicit `previews: {}`. Emit these values for both
production and preview configuration; do not rely on dashboard state or defaults.
The production workers.dev route is disabled while workers.dev Preview URLs
remain enabled. The canonical custom domain remains the public production origin.
Cloudflare also enables Version URLs through `preview_urls`; include them in
the Access protection/coverage checks together with Preview/deployment URLs.
Enabling URLs is separate from authorizing access to them.
Keep `assets` at the top level, including the `ASSETS` binding and selective
Worker-first routing from the delivery specification. V1 has no Preview-specific
vars, secrets or storage bindings; the empty Preview object is sufficient for
this runtime. If such bindings are added in a future release, their Preview-safe
configuration requires a separate reviewed contract. Access remains mandatory;
an empty `previews` object does not provide privacy protection.

Illustrative required runtime fields (not a complete deployment configuration,
and not a claim that this date is already tested for a released CFGB Worker):

```json
{
  "compatibility_date": "2026-10-03",
  "workers_dev": false,
  "preview_urls": true,
  "previews": {}
}
```

Use the same pinned date for retained and recreated upload toolchains. Validate
the generated configuration with the pinned Wrangler during release acceptance,
and assert both production and preview configurations include these fields.
Verify the canonical custom domain remains public, the ordinary production
workers.dev route is disabled, and Preview/Version URLs remain enabled with
anonymous access denied by the reviewed Access policy. Test a fresh Worker and
an existing Worker whose URL settings were previously different.
No Wrangler configuration or runtime-date override belongs in a content repository.

## Artifact checks

Upload checks that `site/`, `worker/index.js`, and `build-manifest.json` are
present, that the artifact format is one this release can upload, and that the
recorded `workerCompatibilityDate` matches the release pin used to generate
Wrangler configuration. It also checks the Node, package-manager, and Wrangler
versions required to run that toolchain, the publication snapshot against the
command's date rule, the current invocation's branch, and Preview Access. The
upload sends the supplied bytes. An edit made after the build remains in that
upload. CFGB does not rebuild the site during upload.

The workspace basename locates the installed toolchain for a same-environment
upload. Its lifetime and cleanup rules are unchanged. A missing or corrupt
workspace fails with `E_TOOLCHAIN`, exit 2.

CFGB v1 treats CI artifact storage, transfer, and the deployment environment as
operator-trusted. The operator may publish an artifact they edited after the
build. An external digest or CI attestation may accompany that artifact; CFGB
does not require or interpret one for site files. Verification of the downloaded
CFGB executable remains the release, bootstrap, and setup-Action contract.

## Diagnostic source metadata

Source commit, branch, build identifier, and provenance provider are optional.
Record a value when the environment supplies it. A missing value, a partial set,
or a difference from the current checkout leaves the build and the upload
successful.

When `WORKERS_CI=1` or any of the Workers Builds variables below is non-empty,
record `provenanceProvider: workers-builds` and each value that is present:

| Variable | Build manifest field |
| --- | --- |
| `WORKERS_CI_COMMIT_SHA` | `sourceCommit` |
| `WORKERS_CI_BRANCH` | `sourceBranch` |
| `WORKERS_CI_BUILD_UUID` | `buildUUID` |

A detached HEAD records `WORKERS_CI_BRANCH` when that variable is present. The
branch is a literal ref name. `buildUUID` stores the exact variable value,
including prefixes and path separators, without trimming, case-folding, UUID
parsing, or path normalization. An empty value is omitted. The workspace path
remains the generated basename. Cloudflare's
[event examples](https://developers.cloudflare.com/workers/ci-cd/builds/event-subscriptions/)
also use prefixed build identifiers; their format is not a CFGB validation rule.

These platform variables can be overridden. Trusted build settings leave them
platform-managed, and repository scripts may not replace them. The recorded
values stay diagnostic.

Outside Workers Builds, record commit and branch from the content checkout and
`provenanceProvider: git` when those values are available. A detached checkout
with no CI branch omits `sourceBranch`, and the build still succeeds. Read that
identity from the content checkout. The extracted renderer workspace is not a
source of branch identity. Other CI upload pipelines pass the current
invocation's branch into the production-branch guard below.

Production compares the current invocation's branch with
`deploy.productionBranch` (parser default `main`). Preview requires a different
branch. A wrong branch fails before upload with `E_DEPLOY_TARGET`, exit 1.
Align the platform's production branch and branch protection with the configured
value. Pass the current branch to the pinned Wrangler Preview adapter, derive
its Preview name using the pinned Wrangler naming behavior, and pass that name
explicitly with `--name`. Derive the name from that branch rather than from a
detached HEAD or a temporary workspace. Reject invalid or colliding names and
verify the branch and name mapping against the pinned Wrangler during
implementation. A transferred artifact keeps the build UUID already stored in
its manifest. Upload leaves that file unchanged.

The manifest also records `publications`: each variant's article key, locale,
slug, summary, and timestamps. Production deploy uses that snapshot for the
current-time future-date check. The snapshot is publication metadata. It is not
a hash of source content.

## Acceptance

Before relying on this integration, test the bootstrap and separate command shells
in a disposable Workers Build: exact binary hash reuse of the CFGB executable,
retained session and Wrangler, Node and npm version checks, the optional pnpm >= 11
path, no toolchain files in the artifact, a detached HEAD that records an
available CI branch, and opaque build-ID and session isolation. Missing, partial,
and differing diagnostic metadata stay successful, as does a dirty checkout.
Test transferred-artifact upload toolchain recreation separately, asserting
zero renderer calls and identical artifact bytes. Data fixtures are future test
inputs, not evidence of executed Cloudflare deployment.
Include non-RFC build identifiers and identifiers containing path separators.
Preserve a non-empty raw `buildUUID`. It is never a path component.
`toolchainSessionId` is only the generated workspace basename. An empty build
identifier is omitted, and the build succeeds. Two builds
do not share a workspace, even when their build identifiers are equal.
