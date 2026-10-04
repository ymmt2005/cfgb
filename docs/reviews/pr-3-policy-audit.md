# PR #3: input-policy audit

## Authority and scope

The user explicitly requires simple direct YAML-to-Go decoding and human approval
before an AI adds a new definition of valid/correct input. This requirement takes
priority over AI-written specifications, schemas, fixtures and review suggestions;
see the first section of [AGENTS.md](../../AGENTS.md).

This audit examines the current PR's Go input/build paths and renderer checks.
It distinguishes verified user instructions from restrictions for which an
explicit human instruction could not be verified. The original first design
conversation was not available as approval evidence. A rule's presence in a
specification, schema, test or earlier PR snapshot is not proof of approval.
**Unverified does not prove that the user never approved it**; it means an agent
must not assume approval or expand the rule.

Decoder/I/O failures and the mechanics needed to carry out an authorized operation
are different from a new content policy. For example, propagating a failed write
is not a new rule banning some otherwise usable article.

## Removed from YAML loading

- AST inspection and generic `map[string]any` projection of configuration and
  article front matter; metadata consumers now use typed Go fields.
- Mandatory configuration/article JSON Schema validation before loading.
- Independently rejecting custom YAML tags or counting YAML documents. The YAML
  decoder supplies its own behavior; tags need not be visible to Go consumers.
- Raw-YAML scalar type/presence rules that reject values the Go decoder accepts.
  Empty fields, numeric text converted by the decoder, folded/literal strings and
  empty slices are not rejected by an additional loader policy.
- The front-matter BOM ban and independent YAML/article-body UTF-8 rejection
  passes. The opening delimiter can have a BOM, and the remaining article body
  bytes are preserved.
- Astro's additional nonempty-title/slug/date and nonempty-topics shape gates.
  Its schema supplies collection types, not extra authoring requirements.

Unknown-field rejection remains allowed by the user's instruction. Native decoder
errors are still reported. `Metadata` uses Go strings/slices/time values, and the
publication snapshot uses typed records. Optional standalone/editor schemas do
not silently become runtime prerequisites.

## Four simplifications approved and implemented

On 2026-10-04 the user explicitly approved all four simplifications highlighted
in the review summary. They are now implemented:

- Article keys use literal filesystem directory names without CFGB's ASCII
  character pattern. Go metadata preserves the key; public media URLs and
  Astro's URL-resolved importer paths encode their components.
- `--out` can be outside the repository. Relative selection still uses the
  configuration directory. Existing overlap checks and exact entry removal
  remain; a final symlink is replaced without deleting its target.
- Both directory-copy paths compare `os.SameFile` identities against the active
  ancestor chain. They detect actual cycles without a fixed depth cap and allow
  independent branches to copy the same directory.
- The image staging adapter has no CFGB extension allowlist. The installed Astro
  image pipeline owns format support and errors.

Regression coverage includes arbitrary filesystem keys, external output and
symlink replacement, trees deeper than 64 levels, independent directory aliases,
root/nested cycles, failed-build cleanup and image-adapter format delegation.
An actual CLI/Astro/Pagefind build combines an external artifact directory,
a Unicode/punctuation-bearing article group and 80-level media delivery.
The four-digit year discovery convention is separate from article-key characters
and remains in the unverified list below.

## Follow-through on already explicit input boundaries

The user's existing instructions require exact, case-sensitive membership in the
translated-language catalog and ordinary Markdown body loading without extra
encoding rules. The continuation removes two inconsistent leftover gates:

- Go and JavaScript no longer apply a language-tag regex before catalog lookup.
  JavaScript checks own properties so inherited names such as `constructor` are
  not mistaken for translated languages. This applies both to configuration and
  direct UI-copy lookup. The optional identifier schema remains editor guidance.
- Home/about/aside loaders no longer reject BOMs or invalid UTF-8 independently
  of the Markdown/JSON pipeline. Their read bytes are preserved just like article
  bodies. JSON serialization still performs its native replacement of invalid
  UTF-8; this is not a promise of arbitrary-byte preservation in HTML.

Tests cover exact unsupported identifiers in both implementations, inherited
JavaScript names, all three prose kinds, BOMs, invalid bytes and line endings.
The extended real example build verifies those prose files render through the
native JSON/Markdown path. No other unverified restriction is changed here.

## Additional Copilot review checked against the current implementation

Six comments on the preceding head identified implementation defects:
working-directory and configuration I/O failures used exit 2; public media reopened the
live repository after staging; an empty timezone reached Intl unchanged; equal
article keys in different years had no final ordering tie-break; and aside
namespacing omitted three standard HTML ID-reference attributes.

