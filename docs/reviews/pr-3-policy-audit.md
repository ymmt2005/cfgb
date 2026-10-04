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

## Remaining restrictions with no verified explicit human instruction

These are existing behavior, **not new rules proposed or approved by this audit**.
They have not been expanded or replaced during this change. They require a human
decision before an agent can treat them as an approved correctness requirement.

| Existing behavior                                                                                                                           | Implementation                                                                                      | Effect / question for human review                                                                                                                                                                                                                                |
| ------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Reject a BOM or invalid UTF-8 in home/about/aside prose                                                                                     | `internal/frontmatter/frontmatter.go`, `collectProse`                                               | Article bodies now pass through; these prose files still have an independent encoding ban.                                                                                                                                                                        |
| Article keys must match lowercase ASCII words separated by single hyphens; years must be exactly four digits                                | `internal/frontmatter/frontmatter.go`, `articleKeyPattern`, `isYear`                                | Uppercase, underscores, dots and other article-key names fail. Other year-directory names are silently excluded. The exact naming restrictions are not established by direct YAML decoding.                                                                       |
| An article group may contain only the shared `assets/` directory                                                                            | `internal/frontmatter/frontmatter.go`, `readGroup`                                                  | Other subdirectories fail even when they contain no variant. This is broader than rejecting an unsupported-language `.md` file.                                                                                                                                   |
| `--out` must be inside the repository                                                                                                       | `internal/build/build.go`, `outputDir`                                                              | An absolute output directory outside the repository fails. Keeping repository inputs rooted and preventing deletion of source inputs do not themselves require this output-location rule.                                                                         |
| Output paths overlapping configured content/linkcard roots, Git or configuration inputs are rejected using broad ancestor/descendant checks | `internal/build/build.go`, `outputDir`, `overlaps`                                                  | Preventing actual source deletion has a concrete purpose, but forbidding every descendant of a configured content/cache root is an additional location policy; it is not automatically authorized by the instruction to delete exactly the selected output entry. |
| Directory copying rejects nesting deeper than 64                                                                                            | `internal/build/build.go`, `copyRootPath`, `copyRootToRootDepth`                                    | A finite deep tree is rejected together with genuinely cyclic input. The fixed depth is not actual cycle detection.                                                                                                                                               |
| Local asset references must remain inside `assets/`, not merely the article group/repository                                                | `renderer/src/lib/content-urls.mjs`, `localAsset`                                                   | `./assets/../other.png` fails even if the target remains in the article group. The narrower boundary needs its own justification/approval.                                                                                                                        |
| Repeated slugs within a language fail rather than applying any precedence                                                                   | `internal/frontmatter/frontmatter.go`, `collectPosts`; `renderer/src/lib/load-site.mjs`, `loadSite` | This avoids conflicting output routes, but the exact rejection policy is still an existing documented rule without a verified original human instruction.                                                                                                         |
| A CFGB-owned extension allowlist controls local Markdown image imports                                                                      | `renderer/src/lib/image-paths.mjs`, `imageExtensions`                                               | Files outside nine listed extensions are rejected before Astro processes them. This does not necessarily track the installed image service's capabilities.                                                                                                        |
| The otherwise valid Go timezone `Local` is forbidden                                                                                        | `internal/config/site.go`, `ValidateSite`                                                           | This is a reproducibility choice beyond whether Go can load/use that timezone. It is separate from YAML decoding.                                                                                                                                                 |
| Language labels cannot be blank, and the default must be a configured key                                                                   | `internal/locale/locale.go`, `Validate`                                                             | These are existing site-use checks, additional to the explicitly requested case-sensitive supported-language membership. Their exact required-value policy needs verified approval.                                                                               |
| npm >=12 and pnpm >=11 are mandatory                                                                                                        | `internal/build/build.go`, `checkToolchain`; `renderer/package.json`                                | Independent minimum-version gates accompany the pinned/tested toolchain. Exact toolchain pins and the Node supported-range contract have a different basis from these package-manager floors.                                                                     |
| Fixed CSP/framing/permissions policies constrain authored HTML and embedding                                                                | `renderer/src/lib/security-headers.json`; Static Assets and Worker responses                        | External scripts, inline scripts, many iframe/media/form uses and embedding the blog are restricted. Browser tests show that CFGB's UI works under this policy; they do not establish user approval of restrictions on authored content.                          |

The optional schemas also retain exact slug/topic/alias/image-path patterns,
nonempty fields and other authoring conventions. They are no longer YAML loading
gates. Adding enforcement to `build` or a new command still requires approval;
optional schema files are not an authorization source.

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
