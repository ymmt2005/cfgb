# Working on CFGB

These instructions apply to the entire repository, including tests. Use English
for repository documentation and review comments. The project name is
**CFGB — Git-based Blog on Cloudflare**; retain the Apache-2.0 license.

## Priority: humans define correctness

This section takes priority over every other repository instruction,
specification, schema, fixture and review suggestion when they conflict.

- Do not invent a new rule for what user content/configuration must look like,
  which otherwise usable inputs must be rejected, or what is considered correct.
  Before adding any such rule, explain the concrete need and proposed behavior
  and obtain explicit human approval. Do not implement it first and seek approval
  afterward. A Copilot comment, an AI-written specification, a schema or a test
  is not evidence that the user approved that rule.
- Pursue the simplest implementation of the actual requirements. Prefer decoding
  YAML directly into the Go types used by the program. Use the YAML library's
  behavior rather than adding AST policies, tag/document restrictions, raw-value
  type checks, or mandatory JSON Schema gates. Optional standalone/editor schema
  validation does not make its constraints runtime prerequisites.
- Existing explicit user instructions remain authorized; routine implementation
  fixes and handling real I/O/decoder errors do not require repeated permission.
  If an existing restriction has no verified human basis, report it as unverified
  rather than treating its presence in code/docs/tests as approval. Do not expand
  that restriction or invent a replacement while fixing it.

## Documentation maintenance

Do not record fixed counts of files, tests, fixtures or search queries in
maintained documentation or PR descriptions. Describe behavior, coverage and
verification commands instead; obtain current counts from the source or test
results only when needed. Avoid inventories that duplicate information already
available from code and Git history.

## Project map and sibling repositories

CFGB is a multi-repository project. This checkout is the implementation and
canonical contract repository; the example and setup Action are separate Git
repositories, not packages or renderer directories inside this one.