These fixes implement existing behavior without new input bans. I/O failures use
exit 3 while decoder/discovery errors remain exit 2 (temporary workspace creation
is covered by the same distinction). Media reads the captured snapshot. Empty
timezone follows Go's UTC interpretation at the renderer adapter, rather than
Copilot's suggested rejection. Publication order uses the full group as a final
tie-break. `itemref`, `popovertarget` and `commandfor` follow namespaced aside IDs.
Coverage includes real deleted-cwd/config-read failures, output preservation,
media delivery after live-source removal, empty-timezone CLI rendering, actual
home/feed ordering and native popover/dialog interaction without JavaScript.

## Remaining restrictions with no verified explicit human instruction

These are remaining existing behavior, **not new rules proposed or approved by
this audit**. The four approved simplifications above are separate; these
remaining restrictions have not been expanded or replaced. They require a human
decision before an agent can treat them as an approved correctness requirement.

| Existing behavior                                                                                                                           | Implementation                                                                                       | Effect / question for human review                                                                                                                                                                                                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Year directories must be exactly four digits                                                                                                | `internal/frontmatter/frontmatter.go`, `isYear`; `internal/build/build.go`, `isArticleYear`          | Nonmatching year-directory names are silently excluded. This discovery convention was not part of the approved article-key character change.                                                                                                                      |
| An article group may contain only the shared `assets/` directory                                                                            | `internal/frontmatter/frontmatter.go`, `readGroup`                                                   | Other subdirectories fail even when they contain no variant. This is broader than rejecting an unsupported-language `.md` file.                                                                                                                                   |
| Output paths overlapping configured content/linkcard roots, Git or configuration inputs are rejected using broad ancestor/descendant checks | `internal/build/build.go`, `outputDir`, `overlaps`                                                   | Preventing actual source deletion has a concrete purpose, but forbidding every descendant of a configured content/cache root is an additional location policy; it is not automatically authorized by the instruction to delete exactly the selected output entry. |
| Local asset references must remain inside `assets/`, not merely the article group/repository                                                | `renderer/src/lib/content-urls.mjs`, `localAsset`                                                    | `./assets/../other.png` fails even if the target remains in the article group. The narrower boundary needs its own justification/approval.                                                                                                                        |
| Repeated slugs within a language fail rather than applying any precedence                                                                   | `internal/frontmatter/frontmatter.go`, `collectPosts`; `renderer/src/lib/load-site.mjs`, `loadSite`  | This avoids conflicting output routes, but the exact rejection policy is still an existing documented rule without a verified original human instruction.                                                                                                         |
| The otherwise valid Go timezone `Local` is forbidden                                                                                        | `internal/config/site.go`, `ValidateSite`                                                            | This is a reproducibility choice beyond whether Go can load/use that timezone. It is separate from YAML decoding.                                                                                                                                                 |
| Language labels cannot be blank, and the default must be a configured key                                                                   | `internal/locale/locale.go`, `Validate`; `renderer/src/lib/load-site.mjs`, `assertConfiguredLocales` | These label/default required-value policies are separate from the explicitly requested case-sensitive supported-language membership and still need verified approval.                                                                                             |
| npm >=12 and pnpm >=11 are mandatory                                                                                                        | `internal/build/build.go`, `checkToolchain`; `renderer/package.json`                                 | Independent minimum-version gates accompany the pinned/tested toolchain. Exact toolchain pins and the Node supported-range contract have a different basis from these package-manager floors.                                                                     |
| Fixed CSP/framing/permissions policies constrain authored HTML and embedding                                                                | `renderer/src/lib/security-headers.json`; Static Assets and Worker responses                         | External scripts, inline scripts, many iframe/media/form uses and embedding the blog are restricted. Browser tests show that CFGB's UI works under this policy; they do not establish user approval of restrictions on authored content.                          |

The optional schemas also retain exact slug/topic/alias/image-path patterns,
nonempty fields and other authoring conventions. They are no longer YAML loading
gates. Adding enforcement to `build` or a new command still requires approval;
optional schema files are not an authorization source.

## Proposed disposition, not an approval record

The following is a concrete simplification proposal. It does **not** authorize
implementing new policies. It applies to the remaining unverified restrictions,
not the four approved and implemented changes above. Removing a restriction also needs assessment of its consumers; do
not replace it with another unexplained check to make existing tests pass.

