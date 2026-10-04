# Working on CFGB

These instructions apply to the entire repository, including tests. Use English
for repository documentation and review comments. The project name is
**CFGB — Git-based Blog on Cloudflare**; retain the Apache-2.0 license.

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
  of a user-facing bug. Preserve the documented narrower boundary of local assets.
- Validate configuration and output selection before removing output. For the
  selected `--out` entry, call `os.RemoveAll(out)` and stop on failure. Do not
  `EvalSymlinks(out)` and delete the target instead: an output symlink is the
  entry to replace. Build under `<out>/.tmp`; never remove an unowned sibling
  `<out>.tmp`. Protect source/configuration/Git inputs from output overlap.
- Allocate toolchain workspaces with `os.MkdirTemp`. Clean up failed workspaces,
  retain successful ones for subsequent upload, and record only their basename
  as `toolchainSessionId`. Treat Cloudflare build identifiers as opaque metadata,
  never as raw filesystem path components.
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

- Isolate YAML front matter using complete unindented `---` delimiter lines;
  read lines with `bufio.Reader.ReadString`, parse only that block with
  `goccy/go-yaml`, and preserve the remaining Markdown bytes. Retain UTF-8/BOM,
  duplicate-key, unknown-field and reader-error checks. Do not parse the body
  as YAML or replace this with a whole-document delimiter regex.
- Validate unprojected inputs against their canonical schemas before typed
  projection/defaults; semantic rules remain separate. Preserve semantic codes
  such as duplicate-alias diagnostics rather than moving them into schema
  constraints. Missing/empty summaries are structurally valid for authoring.
- Supported content languages come from the shared release catalog and match
  exactly, case sensitively. A syntactically valid language tag alone is not
  supported. Keep internal structures extensible without adding untranslated
  languages, redesigning the UI, or broadly normalizing configured identifiers.
  HTTP `Accept-Language` negotiation is a separate protocol behavior.
- Use Markdown ASTs, the source-aware HTML attribute editor, shared content URL
  resolver and metadata index. Avoid independent regex rewrites per feature.
  Do not rewrite literal examples, comments or script contents. Preserve quoting,
  entities, split opening tags, Unicode IDs and authored article anchors.
  Authored raw HTML is trusted content, not a sanitizer/security boundary.
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