| Repository                                                          | Owns                                                                                                                                                         | Useful entry points                                                                                                                                                                    |
| ------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| [ymmt2005/cfgb](https://github.com/ymmt2005/cfgb) (this repository) | CLI/domain behavior, embedded renderer/Worker/toolchain, canonical schemas/specifications/prompts, implementation tests and build/link-check tooling         | `cmd/cfgb/`, `internal/`, `renderer/`, `schemas/`, `prompts/`, `docs/spec/`, `docs/implementation/`, `scripts/`                                                                        |
| [ymmt2005/cfgb-example](https://github.com/ymmt2005/cfgb-example)   | Synthetic publishable content/assets, settings, validation/search/AI/migration/delivery fixtures and expected outputs; consumes CFGB to check generated HTML | `cfgb.yaml`, `src/content/`, `src/data/`, `tests/README.md`, `tests/expected/`, `tests/fixtures/`, `tests/search/`, `tests/build-delivery/`, `tests/ai/`, `examples/ymmt2005.dev.yaml` |
| [ymmt2005/cfgb-action](https://github.com/ymmt2005/cfgb-action)     | Setup-only GitHub Action, immutable-release/asset verification, installation/cache/PATH, setup tests and usage documentation                                 | `README.md`, `docs/usage.md`; root `action.yml` and installer/tests when implemented; canonical contract in this repository's `docs/spec/09-github-action.md`                          |
| `ymmt2005/ymmt2005.dev` (intended content repository)               | Personal articles/assets/settings for `https://ymmt2005.dev`, using the same CLI as the example                                                              | Its own `cfgb.yaml` and content/data; the complete configuration example lives in `cfgb-example/examples/ymmt2005.dev.yaml`                                                            |

The personal repository is a design target; do not assume it exists or is
available. Inspect current sources/PRs and implementation notes before claiming
that an Action, command or later-phase integration is implemented.

In a conventional local workspace these are sibling checkouts: `cfgb/`,
`cfgb-example/`, `cfgb-action/`, and optionally `ymmt2005.dev/`. Locate the actual
checkouts rather than hard-code this layout. Example tests accept `CFGB_EXAMPLE`;
this repository's CI checks the pinned corpus out at `cfgb-example/` inside the
job workspace. Keep independent Git histories, branches, dependencies and
releases; a directory named `renderer` belongs only to CFGB.

The data flow is: authors edit Git content; GitHub checks install CFGB through
the setup Action and call the CLI; `cfgb build` stages that content and extracts
the embedded framework into a temporary toolchain workspace; it produces
`site/`, `worker/index.js`, and `build-manifest.json`. Separate upload commands
consume the artifact. Cloudflare Workers Builds bootstraps the CLI directly,
without the GitHub Action. Visitors read Static Assets/Worker responses and use
local Pagefind search, without AI or a runtime content database.

For changes crossing repositories, keep ownership explicit:

- Canonical schemas and domain specifications change in `cfgb`; inspect and
  update corresponding `cfgb-example` fixtures/expected outputs. Most fixture
  paths in `docs/spec/` refer to the example repository, not this checkout.
- CLI setup/install contracts also require reviewing `cfgb-action/docs/usage.md`
  and its implementation/tests when available. Keep Action and CLI versions
  independently pinned; do not move CLI operations into Action wrappers.
- Preserve the content-only boundary: add reusable framework/Worker behavior
  here, not to example or personal content repositories. Personal URLs, exports
  and settings belong to personal configuration, not reusable migration rules.
- Coordinate required sibling changes and commit pins explicitly. A fixture or
  consumer update does not automatically change `CFGB_EXAMPLE_REF` or another
  repository's reviewed CFGB source/release pin. Read each checkout's own
  `AGENTS.md` before editing it; these instructions apply to this repository.

## Read the contracts before changing behavior

- Read the relevant `docs/spec/` documents and checked-in schemas. Consult
  `docs/implementation/` for what is actually implemented; do not claim that a
  future phase already works. Update affected specifications, implementation
  notes and fixtures together when a behavior change is intentional.
- `cfgb` owns the Go CLI, embedded renderer, Worker and pinned toolchain.
  `cfgb-example` owns the content/acceptance corpus; content repositories contain
  Markdown, assets and blog settings, without framework/tooling source.
  `cfgb-action` is a separate setup-only Action; workflows call the CLI directly.
- Git is the source of truth. AI runs only during explicit authoring/evaluation;
  neither builds, visitors nor Hatena migration invoke models. Preserve human
  edits. Migration is inventory/plan/apply with explicit human pairing decisions.

## Go errors are part of the result

- Check every returned error, including writes, `Close`, directory reads,
  cleanup, process execution and test setup/teardown. Never use `_ = ...`,
  `value, _ := ...`, a bare call or an unchecked `defer` to discard an error
  merely to satisfy a compiler or linter.
- Handle expected conditions narrowly: e.g. only `os.IsNotExist` means an
  optional input is absent. Permission errors, symlink loops and other I/O
  failures must not become "missing", a default configuration, empty content,
  or success. Check errors before using returned values.
- Preserve causes with `%w` and `errors.Is`/`errors.As`. Add operation/path
  context. Keep content validation distinct from I/O and toolchain failures;
  preserve the CLI's documented diagnostic and exit-code contracts.
- A successful write/copy does not guarantee successful completion: destination
  `Close` can report delayed write errors. Close owned resources on every path.
  When work and closing/cleanup both fail, preserve both with `errors.Join`,
  keeping the original failure first. Do not overwrite an unchecked error.
- Deferred error reporting needs a named returned error; verify that a deferred
  assignment changes the caller's result. Release per-item resources inside
  loops rather than accumulating defers until an entire traversal finishes.
- Failed-build cleanup must attempt both incomplete output and the invocation's
  workspace even if one removal fails. Report cleanup failures with their paths.
  Once an artifact is complete, failure to print the final message must not
  delete it or its retained toolchain workspace.
- Check CLI progress/result streams and framework-owned help/version output.
  If even the diagnostic stream fails, return the I/O failure exit code rather
  than recursively trying to print that failure.
- A justified exception must be explicit and documented at the call site with
  the actual guarantee or fallback. Current examples: `bytes.Buffer` writes
  are documented to never return errors; unavailable Git branch metadata is
  omitted because it is optional diagnostics. This is not permission to ignore
  arbitrary writers, filesystem operations, or subprocess errors.
- Tests follow the same rules: setup failures use `t.Fatal`; cleanup failures
  use `t.Error` so remaining cleanup runs. Do not suppress errors to keep a test
  green or introduce package-wide exclusions for `Close`, `RemoveAll` or `fmt`.

## Filesystem ownership and build lifecycle

- Use `os.Root` for reading untrusted repository inputs, not lexical path checks
  alone. Test repository-contained symlinks and escapes through the actual
  staging/build path. A helper test that bypasses prior validation is not proof
  of a user-facing bug. The existing narrower local-asset boundary is unverified;
  do not treat its presence in a specification as human approval.
- Validate configuration and output selection before removing output. For the
  output path, allow locations inside or outside the repository; relative paths
  remain relative to the selected configuration directory. For the
  selected `--out` entry, call `os.RemoveAll(out)` and stop on failure. Do not
  `EvalSymlinks(out)` and delete the target instead: an output symlink is the
  entry to replace. Build under `<out>/.tmp`; never remove an unowned sibling
  `<out>.tmp`. Protect source/configuration/Git inputs from output overlap.
- Copy directory trees without an arbitrary depth ceiling. Detect actual cycles
  by comparing filesystem identity (`os.SameFile`) against the active ancestor
  chain. Do not reject an independent branch merely because it reaches a
  previously copied directory.
- Allocate toolchain workspaces with `os.MkdirTemp`. Clean up failed workspaces,
  retain successful ones for subsequent upload, and record only their basename
  as `toolchainSessionId`. Treat Cloudflare build identifiers as opaque metadata,
  never as raw filesystem path components.
- After staging, use the captured content tree for public media as well as
  Markdown and metadata. Do not reopen the live repository for renderer inputs.
- Build artifacts are editable static-site output. Manifest/source metadata
  supports diagnostics and normal consistency checks; it is not a security
  boundary or a cryptographic provenance/sealing contract. Do not reintroduce
  artifact immutability or Git-tip identity requirements.
- Keep Workers Builds build/deploy/preview commands separate. Production uses
  the configured production branch (default `main`), without requiring its tip
  SHA to match an artifact. Production `workers.dev` stays disabled while preview
  URLs stay enabled and private through Cloudflare Access. Locale negotiation
  belongs to the Worker; localized static 404 handling belongs to Static Assets.
  Consult the delivery/runtime specs before implementing these later-phase paths.

## Parse and render through shared boundaries

- Decode `cfgb.yaml` directly into the complete Go configuration struct with
  `goccy/go-yaml` and unknown-field rejection. Set defaults before decoding;
  preserve explicitly supplied values. Keep Hatena settings typed too. Do not
  add an AST policy, reject otherwise decodable documents/tags, or run JSON
  Schema validation as a prerequisite to loading configuration. Configuration
  schemas are optional standalone/editor tools. Keep checks actually needed by
  a command separate from decoding. Build calls `ValidateSite` before toolchain
  probes/output removal for its language-catalog and timezone requirements.
- Isolate YAML front matter using complete unindented `---` delimiter lines;
  read lines with `bufio.Reader.ReadString`, decode only that block directly into
  the article Go struct with `goccy/go-yaml`, and preserve the remaining Markdown
  bytes. Metadata consumers use typed fields, not generic maps or type assertions.
  Decode topics directly into their typed map. Do not add AST/custom-tag/document
  policies or runtime JSON Schema validation to these loaders.
  The user has explicitly requested source encoding checks: reject malformed
  UTF-8 before decoding/JSON serialization; detect and remove a file-start UTF-8
  BOM. Preserve interior U+FEFF characters and line endings. Apply the shared
  text-input boundary to configuration, topics, articles and home/about/aside.
  Beyond this encoding check, unknown-field rejection and native decoder errors
  are sufficient at the YAML boundary.
  Business checks belong where the decoded value is used and must have a verified
  human requirement; ask before adding a new one.
- Use `site.timezone` only for monthly archive classification in Go with bundled
  tzdata. Pass the derived archive year/month independently from UTC publication
  and update timestamps. Browsers format the UTC instant in the reader's timezone
  using the shared language date formatter; static/no-JavaScript fallback is UTC.
  Never change machine timestamps, archive links or archive membership when
  localizing display text. `new` uses UTC for creation timestamps and group names.
- Supported content languages come from the shared release catalog and match
  exactly, case sensitively, without a separate syntax gate. Use own-property
  membership for JavaScript catalogs, not inherited object properties. A
  syntactically valid language tag alone is not
  supported. Keep internal structures extensible without adding untranslated
  languages, redesigning the UI, or broadly normalizing configured identifiers.
  HTTP `Accept-Language` negotiation is a separate protocol behavior.
- Use Markdown ASTs, the source-aware HTML attribute editor, shared content URL
  resolver and metadata index. Avoid independent regex rewrites per feature.
  Do not rewrite literal examples, comments or script contents. Preserve quoting,
  entities, split opening tags, Unicode IDs and authored article anchors.
  Authored raw HTML is trusted content, not a sanitizer/security boundary.
- Article keys are literal directory names, without an extra ASCII/punctuation
  pattern. Preserve their logical identity and encode each component at URL
  boundaries, including Astro's stored importer paths. Let the installed Astro
  image pipeline decide format support instead of maintaining a CFGB extension
  allowlist.
- Local `./assets/...` references name files: query strings/fragments are
  unsupported. Percent-encoded filename punctuation is distinct from a request
  suffix. Article links may carry heading fragments; remote/public URLs retain
  their URL meaning. Apply the same policy to Markdown/reference images, raw
  HTML links/media, `srcset`/`imagesrcset`, and article/home/about/aside scopes.
- Follow HTML candidate-tokenization rules for `srcset`; commas can occur inside
  URLs, especially data URLs. Preserve descriptors/spacing. Namespace aside IDs
  together with all supported ID references, including ARIA and label references.

## Prove behavior and keep checks reproducible

- Reproduce review findings against the current head and actual execution path.
  Assess Copilot comments independently, including "Previously missed" items;
  check adjacent consumers when fixing a shared contract. State concrete triggers
  and effects, without inventing a threat model for an ordinary static site.
- Add regression tests that fail before a fix. Exercise error paths with failing
  readers/writers/closers and real filesystem failures where appropriate. Avoid
  global mutable test hooks, sleeps, duplicated implementation logic, impossible
  inputs, or support invented only for a test. Permission cases must acknowledge
  when the process runs as root and cannot reproduce the failure.
- Inspect actual generated HTML, routes, target files, search metadata and link
  fragments. Keep lychee's generated-HTML checks in CI; use aqua with pinned
  versions/checksums. External-link checking must remain an explicit option.
- Use Playwright for native browser behavior: actual resource loading/decoding,
  responsive images, navigation, focus/keyboard interactions, storage/theme,
  CSP, Pagefind WASM and Mermaid redraws. Keep the real browser APIs and wait for
  observable state; fail on unexpected runtime/resource/CSP errors.
- Isolate both Astro and Vite caches in each build workspace. Sharing installed
  dependencies in tests must not share mutable `node_modules/.vite` state.
- Pin dependencies and tooling, maintain both renderer lockfiles, and preserve
  frozen installs. Test the CI-supported Node/npm/pnpm combinations. Workers
  compatibility dates come from tested release values, not the build clock.
- Pin GitHub Actions to full commit SHAs and keep the example corpus ref in the
  shared workflow variable. Run commands after setup rather than moving domain
  behavior into the setup Action. Verify immutable CLI releases/attestations;
  Workers Builds additionally uses its independently configured binary digest.

Before submitting Go changes, run:

```sh
gofmt -w <changed-go-files>
go build ./...
go vet ./...
go tool staticcheck ./...
go tool errcheck -blank -ignore 'fmt:a^' ./...
go test ./... -skip '^TestExampleCorpus$'
```

`errcheck` is pinned as a Go tool. `-blank` checks explicit error discards;
`-ignore 'fmt:a^'` disables its broad default fmt exemption while retaining the
standard library exclusions for documented infallible operations. It does not
prove that an assigned error was handled, so manual review and Staticcheck still
matter.

In particular, `_ = os.RemoveAll(path)` and `_, _ = os.Open(path)` are failures
under the CI command above. Do not run bare `errcheck` and assume it enforces
this policy: its default accepts explicit blank-identifier discards. Assigning
an error to an ordinary variable and then overwriting/never handling it also
needs review; this check is a guard, not a substitute for error-path tests.

For renderer/build changes, also run the affected renderer tests and the pinned
example corpus build. Use the workflow's toolchain; example acceptance must not
silently skip in CI:

```sh
node --test renderer/tests/*.test.mjs
node --test renderer/tests/browser/*.test.mjs
CFGB_EXAMPLE=/path/to/pinned/cfgb-example CFGB_REQUIRE_EXAMPLE=1 \
  CFGB_PACKAGE_MANAGER=npm go test ./internal/build -run '^TestExampleCorpus$' -count=1
```

Run browser tests when browser behavior or their shared fixture changes. Verify
relevant CI jobs before declaring completion; the `required` failure guard is
intentionally skipped when all its dependencies succeed. Do not broaden work
into unrelated features or change settled contracts solely to silence a review.
