# GitHub Action setup contract

The public setup Action is `ymmt2005/cfgb-action`, with one root `action.yml`.
It installs an exact immutable CFGB release and registers the executable on PATH.
The current implementation supports the raw executable release format on Linux,
macOS and Windows, for amd64 and arm64. It installs a fresh verified binary per
invocation; it does not currently cache installations. Its pinned GitHub CLI
verifier and archive digests belong to the Action, not caller configuration.

## Ownership and scope

`cfgb` owns the CLI, domain rules, renderer/Worker, embedded dependencies, schemas,
prompts and this canonical specification. `cfgb-action` owns release installation,
release-attestation verification/cache handling, PATH registration, setup outputs, Action tests and usage
documentation. `cfgb-example` supplies the CLI conformance corpus.

Workflows run `cfgb prepare`, `cfgb validate`, `cfgb summarize`, `cfgb build`,
`cfgb deploy` or `cfgb preview` directly in `run` steps after setup. The CLI handles
arguments, diagnostics, exit codes, summary ownership and deployment gates.
The Action has no `operation` selector or domain-command wrappers. Its optional
package-manager selection configures the job environment for subsequent CLI
steps without executing a build. It
never discovers content, executes a domain command or commits/pushes files.
Checkout, job permissions, trusted configuration, runtime prerequisites,
concurrency, bot commits and merge gates belong to the caller workflow.

Keep one public Action repository and entry point. If concrete GitHub-specific
requirements justify additional capabilities later, extend this Action with a
reviewed contract; v1 does not predefine wrappers for every CLI command. A
reusable workflow is a separate job-level mechanism and is not required for v1.

## Inputs and outputs

| Input | Requirement |
| --- | --- |
| `cfgb-version` | Required exact released CFGB version from an immutable release; no implicit latest, branch or range |
| `package-manager` | Optional `npm` or `pnpm`; exports `CFGB_PACKAGE_MANAGER` to subsequent steps in the same job; omission preserves the caller's environment |
| `github-token` | Optional read-only token, defaulting to the job GitHub token; used only for public release verification |

| Output | Meaning |
| --- | --- |
| `cfgb-version` | Exact installed version, emitted after successful verification |
| `cfgb-path` | Absolute verified executable path on the runner |
| `node-version` | Tested Node.js version from the selected CLI release's verified toolchain requirements |
| `npm-version` | Tested npm version from the selected CLI release's verified toolchain requirements |
| `pnpm-version` | Tested pnpm version from the selected CLI release's verified toolchain requirements |

Setup registers the binary directory on PATH for subsequent steps in the same
job. Every job that needs CFGB performs its own setup. No checkout, `cfgb.yaml`,
Node.js, AI credentials or Cloudflare upload credentials are needed to install the
CLI. Build-time Node.js/package-manager prerequisites are configured separately
by the caller according to the selected CFGB release.

An explicit `package-manager` input sets `CFGB_PACKAGE_MANAGER` through
`GITHUB_ENV` only after successful setup. It selects the CLI package manager;
it does not install npm/pnpm or change the build's arguments. Omission writes no
environment selection, preserving existing workflow/job settings. When neither
an input nor an environment selection is present, the CLI defaults to npm.

Download `toolchain-requirements.json` from the selected immutable CLI release,
verify it against that release's attestation before reading it, and check its
`cfgbVersion` matches the requested version. Emit `testedNodeVersion`,
`testedNpmVersion` and `testedPnpmVersion` as the corresponding outputs. These
are exact tested build-toolchain versions, not minimum requirements, inferred
defaults or the Action's internal Node runtime. Missing/invalid metadata fails
setup. Callers run setup first, then feed `node-version` to `actions/setup-node`
and install either the emitted npm or pnpm version before calling `cfgb build`.

## Installation and versioning

Resolve the runner OS/architecture against supported CFGB release assets; fail
clearly for an unsupported runner. Validate the exact version input before
download. Download only the selected runner-specific asset from that exact
immutable `ymmt2005/cfgb` release on github.com. Before executing the binary,
verify release immutability and GitHub's cryptographically signed release
attestation, including the selected asset's bytes and its binding to the expected
repository, tag, commit and asset identity. Checking the repository's current
immutability setting is insufficient: the selected release itself must qualify.
The Action owns platform selection and verification; callers supply only the
exact version, without platform-specific digests. A checksum file alone does
not satisfy release-attestation verification.
Do not compile from the caller checkout or accept a caller-controlled download
URL. Validate safe extraction and the installed binary's reported version.
Release asset naming/checksums and published/embedded toolchain requirements
follow the [build runtime contract](10-build-runtime.md). Setup only installs the
CLI; subsequent CLI build/upload commands manage their own retained toolchain
sessions. Treat input values literally; never interpolate them into executable shell code.

