# Configuration and schemas

`cfgb.yaml` is the only site configuration read by the CLI. JSON Schema 2020-12
files in `schemas/` specify structural validation; semantic rules here and in
the content contract also apply. Unknown fields fail, except inside explicitly
open topic/locale maps. Require format assertion for `date-time` and `uri`.

| Field | Requirement / default |
| --- | --- |
| `schemaVersion` | Required, integer 1 |
| `site.title` | Required nonempty text |
| `site.baseUrl` | Required HTTPS origin, no path/query/fragment/trailing slash/userinfo |
| `site.defaultLocale` | Required key in `locales` |
| `site.timezone` | Required IANA timezone; archives/dates use it |
| `locales` | Nonempty map; v1 supports `ja` and `en`, each with a `label` |
| `content.root` | Default `src/content` |
| `content.topics` | Default `src/data/topics.yaml` |
| `content.linkcards` | Default `src/data/linkcards` |
| `home.latestPosts` | Default 5, integer 1–50 |
| `search.provider` | Default/only value `pagefind` |
| `ai.enabled` | Default false |
| `ai.gateway` | Default/only v1 integration `cloudflare` |
| `ai.summary` | Map from enabled locales to `{provider,model}`; required when AI enabled |
| `security.previewAccess` | Only allowed value true; omission means true; preview release gate, not policy provisioning |
| `hatena.blogs` | Optional array `{url,locale}`; normalized unique origins |

AI disabled with `summary: {}` is a usable offline configuration. Do not insert
`TBD` as if it were a real provider/model. Missing model configuration is an
error only when an AI command is requested or `ai.enabled` is true. No model is
selected by these specifications. Provider IDs and model IDs are adapter input;
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
`WORKERS_CI_*` provenance variables remain platform-managed. These settings
belong to the build environment, not site YAML or runtime Worker variables.
See the [build runtime contract](10-build-runtime.md). `.env.example` contains
names only.

CFGB applies defaults in its parser; JSON Schema default annotations do not
populate missing configuration. In v1, `security.previewAccess: false` is an
error. Omission of the field or parent section still requires private previews.
The setting is an assertion, not automatic creation of Access policies. Default
protection uses Worker-level previews-only `preview_worker` Access with verified
Worker identity and policy; hostname-specific coverage is an advanced option.

Relative paths must stay inside the repository after symlink resolution. Topic
IDs match `[a-z0-9]+(-[a-z0-9]+)*`. Every topic must have exactly the configured
locale labels, even when it currently has articles in only one locale.
All article topics must be registered. AI may suggest existing topics, never
create new ones. `site.defaultLocale` must exist; variant locales must be enabled.

The [personal configuration example in cfgb-example](https://github.com/ymmt2005/cfgb-example/blob/main/examples/ymmt2005.dev.yaml)
illustrates different values.
It is a complete alternate config, not an implicit merge mechanism. To use it,
copy it to the personal repository root as `cfgb.yaml`; relative paths resolve
from that root. The example repository retains a reserved non-routable origin.

## Schema inventory

- `cfgb.schema.json`: site configuration.
- `article.schema.json`: article frontmatter structure; omitted/empty summary is structurally valid, with mode-specific semantic requirements.
- `topics.schema.json`: localized topic master.
- `sidecar.schema.json`: AI and import provenance per variant.
- `linkcard.schema.json`: committed fetch metadata.
- `migration-manifest.schema.json`: paired source/target identity for successful applications only; no conflict status.
- `migration-conflicts.schema.json`: separate conflict observations/proposals, never ownership records.

Schema versions are not CLI release numbers. Unsupported newer versions fail
with a clear upgrade diagnostic. Breaking upgrades are explicit migrations with
a dry-run diff. Renderer and CLI must run the same conformance fixtures, including
duplicate YAML key rejection; schema validation alone is insufficient.
