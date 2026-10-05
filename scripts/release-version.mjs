import { readFileSync } from "node:fs";
import { join } from "node:path";

export const versionPath = "internal/version/version.go";

export function sourceVersion(source) {
  const match = source.match(/^var Version(?: string)?\s*=\s*"([^"\n]+)"/m);
  if (!match) throw new Error(`Missing Version string in ${versionPath}`);
  return match[1];
}

export function readVersion(cwd = process.cwd()) {
  return sourceVersion(readFileSync(join(cwd, versionPath), "utf8"));
}
