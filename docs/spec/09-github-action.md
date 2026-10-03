# Unified GitHub Action contract

Status: design only. No Action implementation, published tag or active workflow
is provided yet. The public Action will be `ymmt2005/cfgb-action`, with one root
`action.yml`. Setup, preparation, validation, summaries, build and optional upload
operations share this entry point and release stream. Add future capabilities to
this Action rather than publishing separate setup/build/summary Actions.

## Ownership

`cfgb` owns the CLI, domain rules, renderer/Worker, embedded dependencies, schemas,
prompts and this canonical specification. `cfgb-action` owns installation,
operation-to-CLI argument mapping, GitHub annotations/outputs, Action tests and
usage documentation. `cfgb-example` supplies the shared conformance corpus.
The Action obtains a released CFGB executable; it never copies renderer sources,
implements summary ownership or migration rules, or runs scripts from an article
repository. Node.js remains a prerequisite for build operations.

One invocation selects one operation. Workflows combine multiple invocations of
the same Action for preparation and final validation, with separate job permissions
where necessary. The Action does not silently run a complete publication pipeline
or commit/push changes. Checkout, events, concurrency, permissions, trusted
configuration, bot commits and merge gates belong to the caller workflow.
A reusable workflow is a separate job-level mechanism; it is not required for v1.

## Inputs and CLI mapping

Inputs are typed and allowlisted, not an arbitrary command or shell-argument
string. Unknown operations, unsupported combinations and malformed inputs fail
before running CFGB. Treat every input literally when constructing an argument
vector; never interpolate article content or input values into executable shell
code. Relative paths use the selected configuration directory as in the CLI.

| Input | Requirement / default |
| --- | --- |
| `operation` | `setup` (default), `prepare`, `validate`, `summarize`, `build`, `deploy`, `preview` |
| `cfgb-version` | Required exact released CFGB version; no implicit latest, branch or range |
| `working-directory` | Default `.` within the checked-out repository; no symlink/out-of-repository escape |
| `config` | Optional; omission retains nearest `cfgb.yaml` discovery from the working directory |
| `validation-mode` | `default` (default), `authoring`, `publish`; only nondefault for `validate` |
| `articles` | Optional newline-separated article keys, one per line; `prepare`/`summarize` only |
| `changed-since` | Git ref for `summarize`; choose this or explicit articles, not both |
| `lang` | Enabled locale restricting `summarize` selection; same CLI semantics |
| `offline` | `false` default; `prepare` only |
| `refresh-links` | `false` default; `prepare` only |
| `dry-run` | `false` default; `prepare`/`summarize` only |
| `out` | Artifact directory, `dist` default; `build` only |
| `from` | Required artifact directory for `deploy`/`preview` |

Boolean inputs accept literal `true`/`false` strings. Default values of irrelevant
inputs do not activate another operation; nondefault operation-specific inputs
for the wrong operation are errors. Do not expose `--replace-manual` or a
clock override in the public v1 Action. Human summary replacement remains an
explicit local CLI operation. Migration, remote old-site edits and AI evaluation
remain direct CLI operations in v1; future automation support uses this same entry point.

| Operation | Effect |
| --- | --- |
| `setup` | Install verified binary and register PATH; no content operation |
| `prepare` | Run `cfgb prepare`; report assets and prepare link-card caches; no AI |
| `validate` | Run `cfgb validate` with the selected mode; read-only |
| `summarize` | Run `cfgb summarize` for explicit selection or `--changed-since`; no automatic commit |
| `build` | Run `cfgb build --out ...`; no upload, no content generation |
| `deploy` | Run `cfgb deploy --from ...`; production gate and upload, no rebuild |
| `preview` | Run `cfgb preview --from ...`; private-preview gate and upload, no rebuild |

Every operation first ensures the requested verified CFGB version is available.
An empty summary selection without `changed-since` is an error, never all articles.
A `prepare` invocation without article selection follows the CLI's all-variant
behavior. Inputs and selection must not alter existing manual-edit protection.

## Installation and versioning

