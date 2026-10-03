# Build runtime, bootstrap and provenance

Status: implementation contract only. No release, installer, CLI, deployment
workflow or Cloudflare resource is implemented/provisioned by this document.
This contract complements [CLI](01-cli.md), [delivery](04-delivery.md) and the
[setup Action](09-github-action.md).

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
| `NODE_VERSION` | Exact supported Node.js version selected from the release requirements |
| `PNPM_VERSION` | Exact pnpm version specified by the release |
| `SKIP_DEPENDENCY_INSTALL` | `1`; CFGB owns dependency installation outside the content checkout |

Each CFGB release publishes `toolchain-requirements.json` and embeds the same
requirements. Required fields are `schemaVersion: 1`, `nodeRange` (SemVer range),
`testedNodeVersion` (exact version satisfying that range), `pnpmVersion`,
`wranglerVersion`, `workerCompatibilityDate` (tested Workers runtime date in
`YYYY-MM-DD` form), `rendererVersion` and `lockfileHash` (SHA-256 of embedded
lockfile bytes). A compatible Node range such as `24.x` is illustrative, not an
already selected runtime. The actual versions are verified when releasing CFGB.
Wrangler must meet the Preview minimum of 4.135.0.

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
"$HOME/.local/bin/cfgb" build --out dist
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

## Node, pnpm and toolchain workspace

Node.js is not pinned by a JavaScript package lockfile. Before dependency install,
`build` checks the actual Node runtime against the embedded `nodeRange` and the
actual pnpm against exact `pnpmVersion`; it does not trust environment-variable
values as proof of the installed versions. Workers Builds uses `NODE_VERSION`
and `PNPM_VERSION` to provision them. Other environments install compatible Node
and the same pinned pnpm independently. Unsupported/missing runtimes fail with
`E_TOOLCHAIN`, exit 2, before rendering or upload.

`build` creates a toolchain session outside the content repository. In Workers
Builds `toolchainSessionId` is the 64-character lowercase hexadecimal SHA-256
of the exact UTF-8 bytes of `buildUUID`, and its root is
`$HOME/.cache/cfgb/builds/<sha256(buildUUID)>/`. Never use the raw build identifier
as a path component. Elsewhere use an opaque random session ID under CFGB's user
cache. Reject symlink/path escapes regardless of the hashed component. The
workspace contains extracted package/lockfile sources,
`pnpm install --frozen-lockfile` dependencies, including the pinned Wrangler, and
private session metadata. Dependency installation is allowed network access;
content rendering, indexing and integration checks subsequently run offline.

Retain this workspace after `build` returns and through the Deploy/Preview
command in the same Workers Build. Cleanup is after upload completion/failure or
build-environment disposal, never at successful build return. The workspace is
not part of the deployable artifact and does not belong in Git, static assets,
artifact transfer or visitor runtime. No credentials are stored in session state.

`build-manifest.json` records `toolchainSessionId` and toolchain requirements plus
observed versions; it contains no absolute workspace paths or node_modules.
Session metadata binds its ID, original raw build identifier when present,
source identity, embedded lockfile digest, verified
artifact-manifest digest and installed tool versions. Upload resolves that session
through CFGB's cache registry/deterministic Workers Build path, verifies these
bindings and invokes Wrangler by its workspace path. Never use a global Wrangler
or unpinned npx resolution.
For Workers Builds, recompute the session ID from the manifest's raw `buildUUID`
and require it to match the recorded `toolchainSessionId` before resolving the
workspace. A hash-derived path does not replace exact raw provenance comparison.

Same-build uploads require the original matching session. Missing/corrupt session
state fails with `E_TOOLCHAIN`, exit 2; no silent different-version fallback. The
session binding prevents accidental cross-build/session reuse and preserves
toolchain consistency; it is not a cryptographic artifact-authenticity seal. For
an artifact intentionally moved outside the original build environment, the CLI
may recreate only the identical embedded upload toolchain using the same CFGB
release, after artifact/runtime verification. It must not render, generate content
or alter artifact bytes. This recreation is dependency installation, not a site
rebuild. Record a fresh local session binding; preserve artifact provenance.

Upload rechecks the actual Node/pnpm/Wrangler versions and session integrity.
Within the original session, observed Node must also match the build observation;
a recreated upload session can use another Node version within the recorded
supported range, while retaining a separate record of that upload runtime. The
manifest's `toolchain` records `nodeRange`, `testedNodeVersion`, `pnpmVersion`,
`wranglerVersion`, `workerCompatibilityDate`, `rendererVersion`, `lockfileHash`
and observed `nodeVersion`;
observed pnpm/Wrangler must match their exact required versions. The selected
CFGB release's requirements must match the artifact's recorded requirements.
Altered/incompatible artifact metadata fails with `E_ARTIFACT`, exit 1.

## Generated Wrangler runtime configuration

The CFGB release pins `workerCompatibilityDate` together with its Worker source
and Wrangler version, and records it in the artifact's toolchain requirements.
Release testing covers that exact runtime date for production and preview.
Build/deploy/preview must not derive it from their execution date, the source
commit date or the current platform default. Changing it requires a reviewed,
tested CFGB release. Missing/invalid embedded requirements fail `E_TOOLCHAIN`,
exit 2; a mismatching artifact requirement fails `E_ARTIFACT`, exit 1.

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

