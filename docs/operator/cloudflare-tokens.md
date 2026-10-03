# Cloudflare tokens for CFGB

Documentation-confirmed recipe as of 2026-10-03. No token has been presented to this repository, and none of these profiles has been live-tested against a Worker. Token verification (`/client/v4/user/tokens/verify`) only proves that a token is active. It does not prove deploy or Access access.

The visual mockup and ordinary local tests need none of these credentials.

Create a **user API token** at [My Profile > API Tokens](https://dash.cloudflare.com/profile/api-tokens) with **Create Custom Token**. Workers Builds does not accept account-owned tokens yet. The token Workers Builds generates automatically includes KV and R2 edit plus access to every zone; do not adopt that template. Review the custom token before saving it.

Scope account permissions to the selected account and zone permissions to the site zone. Store the value in the consumer’s secret store. Do not put it in Git, an issue, a pull request, `cfgb.yaml`, or a diagnostic log.

## What is needed now

Nothing is required to review the mockup or to run local tests. The first live check needs the deploy token below, and only after a development Worker exists.

| Secret | Dashboard permissions | Where it goes | When |
| --- | --- | --- | --- |
| `CLOUDFLARE_API_TOKEN` | Account → Workers Scripts → Edit. Also Account → Account Settings → Read while that is still required by the pinned Wrangler. Add Account → Workers Admin only to create a Worker that does not exist yet. Add Zone → Workers Routes → Write only when attaching or changing a route or custom domain. | Cursor environment, or the Worker’s **Settings → Builds → API token** (supplied to the build as `CLOUDFLARE_API_TOKEN`) | First development deploy, then Workers Builds |
| `CLOUDFLARE_ACCOUNT_ID` | Not a token. Account ID. | Same consumers, as a variable | With the deploy token |
| `CFGB_CF_WORKER_NAME` | Not a token. Existing Worker name. | Same consumers, as a variable | With the deploy token |
| `CFGB_CF_ACCESS_API_TOKEN` | Account → Access: Apps and Policies → Read. Add Access: Groups → Read only if a policy refers to a group that must be inspected. | Workers Builds build-time secret, or the Cursor environment for a preview check | Before the first private preview upload |
| `CFGB_CF_AIG_TOKEN` | Account → AI Gateway → Run. No Gateway management permission. The Run grant is account-wide. | Authoring or evaluation job only. Not the deploy build. | When summary generation is enabled |
| `CFGB_CF_ACCOUNT_ID`, `CFGB_CF_GATEWAY_ID` | Not tokens. | Same authoring job | With the Gateway token |

`cfgb.yaml` does not load these values. The CLI reads the environment.

Conditional notes:

- Deploying a new version of an existing Worker needs Workers Scripts Edit. Creating the Worker needs Workers Admin. After a custom domain or route exists, later deploys that do not change it can stay on Edit.
- Custom Domains do not currently honor per-Worker roles. A per-Worker token is not a promise that every domain call will succeed.
- Current Workers documentation says granular Wrangler permissions authenticate with an account-owned token, while Workers Builds still requires a user token and still accepts the legacy Workers Scripts label. Confirm the pinned Wrangler (`4.147.0`) against the user token before treating this profile as minimal.
- An Access API token does not open the preview site. Browser or harness checks need a separate Access service token (`CF-Access-Client-Id` and `CF-Access-Client-Secret`) accepted by a reviewed Service Auth policy. Those headers are harness credentials, not CFGB CLI configuration.
- Do not grant KV, R2, D1, or Pages edit. v1 does not use them.

## Create a token

1. Identify the account, the development Worker, and the site zone.
2. Open **My Profile → API Tokens → Create Token → Create Custom Token**.
3. Name it for one consumer, for example `cfgb-cursor-dev-deploy`, `cfgb-ci-deploy`, or `cfgb-preview-inspect`.
4. Add only the permissions in the table row for that secret, plus a conditional permission from the notes above when that operation is actually in scope.
5. Limit the token to the specific account and, for zone permissions, the site zone. Set an expiry that matches the consumer. Workers Builds cannot use an IP restriction that the build network cannot meet.
6. Create the token and store the value once. Cursor receives it in the agent environment. GitHub Actions receives it as a repository or environment secret, injected only into the command that needs it. Workers Builds receives the deploy token in **Settings → Builds → API token** and the Access token as a build-time secret. Do not place any of them in runtime Worker variables.
7. The setup Action does not take a Cloudflare token.

For the Gateway token, an alternative is the selected Gateway’s **Settings → Create authentication token**, then enabling **Authenticated Gateway**. Keep the existing `cf-aig-authorization` header contract.

## Not yet verified

After a token exists, check activity with `/client/v4/user/tokens/verify`, then exercise the real call: pinned Wrangler deploy of the development Worker, Access application and policy reads for `preview_worker`, or one small Gateway inference. Record the result back in this file. Until that run, this page is a creation recipe, not a measured permission set.
