# Primary-source references

Reviewed on 2026-10-03. Specifications above are CFGB design choices. These sources
support platform integration points; exact dependency versions must be verified
when implementation begins and pinned in that repository.

- [Astro Markdown](https://docs.astro.build/en/guides/markdown-content/): Markdown pipeline and heading information.
- [Astro sitemap integration](https://docs.astro.build/en/guides/integrations-guide/sitemap/): index plus numbered files, filtering, serialization and entry limits; CFGB supplies actual translation-group links.
- [Expressive Code installation](https://expressive-code.com/installation/): Astro integration; code rendering belongs to the renderer owned by CFGB.
- [VS Code Markdown](https://code.visualstudio.com/docs/languages/markdown): paste/drop destination configuration. Cursor compatibility requires a manual check.
- [Pagefind multilingual search](https://pagefind.app/docs/multilingual/): language-specific indexes and Extended support for Japanese.
- [Pagefind API](https://pagefind.app/docs/api/): real result-data retrieval for search acceptance.
- [Workers Builds configuration](https://developers.cloudflare.com/workers/ci-cd/builds/configuration/): separate Build, Deploy and Preview commands, official `WORKERS_CI_*` variables (which can be overridden).
- [Workers Builds event subscriptions](https://developers.cloudflare.com/workers/ci-cd/builds/event-subscriptions/): examples use prefixed build identifiers; CFGB stores those values as an opaque diagnostic `buildUUID` and does not use them as workspace paths.
- [Workers Builds image](https://developers.cloudflare.com/workers/ci-cd/builds/build-image/): `NODE_VERSION`, `PNPM_VERSION`, dependency-install override and runner platform.
- [Workers Builds branches](https://developers.cloudflare.com/workers/ci-cd/builds/build-branches/): production versus Preview builds and current preview command.
- [Worker Previews](https://developers.cloudflare.com/workers/previews/): Wrangler 4.135.0+ minimum; branch/deployment URLs are public unless Access protects them.
- [Wrangler configuration](https://developers.cloudflare.com/workers/wrangler/configuration/): required top-level Worker compatibility date; independent `workers_dev` and `preview_urls`, including omitted-setting behavior.
- [Worker Version URLs](https://developers.cloudflare.com/workers/versions-and-deployments/version-urls/): `preview_urls` also enables Version URLs; include these in Access coverage.
- [Worker Preview configuration](https://developers.cloudflare.com/workers/previews/configuration/): top-level assets/runtime settings and explicit empty `previews` for Workers without Preview-specific bindings.
- [Workers Access](https://developers.cloudflare.com/workers/configuration/cloudflare-access/): Worker-level previews-only `preview_worker` destinations and hostname-specific alternatives.
- [Wrangler Worker commands](https://developers.cloudflare.com/workers/wrangler/commands/workers/): explicit Preview `--name` rather than temporary-checkout branch inference.
- [Static Assets Worker routing](https://developers.cloudflare.com/workers/static-assets/routing/worker-script/): selective Worker-first routing and ASSETS binding.
- [Static site generation routing](https://developers.cloudflare.com/workers/static-assets/routing/static-site-generation/): hierarchical `404-page` fallback and 404 status.
- [Static Assets headers](https://developers.cloudflare.com/workers/static-assets/headers/): static `_headers`; Worker responses need their own headers.
- [Static Assets redirects](https://developers.cloudflare.com/workers/static-assets/redirects/): `_redirects` for same-site aliases.
- [GitHub immutable releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases): locked release assets/tags, release attestations and publishing all assets before final publication.
- [GitHub release integrity verification](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/secure-your-dependencies/verify-release-integrity): immutable-release and local-asset verification.
- [GitHub CLI release verification](https://cli.github.com/manual/gh_release_verify) and [asset verification](https://cli.github.com/manual/gh_release_verify-asset): exact-tag signed-attestation checks with explicit repository selection.
- [GitHub Marketplace publication](https://docs.github.com/en/actions/how-tos/create-and-publish-actions/publish-in-github-marketplace): root Action metadata and an Action-specific public repository.
- [GitHub workflow syntax](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax): step-level Action references and commit pins.
- [Reusing workflow configurations](https://docs.github.com/en/actions/concepts/workflows-and-actions/reusing-workflow-configurations): Action steps versus job-level reusable workflows.
- [GitHub token behavior](https://docs.github.com/en/actions/concepts/security/github_token): do not assume token-generated activity reruns checks without approval.
- [AI Gateway compatibility API](https://developers.cloudflare.com/ai-gateway/usage/chat-completion/): endpoint and authentication modes; not a recommendation of any listed model.
- [Hatena AtomPub](https://developer.hatena.ne.jp/ja/documents/blog/apis/atom/): original content types, formatted HTML, pagination and remote editing API.

No current price, model quality, hosting quota or cross-version compatibility is
assumed by the corpus. This repository intentionally does not include a pretend
working deployment configuration with unverified account-specific values.