## Artifact verification scope

The artifact manifest, output hashes, source checks and toolchain-session bindings
are deployment-correctness and reproducibility mechanisms. They establish that
CFGB is operating on a self-consistent artifact for the expected source/runtime
context and that upload does not silently rebuild or change its bytes. They do
not provide cryptographic authentication of a site artifact against a malicious
artifact store, transfer channel or compromised deployment environment.

CFGB v1 assumes CI artifact storage/transfer and the deployment environment are
inside the operator's trusted CI boundary. This is appropriate for the site
artifact, which is deployed by that same operator rather than distributed as a
trusted executable to third parties. If a deployment environment with upload
credentials is compromised, an attacker can deploy arbitrary Worker/assets
regardless of the CFGB manifest, so CFGB does not add a site-artifact signing or
attestation requirement in v1. Operators that need a stronger provenance model
may layer an external digest/signature/CI attestation on top; CFGB neither
requires nor interprets it.

Transferred artifacts are therefore supported without a CFGB-specific external
attestation. They still undergo the ordinary artifact-byte/hash, source identity,
publication, runtime-version and target checks, and upload-toolchain recreation
must leave their bytes unchanged. Supply-chain verification of the CFGB
executable itself remains a separate security mechanism defined by the release
and setup-Action contracts.

## Provenance resolution

When `WORKERS_CI=1` or any Workers Builds provenance variable is present, use the
Workers Builds adapter. Its authoritative inputs are:

| Variable | Build manifest field |
| --- | --- |
| `WORKERS_CI_COMMIT_SHA` | `sourceCommit` |
| `WORKERS_CI_BRANCH` | `sourceBranch` |
| `WORKERS_CI_BUILD_UUID` | `buildUUID` |

All three must be present and valid in Workers Builds; missing/invalid or
checkout-mismatched metadata fails with `E_BUILD_SOURCE`, exit 1. Do not fall back to Git
branch discovery on missing/partial CI metadata. Store `provenanceProvider:
workers-builds` and validate the declared commit against `git rev-parse HEAD`.
Detached HEAD is normal; it does not replace the CI branch with `HEAD`. Treat the
branch as a literal ref name, never executable input. `buildUUID` is optional in
the general artifact contract but required for Workers Builds artifacts.

Treat `WORKERS_CI_BUILD_UUID` as a non-empty opaque UTF-8 build identifier,
not an RFC UUID. Store its exact value as `buildUUID`, without trimming,
case-folding, parsing UUID syntax or normalizing path separators. Compare the
original values exactly during same-build upload. Cloudflare's
[event examples](https://developers.cloudflare.com/workers/ci-cd/builds/event-subscriptions/)
also use prefixed build identifiers; their format is not a CFGB validation rule.

These platform variables can be overridden. Trusted build settings must leave
them platform-managed, and repository scripts may not replace them. Environment
provenance alone does not prove a checkout is trustworthy; retain clean-source,
input-hash and commit checks.

Outside Workers Builds, obtain commit and branch from the original Git checkout
and record `provenanceProvider: git`. Never inspect the extracted renderer/upload
workspace for branch identity. Detached Git checkouts can build read-only artifacts
with an unknown branch, but uploads requiring a branch must fail with
`E_BUILD_SOURCE`, exit 1. Other CI upload pipelines must check out the actual
reviewed named branch; do not guess from tags or silently assign main.

Build and upload compare current source identity to the finalized artifact. In the
same Workers Build require commit, branch and build UUID to agree; retry builds
have their own UUID/session and build their own artifact. Reject mismatches before
upload with `E_BUILD_SOURCE`, exit 1. Production requires the recorded/current
branch to be `main`; Preview requires a non-main branch. Pass the authoritative
branch to the pinned Wrangler Preview adapter, derive its Preview name using
the pinned Wrangler naming behavior, and pass that name explicitly with `--name`.
Do not let Wrangler infer it from detached HEAD or a temporary workspace. Reject
invalid/colliding names and verify branch/name mapping against the pinned Wrangler
during implementation.
A transferred artifact retains its original build UUID for traceability; it does
not impersonate the new environment's build UUID.

## Acceptance

Before relying on this integration, test the bootstrap and separate command shells
in a disposable Workers Build: exact binary/hash reuse, retained session/Wrangler,
Node/pnpm version checks, no toolchain files in the artifact, CI detached HEAD,
partial/overridden/mismatched provenance, opaque build-ID/session isolation and clean-source
checks. Test transferred-artifact upload toolchain recreation separately, asserting
zero renderer calls and identical artifact bytes. Data fixtures are future test
inputs, not evidence of executed Cloudflare deployment.
Include non-RFC identifiers and identifiers containing path separators/traversal
text: preserve the raw manifest value, derive only the lowercase SHA-256 workspace
component, and never create paths from raw values. Empty identifiers fail source
validation; different identifiers must not reuse an artifact/session from another build identity.
