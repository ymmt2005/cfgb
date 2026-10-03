# Embedded renderer toolchain

This directory is owned by the CFGB executable. Content repositories do not copy it.

`package-lock.json` is the default lockfile and pins the Astro 6 baseline and the other build tools. `pnpm-lock.yaml` remains for `CFGB_PACKAGE_MANAGER=pnpm`. Astro 7 is published and is intentionally not used.

`toolchain-requirements.json` at the repository root is the release pin, including the SHA-256 of `package-lock.json` and of `pnpm-lock.yaml`.
