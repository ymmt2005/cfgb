# Embedded renderer toolchain

This directory is owned by the CFGB executable. Content repositories do not copy it.

`package-lock.json` is the default lockfile and pins Astro 7.3.5 and the other build tools. npm >= 12 is required. Its `allowScripts` field permits install scripts for `esbuild` and `workerd`; other dependency install scripts stay blocked. sharp 0.35 has no install script. `pnpm-lock.yaml` and `pnpm-workspace.yaml` remain for `CFGB_PACKAGE_MANAGER=pnpm`, which requires pnpm >= 11. `allowBuilds` permits install scripts for `esbuild` and `workerd`, and denies `fsevents`.

`toolchain-requirements.json` at the repository root is the release pin, including the SHA-256 of `package-lock.json` and of `pnpm-lock.yaml`.
