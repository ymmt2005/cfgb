# Configuration and schemas

`cfgb.yaml` is the only site configuration read by the CLI. Decode it directly
into the complete Go configuration struct with `goccy/go-yaml`, rejecting unknown
fields. The YAML decoder determines which document/tag syntax can be decoded;
configuration loading adds no AST syntax policy or JSON Schema validation gate.
`schemas/cfgb.schema.json` remains an optional editor/standalone validation aid,
not a prerequisite for using the CLI. Optional standalone JSON Schema 2020-12
validation asserts `date-time` and `uri` formats; the CLI does not require that
validation before consuming typed values.

The table describes site settings, defaults and the requirements of commands
that consume them. Decoding does not itself enforce every publishing/provider
constraint. Keep those domain checks in their relevant commands.

| Field | Requirement / default |
| --- | --- |
| `schemaVersion` | Required, integer 1 |
| `site.title` | Required nonempty text |
| `site.image` | Optional local branding image, resolved relative to `cfgb.yaml`; empty or omitted uses text-only OG cards and generates no favicons |
| `site.baseUrl` | Public site URL; may include a hosting path such as `/cfgb-example/`. A trailing slash is optional. No query/fragment/userinfo |
| `site.defaultLocale` | Required key in `locales` |
| `site.timezone` | IANA timezone used only for monthly archive classification |
| `locales` | Nonempty map of supported content/UI language identifiers. This release supports exact, case-sensitive `ja` and `en`, each with a nonempty `label` |
| `content.root` | Default `src/content` |
| `content.topics` | Default `src/data/topics.yaml` |
| `content.linkcards` | Default `src/data/linkcards` |
| `home.latestPosts` | Default 5, integer 1–50 |
| `search.provider` | Default/only value `pagefind` |
| `deploy.productionBranch` | Default `main`; literal Git short branch name, e.g. `master` |
| `ai.enabled` | Default false |
| `ai.gateway` | Default/only v1 integration `cloudflare` |
| `ai.summary` | Map from enabled locales to `{provider,model}`; required when AI enabled |
| `security.previewAccess` | Only allowed value true; omission means true; preview release gate, not policy provisioning |
| `hatena.blogs` | Optional array `{url,locale}`; `locale` is a locale identifier; normalized unique origins |

The `locales` keys select translated content and UI from the release catalog,
not arbitrary operating-system locales. Identifier syntax alone does not imply
support: `EN`, `en-US`, `ja-JP`, `pt-BR`, and `fr` are not supported selections in
this release. Match catalog and configured identifiers case-sensitively, without
normalizing or aliasing them. Extending the catalog requires translated UI and
review of its presentation; a new language is not accepted merely because its
identifier can be parsed.

`site.image` supplies the shared branding mark for generated article OG cards
and favicons. For example, `image: src/assets/site.svg` under `site` selects a
local source image; the format is decoded by the installed Sharp pipeline.
The CLI captures its bytes through the repository root alongside the content
snapshot. Relative paths resolve from the selected configuration directory,
including when using `--config`. Contained repository symlinks remain usable.
The image is optional; an explicitly configured file that cannot be read or
decoded fails the build rather than silently disappearing.

Generated fallback OG cards include the article title, site title and, when
configured, this branding mark. Article `ogImage` takes precedence over the
fallback card. The site image also supplies `/favicon.png` and
`/apple-touch-icon.png`, linked with the hosting prefix. These square icons
contain the original image with transparent padding; they do not crop or
distort it. The original branding file is not automatically published.

AI disabled with `summary: {}` is a usable offline configuration. Do not insert
`TBD` as if it were a real provider/model. Missing model configuration is an
error only when an AI command is requested or `ai.enabled` is true. No model is
selected by these specifications. Although an empty `ai.summary` map is structurally
valid, `ai.enabled: true` requires a provider/model entry for every configured
locale in semantic validation (`E_PROVIDER_CONFIG`, exit 2). Provider IDs and
model IDs are adapter input;
unknown/unsupported IDs fail explicitly without fallback to another provider.

Secrets and Cloudflare account identifiers are environment configuration, not
committed YAML: `CFGB_CF_ACCOUNT_ID`, `CFGB_CF_GATEWAY_ID`, `CFGB_CF_AIG_TOKEN`,
and optional provider credentials as described in the AI contract. Never pass
secrets on the CLI or embed them in diagnostics. Hatena credentials are covered
by the migration contract. `CFGB_CF_WORKER_NAME`, `CLOUDFLARE_ACCOUNT_ID` and
`CLOUDFLARE_API_TOKEN` configure uploads; they are not visitor-runtime secrets.
`CFGB_CF_ACCESS_API_TOKEN` is the separate read-only Access inspection token for
preview preflight. `CFGB_VERSION`, `CFGB_SHA256`, `NODE_VERSION`, `PNPM_VERSION`
and `SKIP_DEPENDENCY_INSTALL` are trusted Workers Builds bootstrap settings;
`WORKERS_CI_*` variables remain platform-managed diagnostic metadata. These settings
belong to the build environment, not site YAML or runtime Worker variables.
See the [build runtime contract](10-build-runtime.md). `.env.example` contains
names only.

CFGB seeds the Go struct with defaults before decoding; JSON Schema default
annotations do not populate configuration. Explicit values, including zero,
false and empty strings, are preserved. Hatena blogs are typed Go entries, not
AST nodes or opaque YAML. Build calls the separate `ValidateSite` check for
supported languages and IANA timezones using the CLI's bundled database
(`Local` is not a site timezone). Decode/build-setting failures exit 2 before
removing existing output. Provider/deployment validation belongs to the commands that
use those settings. Omitted preview protection defaults to true; an explicit
false remains false and must fail the private-preview gate when preview is
requested.