| Existing restriction                     | Suggested direction                                                   | Reason / work to check                                                                                                                                                                              |
| ---------------------------------------- | --------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Four-digit year discovery                | Review separately from article-key identity                           | Nonmatching years currently disappear silently. The user approved removing key character restrictions, not changing the discovery layout.                                                           |
| Only `assets/` subdirectories in a group | Ignore unrelated directories during variant discovery                 | Discover the supported Markdown variants rather than reject every extra author-owned folder. Unsupported-language variants are a separate, explicitly requested check.                              |
| Broad output overlap checks              | Protect actual inputs without a blanket cache/content descendant rule | Preventing destruction of real source inputs is useful. A rule about every possible descendant is broader; review symlinks and the staging/removal order rather than invent canonicalization rules. |
| Narrow asset boundary                    | Agree on the intended reference scope before changing it              | Repository containment is explicit; `assets/` containment is narrower. Media publication paths and original-file imports must work together if the scope changes.                                   |
| Duplicate slug rejection                 | Retain only if the human accepts it as the chosen collision behavior  | Two articles currently target the same output route. Rejection has a direct practical purpose, but an agent must not silently choose overwrite/precedence as a replacement.                         |
| `Local` timezone ban                     | Decide whether machine-local time is acceptable                       | Go can use it. Reproducibility is a possible product preference, not a decoder failure or proof of approval.                                                                                        |
| Language label/default checks            | Review the remaining site-use requirements separately                 | Missing default selection affects routing; blank display labels affect presentation. Exact catalog membership is already implemented; these required-value policies are separate product decisions. |
| Package-manager minimum versions         | Keep only toolchain constraints with an actual compatibility basis    | Distinguish a tested pin from a necessary minimum. Inspect the embedded lockfiles, package-manager-specific features and dependencies before relaxing the floors.                                   |
| Fixed CSP/framing/permissions policy     | Review authored-content and embedding requirements with the human     | The policy blocks valid uses of trusted authored HTML. UI/browser tests prove the current UI works, not that arbitrary blog content should be restricted.                                           |

## Optional-schema restrictions are a different category

The following checked-in constraints are **not runtime loading requirements**:

- Configuration: schema version `1`, nonempty title, HTTPS-origin-only base URL,
  `home.latestPosts` between 1 and 50, required nested fields, and format/pattern
  constraints on optional provider/import/deployment settings.
- Article metadata: ASCII single-hyphen slugs/topic IDs, a nonempty title,
  required fields, nonempty/unique topics, single-line summary, restricted OG
  image/alias path characters, and string/date formats.
- Topics: localized-map shape and topic-key/label constraints in the standalone
  topic schema; Go loads its typed map without a Schema pass.

Some design requirements have visible human support, such as the summary modes,
semantic duplicate-alias diagnostic, configured production branch and mandatory
private preview. This list does not withdraw those instructions. It distinguishes
their command-specific checks from requiring the entire Schema as a gate. Nor
does it prove that every remaining Schema constraint was explicitly approved.

Do not turn the configuration field table, a negative fixture or the acceptance
guide into an automatic implementation mandate. The CLI/acceptance/delivery
documents now distinguish decoded Go values from optional raw-value Schema
scenarios: numeric/null conversions follow the decoder, duplicate aliases are
semantic checks, and Schema validation is not a required earlier stage.

The standalone Go schema helper remains available but is not imported by the
configuration/front-matter/build packages. Its presence is not a CLI integration
or a requirement to add one. No new Schema command was implemented.

Text handling also has separate stages: loaders preserve article and prose body bytes,
but Go's JSON encoder applies its native string encoding when those records go
to the renderer. The loader test is not proof of byte-for-byte preservation of
arbitrary invalid UTF-8 through JSON and HTML. This does not authorize adding an
encoding rejection policy.

The sibling example fixtures are now separated accordingly. The nine null,
numeric and boolean summary cases check the decoded Go string: null follows
the empty-summary mode rule, while numeric/boolean scalars do not produce a
raw-type error. A loader regression test verifies all three conversions.
Five optional standalone Schema cases live in a separate file, including the
former invalid-slug/external-alias cases and the three raw summary types.
These cases do not authorize CLI rejection. `validate` is not implemented in
this PR, so its future semantic fixture outcomes are not claimed as executed
command acceptance.

## Instructions verified in the visible user discussion

- Use `os.Root` for repository input reads and respect the repository boundary.
- Delete the selected `--out` entry with `os.RemoveAll(out)`, without resolving it
  and deleting a symlink target.
- Use `os.MkdirTemp` for invocation-owned toolchain workspaces, retaining successful
  workspaces for upload and recording their basename.
- Accept only translated/supported content languages, with case-sensitive matching;
  keep extensible internals without redesigning the current UI.
- Do not support query/fragment suffixes on local file references. This does not
  authorize unrelated filename/image-extension restrictions.
- Check Go errors, including explicit blank-identifier discards; document justified
  exceptions and exercise actual error paths.
- Keep the renderer in CFGB, keep content repositories framework-free, and provide
  a separate setup-only Action. Use lychee via checksum-pinned aqua and browser
  tests for actual browser behavior.
- Treat static-site artifacts as editable operator-owned output rather than a
  cryptographic provenance/security boundary.

Generic permission to fix bugs or add tests is not approval to invent a new
input restriction. New policy needs its own concrete explanation and explicit
human confirmation, including when suggested by Copilot.
