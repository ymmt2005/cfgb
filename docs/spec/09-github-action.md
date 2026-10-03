# GitHub Action setup contract

Status: design only. No Action implementation, published tag or active workflow
is provided yet. The public Action will be `ymmt2005/cfgb-action`, with one root
`action.yml`. Its v1 responsibility is to install a verified CFGB release and
register its executable on PATH for subsequent workflow steps.

## Ownership and scope

`cfgb` owns the CLI, domain rules, renderer/Worker, embedded dependencies, schemas,
prompts and this canonical specification. `cfgb-action` owns release installation,
checksum/cache handling, PATH registration, setup outputs, Action tests and usage
documentation. `cfgb-example` supplies the CLI conformance corpus.

Workflows run `cfgb prepare`, `cfgb validate`, `cfgb summarize`, `cfgb build`,
`cfgb deploy` or `cfgb preview` directly in `run` steps after setup. The CLI handles
arguments, diagnostics, exit codes, summary ownership and deployment gates.
The Action has no `operation` selector or command-specific inputs/outputs. It
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
| `cfgb-version` | Required exact released CFGB version; no implicit latest, branch or range |

| Output | Meaning |
| --- | --- |
| `cfgb-version` | Exact installed version, emitted after successful verification |
| `cfgb-path` | Absolute verified executable path on the runner |

Setup registers the binary directory on PATH for subsequent steps in the same
job. Every job that needs CFGB performs its own setup. No checkout, `cfgb.yaml`,
Node.js, AI credentials or Cloudflare upload credentials are needed to install the
CLI. Build-time Node.js/package-manager prerequisites are configured separately
by the caller according to the selected CFGB release.

## Installation and versioning

Resolve the runner OS/architecture against supported CFGB release assets; fail
clearly for an unsupported runner. Download only the selected version's artifacts
from `ymmt2005/cfgb` releases and verify the published SHA-256 before execution.
Do not compile from the caller checkout or accept a caller-controlled download
URL. Validate safe extraction and the installed binary's reported version.
Release asset naming/checksums and published/embedded toolchain requirements
follow the [build runtime contract](10-build-runtime.md). Setup only installs the
CLI; subsequent CLI build/upload commands manage their own retained toolchain
sessions. Treat input values literally; never interpolate them into executable shell code.

Cache by exact version, OS, architecture and checksum; verify bytes on every
reuse. Repeated invocations can reuse a verified installation. Installation lives
in runner-managed storage, outside the content checkout. Setup adds no framework,
Worker, package or other files to the article repository. Action-owned installer
dependencies belong in `cfgb-action`; renderer dependencies remain embedded in CFGB.

Action releases and CLI releases are independent. Document supported release
formats and runner targets and reject unsupported combinations. Never substitute
a different CLI version. Pin the Action to a full commit SHA in trusted workflows
and independently pin `cfgb-version`. Major Action tags such as `v1` are convenience
references once published, not immutable pins. Use the same CFGB release in
GitHub checks and Workers Builds. No versions are published by this documentation.

## Failure behavior and trust boundary

Malformed version input, unsupported assets, download/checksum/extraction failures,
corrupt cache or reported-version mismatch fail the setup step. Emit successful
setup outputs and register PATH only after verification. Do not print credentials
or use AI/upload credentials during installation. Setup does not obtain tokens or
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

Test exact-version installation, safe extraction, checksum failure, corrupt-cache
rejection, unsupported release/runner, literal version-input handling and version
mismatch. Verify setup works without a checkout or blog configuration, creates no
repository files, emits only verified setup outputs, and makes the selected CLI
available to subsequent steps. Verify repeated setup and separate-job behavior.

CLI tests run the shared example corpus directly. Once implemented, a smoke
workflow can install CFGB using this Action and invoke the CLI in a later `run`
step; domain and delivery conformance remain CLI tests. No live Action acceptance
is claimed at this design stage.
