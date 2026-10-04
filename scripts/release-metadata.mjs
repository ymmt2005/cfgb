import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { readVersion } from "./release-version.mjs";

const pkg = JSON.parse(readFileSync("renderer/package.json", "utf8"));
const build = readFileSync("internal/build/build.go", "utf8");
const worker = readFileSync("internal/worker/worker.go", "utf8");
function constant(source, name) {
  const match = source.match(new RegExp(`\\b${name}\\s*=\\s*"([^"]+)"`));
  if (!match) throw new Error(`Missing release constant ${name}`);
  return match[1];
}
mkdirSync(".release", { recursive: true });
writeFileSync(
  ".release/toolchain-requirements.json",
  JSON.stringify(
    {
      cfgbVersion: readVersion(),
      nodeRange: pkg.engines.node,
      npmMinimum: constant(build, "minimumNpmVersion"),
      pnpmMinimum: constant(build, "minimumPnpmVersion"),
      testedNodeVersion: constant(build, "testedNodeVersion"),
      testedNpmVersion: constant(build, "testedNpmVersion"),
      testedPnpmVersion: constant(build, "testedPnpmVersion"),
      rendererVersion: pkg.version,
      wranglerVersion: pkg.dependencies.wrangler,
      workerCompatibilityDate: constant(worker, "CompatibilityDate"),
    },
    null,
    2,
  ) + "\n",
);
