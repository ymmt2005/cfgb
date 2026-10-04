// Adapt CFGB's static/Worker routes to lychee. Link extraction and validation
// belong to lychee; this script only supplies the generated routing rules.
import { spawnSync } from "node:child_process";
import { readFileSync, statSync } from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

const [site, origin, ...options] = process.argv.slice(2);
if (!site || !origin) {
  console.error(
    "Usage: node scripts/check-links.mjs SITE ORIGIN [--online] [--output FILE] [--config FILE]",
  );
  process.exit(2);
}
try {
  const root = path.resolve(site);
  if (!statSync(root).isDirectory())
    throw new Error("SITE must be a directory");
  const base = new URL(origin);
  if (
    !["http:", "https:"].includes(base.protocol) ||
    base.pathname !== "/" ||
    base.search ||
    base.hash ||
    base.username ||
    base.password
  ) {
    throw new Error(
      "ORIGIN must be an HTTP(S) origin without a path, query, or credentials",
    );
  }
  const localRoot = pathToFileURL(root).href;
  const local = (route) => new URL(`.${route}`, `${localRoot}/`).href;
  const escape = (value) => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const args = ["--no-progress", "--root-dir", root];
  let online = false;
  for (let i = 0; i < options.length; i++) {
    if (options[i] === "--online") online = true;
    else if (["--output", "--config"].includes(options[i]) && options[i + 1])
      args.push(options[i], options[++i]);
    else throw new Error(`Unknown or incomplete option: ${options[i]}`);
  }
  // Alias remaps come first: lychee applies only the first matching remap.
  // Preserve query/fragment suffixes and check them at the canonical target.
  for (const line of readFileSync(path.join(root, "_redirects"), "utf8").split(
    /\r?\n/,
  )) {
    if (!line.trim() || line.trimStart().startsWith("#")) continue;
    const fields = line.trim().split(/\s+/);
    if (
      fields.length !== 3 ||
      fields[2] !== "301" ||
      !fields
        .slice(0, 2)
        .every((route) => route.startsWith("/") && !route.startsWith("//"))
    ) {
      throw new Error(`Unsupported generated redirect: ${line}`);
    }
    const [source, target] = fields;
    for (const url of [local(source), new URL(source, base).href]) {
      // Local file URLs may lose a trailing directory slash during lychee's
      // normalization. HTTP alias URLs retain the exact generated spelling.
      const pattern =
        url.startsWith("file:") && url.endsWith("/")
          ? `${escape(url.slice(0, -1))}/?`
          : escape(url);
      args.push("--remap", `^${pattern}([?#]|$) ${local(target)}$1`);
    }
  }
  args.push("--remap", `^${escape(base.origin)}([/?#]|$) ${localRoot}$1`);
  // These routes are served by the Worker, not emitted as static HTML. All
  // other root-relative and same-origin URLs still require real output files.
  args.push("--exclude", `^${escape(localRoot)}/?([?#].*)?$`);
  args.push("--exclude", `^${escape(local("/__locale"))}([?#].*)?$`);
  args.push(
    online ? "--cache" : "--offline",
    "--format",
    "markdown",
    path.join(root, "**", "*.html"),
  );
  const result = spawnSync("lychee", args, { stdio: "inherit" });
  if (result.error) throw result.error;
  process.exit(result.status ?? 1);
} catch (error) {
  console.error(error.message);
  process.exit(2);
}
