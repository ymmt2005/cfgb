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
| `security.previewAccess` | Default true; release gate, not an access-policy provisioning switch |
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
by the migration contract. `.env.example` contains names only.

Relative paths must stay inside the repository after symlink resolution. Topic
IDs match `[a-z0-9]+(-[a-z0-9]+)*`. Every topic must have exactly the configured
locale labels, even when it currently has articles in only one locale.
All article topics must be registered. AI may suggest existing topics, never
create new ones. `site.defaultLocale` must exist; variant locales must be enabled.

Production overlay `examples/ymmt2005.dev.yaml` illustrates different values.
It is a complete alternate config, not an implicit merge mechanism. To use it,
copy it to the personal repository root as `cfgb.yaml`; relative paths resolve
from that root. The example repository retains a reserved non-routable origin.

## Schema inventory

- `cfgb.schema.json`: site configuration.
- `article.schema.json`: publication frontmatter; authoring permits omitted/empty summary.
- `topics.schema.json`: localized topic master.
- `sidecar.schema.json`: AI and import provenance per variant.
- `linkcard.schema.json`: committed fetch metadata.
- `migration-manifest.schema.json`: source/target identity and restart state.

Schema versions are not CLI release numbers. Unsupported newer versions fail
with a clear upgrade diagnostic. Breaking upgrades are explicit migrations with
a dry-run diff. Renderer and CLI must run the same conformance fixtures, including
duplicate YAML key rejection; schema validation alone is insufficient.
