import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const script = fileURLToPath(new URL("../check-links.mjs", import.meta.url));
const config = fileURLToPath(
  new URL("../../.github/lychee.toml", import.meta.url),
);

test("lychee validates static files, fragments, canonical URLs, and redirect targets", () => {
  const site = mkdtempSync(path.join(tmpdir(), "cfgb-links-"));
  try {
    mkdirSync(path.join(site, "en", "posts", "new"), { recursive: true });
    writeFileSync(
      path.join(site, "_redirects"),
      "/en/old/ /en/posts/new/ 301\n",
    );
    writeFileSync(
      path.join(site, "picture.svg"),
      '<svg xmlns="http://www.w3.org/2000/svg"/>',
    );
    writeFileSync(
      path.join(site, "en", "posts", "new", "index.html"),
      '<h2 id="section">Section</h2>',
    );
    const check = (body) => {
      writeFileSync(path.join(site, "en", "index.html"), body);
      return spawnSync(
        process.execPath,
        [script, site, "https://example.invalid", "--config", config],
        { encoding: "utf8" },
      );
    };
    const valid = check(`<a href="/en/posts/new/#section">Relative</a>
      <a href="https://example.invalid/en/posts/new/#section">Canonical</a>
      <a href="/en/old/#section">Alias</a>
      <a href="https://example.invalid/en/old/#section">Absolute alias</a>
      <img src="/picture.svg" alt="Picture">
      <a href="/">Locale redirect</a><a href="https://example.invalid/">Absolute root</a>
      <a href="/__locale?lang=en&amp;next=%2Fen%2F">Locale choice</a>
      <a href="https://external.invalid/missing">Skipped in offline mode</a>`);
    assert.equal(valid.status, 0, valid.stdout + valid.stderr);
    for (const body of [
      '<a href="/missing/">Missing page</a>',
      '<img src="/missing.svg" alt="Missing">',
      '<img src="/picture.svg" srcset="/missing.svg 2x" alt="Missing variant">',
      '<a href="/en/posts/new/#missing">Missing fragment</a>',
      '<a href="https://example.invalid/en/posts/new/#missing">Missing canonical fragment</a>',
      '<a href="/en/old/#missing">Missing alias fragment</a>',
      '<a href="https://example.invalid/en/old/#missing">Missing absolute alias fragment</a>',
      '<a href="/__locale-typo?lang=en">Misspelled endpoint</a>',
    ]) {
      const result = check(body);
      assert.notEqual(result.status, 0, body);
      assert.match(result.stdout + result.stderr, /missing|__locale-typo/);
    }
    // A directory alone must not pass when its index page has disappeared.
    rmSync(path.join(site, "en", "posts", "new", "index.html"));
    for (const href of [
      "/en/posts/new/",
      "/en/old/",
      "https://example.invalid/en/old/",
    ]) {
      const result = check(`<a href="${href}">Missing target</a>`);
      assert.notEqual(result.status, 0, href);
    }
  } finally {
    rmSync(site, { recursive: true, force: true });
  }
});