The setting is an assertion, not automatic creation of Access policies. Default
protection uses Worker-level previews-only `preview_worker` Access with verified
Worker identity and policy; hostname-specific coverage is an advanced option.

`cfgb.yaml` is read through an `os.Root` opened at the repository. A symlink that
stays inside the repository is followed. A symlink that leaves the repository is
rejected. Relative paths must stay inside the repository after symlink resolution.

`site.timezone` is used only to classify articles into monthly archives. Build
uses Go's native timezone interpretation with bundled tzdata; an omitted/empty
name resolves to UTC. Go supplies each article's archive year/month separately
and normalizes publication/update timestamps to UTC for renderer metadata.
The renderer and browser do not interpret the configured site timezone.
Visible dates start as a static UTC fallback and JavaScript formats the UTC
instant in the reader's browser timezone, retaining the article language's date
format. Machine timestamps, chronological ordering and archive links do not
change with the reader's timezone. `new` also uses UTC for timestamps/group names.

Supported content languages are exact, case-sensitive keys in
`renderer/src/lib/locales.json`, shared by the CLI and renderer. This release's
catalog contains `ja`, `en`, `zh-Hans` (Simplified Chinese) and `ko` (Korean).
There is no separate runtime language-tag syntax
check: `pt-BR`, `EN`, `en-US`, and any other key absent from the catalog are
unsupported. The optional identifier schema is editor guidance, not a build gate.
UI copy, date presentation, and OpenGraph locale metadata live in that catalog.
Build also currently rejects an empty locale map, a blank or whitespace-only
label, and a default language absent from the configured keys; those remaining
required-value policies are listed separately in the input-policy audit.
Checks happen after decoding and before output removal or rendering. The renderer
does not treat an unknown language as English. Adding a language means extending
the catalog and providing its UI translations and date presentation. The header
lists every configured language using its configured label, with the current
language marked. Configuration keys remain exact; HTTP language negotiation is
case-insensitive and can match regional browser preferences by language/script
(for example `zh-CN` to `zh-Hans`, and `ko-KR` to `ko`). Traditional Chinese
preferences do not match the Simplified Chinese language.

Topic IDs match `[a-z0-9]+(-[a-z0-9]+)*`. Every topic must have exactly the configured
locale labels, even when it currently has articles in only one locale. JSON Schema
allows locale-label keys that match the identifier pattern, including a partial
or empty map; semantic validation compares each map
with the configured locales. An empty map or missing configured label is
`E_TOPIC`, exit 1, in every validation mode.
All article topics must be registered. AI may suggest existing topics, never
create new ones. `site.defaultLocale` must exist; variant locales must be enabled.

`deploy.productionBranch` is optional, including its parent section; the parser
applies `main` when omitted. Treat its value as the literal Git short branch
name reported by the current checkout. Do not expand full refs, symbolic names,
revision expressions, Git shorthand or branch patterns, or accept per-command
branch overrides. Production deployment compares that literal value with the
current branch; a missing or differing branch fails the production gate.
Deploy requires the current invocation's branch to equal the configured value;
Preview rejects that value. A valid branch that fails either command rule is
`E_DEPLOY_TARGET`, exit 1. The branch recorded in an artifact is diagnostic.
Changing this setting applies on the next invocation.
Keep the Workers Builds production-branch setting and protected Git branch
aligned with this value before enabling automatic builds. Review branch-setting
changes under the existing trusted configuration/push policy; changing the name
does not relax that trust boundary.

The [personal configuration example in cfgb-example](https://github.com/ymmt2005/cfgb-example/blob/main/examples/ymmt2005.dev.yaml)
illustrates different values.
It is a complete alternate config, not an implicit merge mechanism. To use it,
copy it to the personal repository root as `cfgb.yaml`; relative paths resolve
from that root. The example repository retains a reserved non-routable origin.

## Schema inventory

- `cfgb.schema.json`: optional standalone/editor validation of site configuration; not applied during configuration loading.
- `article.schema.json`: optional standalone/editor article guidance; not a front-matter loading gate. Omitted/empty summary is allowed; mode-specific semantic checks are separate.
- `topics.schema.json`: localized topic master.
- `sidecar.schema.json`: AI and import provenance per variant.
- `linkcard.schema.json`: committed fetch metadata.
- `migration-manifest.schema.json`: paired source/target identity for successful applications only; no conflict status.
- `migration-conflicts.schema.json`: separate conflict observations/proposals, never ownership records.

Schema versions are not CLI release numbers. Commands that consume versioned
content reject unsupported versions with a clear upgrade diagnostic; the
configuration decoder preserves the supplied `schemaVersion` without a Schema
gate. Breaking upgrades are explicit migrations with
a dry-run diff. Decoder fixtures exercise the YAML library's behavior, including
its duplicate-key errors; do not add an independent YAML syntax policy. Optional
Schema scenarios are separate from loading/build acceptance.

Source configuration, topics and Markdown must be valid UTF-8. Detect and remove
a UTF-8 BOM only at the start of a file; preserve interior U+FEFF and line endings.
Reject malformed UTF-8 before YAML decoding or JSON serialization, with source
file context, rather than silently replacing bytes with U+FFFD. YAML still decodes
directly into Go structs/maps; this encoding check adds no YAML tag/document policy.