Verification must provide the guarantees of GitHub's `gh release verify` and
`gh release verify-asset`, using an explicit fixed repository and exact tag.
Illustrative commands after download of the Linux/amd64 asset are:

```sh
gh release verify vX.Y.Z --repo github.com/ymmt2005/cfgb
gh release verify-asset vX.Y.Z ./cfgb-linux-amd64 --repo github.com/ymmt2005/cfgb
```

These are verification examples, not a complete installer. The Action owns a
reviewed verifier and its trusted dependencies; if it uses `gh`, pin a compatible
version rather than assume an arbitrary runner installation supports these
commands. Verification must work without a caller checkout. If authenticated
public-release reads are needed, use the job's GitHub token without write
permissions; do not require a caller PAT, AI/upload secrets or attestation-write
permissions. Do not disable verification when the verifier or evidence is absent.

Cache by exact version, OS, architecture, asset identity and digest obtained from
verified release attestation. Verify release evidence and cached executable bytes
on every reuse before execution; cached metadata is not a trust anchor.
Repeated invocations can reuse a verified installation. Installation lives
in runner-managed storage, outside the content checkout. Setup adds no framework,
Worker, package or other files to the article repository. Action-owned installer
dependencies belong in `cfgb-action`; renderer dependencies remain embedded in CFGB.

Action releases and CLI releases are independent. Document supported release
formats and runner targets and reject unsupported combinations. Never substitute
a different CLI version. Pin the Action to a full commit SHA in trusted workflows
and independently pin `cfgb-version`. Major Action tags such as `v1` are convenience
references once published, not immutable pins. Use the same exact CFGB release
in GitHub checks and Workers Builds: `cfgb-version` = `CFGB_VERSION`. Workers
Builds retains its independently configured `CFGB_SHA256` for the fixed
Linux/amd64 asset. For the same target, this digest must match that asset in the
verified release attestation, yielding identical verified executable bytes.
Other runner targets use their own assets from the same immutable release;
cross-platform release equivalence does not imply byte identity. Consult the CLI release list for published versions.

## Failure behavior and trust boundary

Missing/malformed version input, mutable releases, absent/invalid attestations,
wrong repository/tag/asset binding, byte mismatch, unavailable verification,
unsupported assets, download/extraction failures, corrupt cache or reported-version
mismatch fail the setup step. No fallback to checksum-only or unverified installation.
Emit successful setup outputs and register PATH only after verification. Do not print credentials
or use AI/upload credentials during installation. Setup does not mint tokens or
grant repository permissions. Native step failure is mandatory; `continue-on-error`
is a caller policy, never an Action default.

Subsequent `run` steps retain the CLI's own exit status and diagnostic contract;
there is no Action translation into command-specific annotations or outputs.
Workflows must allow failed CLI steps to fail their required checks. Caller
scripts must not accidentally hide errors in pipelines or later successful commands.

The default architecture uses GitHub Actions for authoring/validation and
Cloudflare Workers Builds for production/preview deployment. Workers Builds runs
the CLI directly and does not require this Action. If deployment ownership is
explicitly moved to GitHub, invoke the same CLI in `run` steps with the existing
artifact/publication/Access gates and avoid a competing deployment path.

The [delivery contract](04-delivery.md) continues to govern trusted configuration,
separate AI/upload credentials, fork isolation, latest-head checks and scoped bot
commits. Installing CFGB does not authorize a subsequent credentialed operation.

## Implementation acceptance

Test exact-version installation from immutable releases, mutable-release refusal,
absent/invalid attestation, wrong repository/tag/asset identity, tampered binary,
unavailable verifier, safe extraction and corrupt-cache
rejection, unsupported release/runner, literal version-input handling and version
mismatch. Verify setup works without a checkout or blog configuration, creates no
repository files, emits only verified setup outputs, and makes the selected CLI
available to subsequent steps. Verify repeated setup and separate-job behavior.
Assert a matching checksum file cannot bypass a failed attestation check, and
cached bytes/evidence are reverified. Test platform selection across supported
runner targets without caller digests. For matching release/target, assert the
Workers Builds configured digest matches the Action's attested executable bytes.

CLI tests run the shared example corpus directly. The Action CI installs the
exact immutable release on native supported runners and invokes the CLI in a
later `run` step. The example's Links and Pages workflows use the separately
commit-pinned Action with CFGB v0.1.0. Domain and delivery conformance remain CLI
tests; these setup/build checks do not imply that later authoring, migration or
Cloudflare upload commands are implemented.
