import { appendFileSync } from "node:fs";
import { execFileSync, spawnSync } from "node:child_process";
import { pathToFileURL } from "node:url";
import { readVersion, sourceVersion, versionPath } from "./release-version.mjs";

export async function lookupRelease(tag, fetchRelease = fetch) {
  const repository = process.env.GITHUB_REPOSITORY || "ymmt2005/cfgb";
  const response = await fetchRelease(
    `https://api.github.com/repos/${repository}/releases/tags/${tag}`,
    {
      headers: {
        Accept: "application/vnd.github+json",
        Authorization: `Bearer ${process.env.GH_TOKEN}`,
      },
    },
  );
  if (response.status === 404) return null;
  if (!response.ok)
    throw new Error(`Release lookup failed: ${response.status}`);
  return response.json();
}

// Like pbschema-lens, compare HEAD with its parent: main uses squash merges.
export async function detectRelease(
  cwd = process.cwd(),
  lookup = lookupRelease,
) {
  const tag = readVersion(cwd);
  const skip = { tag, release: false, verify: false };
  if (!/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(tag)) {
    console.log(`Skip: Version is not an exact release tag (${tag})`);
    return skip;
  }
  const parent = spawnSync(
    "git",
    ["rev-parse", "--verify", "--quiet", "HEAD^"],
    {
      cwd,
      encoding: "utf8",
    },
  );
  if (parent.error) throw parent.error;
  if (parent.status === 0) {
    const previous = sourceVersion(
      execFileSync("git", ["show", `HEAD^:${versionPath}`], {
        cwd,
        encoding: "utf8",
      }),
    );
    if (previous === tag) {
      console.log(`Skip: Version unchanged (${tag})`);
      return skip;
    }
    console.log(`Version bump ${previous} -> ${tag}`);
  } else if (parent.status !== 1) {
    throw new Error(`Cannot inspect parent commit: ${parent.stderr}`);
  }

  const existing = await lookup(tag);
  if (existing && !existing.draft) {
    if (!existing.immutable)
      throw new Error("Existing release is not immutable");
    console.log(`Skip: immutable release ${tag} already exists`);
    return { tag, release: false, verify: true };
  }
  return { tag, release: true, verify: true };
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const result = await detectRelease();
  for (const [key, value] of Object.entries(result)) {
    console.log(`${key}=${value}`);
    if (process.env.GITHUB_OUTPUT) {
      appendFileSync(process.env.GITHUB_OUTPUT, `${key}=${value}\n`);
    }
  }
}
