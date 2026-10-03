# Embedded renderer toolchain

This directory is owned by the CFGB executable. Content repositories do not copy it.

`package-lock.json` is the default lockfile and pins the Astro 6 baseline and the other build tools. npm >= 12 is required. Its `allowScripts` field permits install scripts for `esbuild`, `sharp`, and `workerd`; other dependency install scripts stay blocked. `pnpm-lock.yaml` and `pnpm-workspace.yaml` remain for `CFGB_PACKAGE_MANAGER=pnpm`, which requires pnpm >= 11. `allowBuilds` permits install scripts for `esbuild`, `sharp`, and `workerd`. Astro 7 is published and is intentionally not used.

`toolchain-requirements.json` at the repository root is the release pin, including the SHA-256 of `package-lock.json` and of `pnpm-lock.yaml`.