Resolve the runner OS/architecture against supported CFGB release assets; fail
clearly for an unsupported runner. Download only the selected version's artifacts
from `ymmt2005/cfgb` releases and verify the published SHA-256 before execution.
Do not compile from the caller checkout or accept a caller-controlled download
URL. Validate safe extraction and the installed binary's reported version.

Cache by exact version, OS, architecture and checksum; verify bytes on every
reuse. Repeated invocations can reuse a verified installation. Installation lives
in runner-managed storage, not in the content repository, and setup adds its bin
directory to PATH for later steps. Action-owned adapter dependencies belong in
`cfgb-action`; renderer dependencies remain embedded in CFGB. Node.js/package
manager requirements for a build are documented per CFGB release and checked
before rendering. Installation never creates Astro/Worker/package files in the
article repository.

Action releases and CLI releases are independent. Maintain an explicit supported
CLI-version range in each Action release and reject incompatible combinations;
never fall back to a different CLI version. Pin the Action to a full commit SHA
in trusted workflows and independently pin `cfgb-version`. Major Action tags such
as `v1` are convenience references once published, not immutable pins. Use the
same CFGB release in GitHub checks and Workers Builds. No versions are published
by this documentation change.

## Outputs and failure behavior

| Output | Meaning |
| --- | --- |
| `cfgb-version` | Exact installed version after verified setup |
| `cfgb-path` | Absolute executable path on the runner after verified setup |
| `exit-code` | CFGB's actual exit status when invoked; absent if invocation never occurred |
| `diagnostics-path` | Runner-temporary JSON diagnostics file when available |
| `artifact-path` | Absolute verified artifact directory after successful `build` |
| `preview-url` | Ready preview URL only after successful Access verification |

Use the CLI's structured diagnostics for annotations and summaries, preserving
stable codes and warning/error severity. A warning-only validation succeeds;
a nonzero CLI exit fails the step. Preserve exit categories 1/2/3/4 in the output
rather than presenting every error as validation failure. An Action-side input or
installation error fails the step with a clear diagnostic, without inventing a
CLI exit status. Native step failure is mandatory; caller `continue-on-error`
is a caller policy, never an Action default. Do not expose a ready artifact or
preview URL on failure. Mask credentials and avoid raw provider/export content
in logs, outputs and annotations.

## Delivery and trust boundary

The default architecture uses GitHub Actions for authoring/validation and
Cloudflare Workers Builds for production/preview deployment. Workers Builds runs
the CFGB CLI directly; it does not require the GitHub Action. Action `deploy` and
`preview` operations expose the same CLI for explicitly chosen alternative
pipelines; they must not create a competing deployment path in the default setup.
Switch deployment ownership deliberately if adopting GitHub-hosted uploads.

Credentials are environment variables defined by the CLI/AI/delivery contracts,
not arbitrary secret-valued Action inputs. The Action uses only credentials
needed by the selected operation; it never obtains tokens or grants permissions
itself. Keep AI and upload credentials in separate jobs. Use trusted configuration
for credentialed preparation and pin the Action/CLI independently of PR content.
Fork checks remain read-only and credential-free. Do not execute fork code with
secrets through `pull_request_target`. Same-repository push access remains limited
to trusted maintainers and scoped bots as specified in [delivery](04-delivery.md).

The Action does not bypass the latest-head check, protected deployment environment,
main-only publication, artifact integrity or mandatory private-preview requirements.
Selecting a mutating/upload operation is explicit; mere token availability does
not activate it. A bot commit is a separate reviewed caller-workflow step with
scoped write permissions, selected-file allowlists and stale-head protection.

## Implementation acceptance

Test every operation against the same pinned example corpus and compare CLI
arguments, diagnostics and exit status. Include installation checksum failure,
corrupt cache, unsupported OS/architecture or CLI version, literal input injection,
wrong-operation inputs, omitted/empty summary selection, missing Git history for
`changed-since`, warning-only authoring success and failed-publication gating.
Verify setup creates no repository files; preparation preserves human summaries;
build never uploads; uploads never rebuild; preview failures emit no ready URL.
Test permissions and stale-head/bot behavior with disposable workflows once the
Action exists. Fixtures document expectations; no live integration test is claimed
at this design stage.
