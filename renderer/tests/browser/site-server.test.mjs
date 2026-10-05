import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "node:test";
import { serveBuiltSite } from "./site-server.mjs";
import { withPage } from "./fixture.mjs";

test("CSP failures remain visible after navigation to a clean page", async () => {
  const root = mkdtempSync(path.join(tmpdir(), "cfgb-browser-headers-"));
  let site;
  try {
    writeFileSync(path.join(root, "_headers"), "/*\n  Content-Security-Policy: script-src 'self'\n");
    writeFileSync(path.join(root, "blocked.html"), '<!doctype html><title>Blocked</title><script>window.inlineScriptRan = true;</script>');
    writeFileSync(path.join(root, "clean.html"), "<!doctype html><title>Clean</title>");
    site = await serveBuiltSite(root);
    await assert.rejects(withPage(site, {}, async (page) => {
      await page.goto(`${site.origin}/blocked.html`);
      await page.waitForFunction(() => window.cspViolations.length > 0);
      assert.equal(await page.evaluate(() => window.inlineScriptRan), undefined);
      await page.goto(`${site.origin}/clean.html`);
      assert.equal(await page.title(), "Clean");
    }), /generated CSP violations/);
  } finally {
    try { await site?.close(); }
    finally { rmSync(root, { recursive: true, force: true }); }
  }
});
