# Implementation progress

Baseline reviewed before this work:

| Repository | Commit |
| --- | --- |
| cfgb | `66daf2f26a80ee42232976f1c332afb100ce221d` |
| cfgb-example | `7f9f3553f259e86e1417d24f625eb97bbaefa025` |
| cfgb-action | `e8150e2b5bc47388ef3dbecd7768687a4780500c` |

## M0-A — visual mockup

Browsable static prototype in `design/mockup/`. Search results are labeled mock data. The owner has not reviewed it yet. The production renderer does not exist, so this direction is not yet copied into Astro.

## M0-B — token handoff

`docs/operator/cloudflare-tokens.md` is a documentation-confirmed creation recipe. It is not live-tested. No credential has been used.

## M0-C — foundation started

- Go module `github.com/ymmt2005/cfgb`, toolchain Go 1.27.1. `cfgb version` works. Other commands exit 2.
- Schemas, prompts, and `toolchain-requirements.json` are embedded.
- Renderer dependency pin is Astro `6.4.8` (Astro 7.3.5 is published and is not the v1 baseline), `@astrojs/sitemap` `3.7.4`, `astro-expressive-code` `0.44.2`, Mermaid `12.1.0`, Pagefind `1.5.2`, Wrangler `4.147.0`, pnpm `10.33.3`. pnpm 12.8.1 is published; this pin is the version that produced the lockfile here.
- `workerCompatibilityDate` is `2026-09-22`. It has not been deployed.
- Corpus tests are not wired yet. The example commit above is the pin to use.
- Node `22.14.0` is the tested runtime. `nodeRange` is `>=22.12.0 <23`, matching Astro 6.4.8.

## Not done

Renderer layouts, Worker, validation, build, and every later milestone. Live Cloudflare, model, and release gates remain open.
