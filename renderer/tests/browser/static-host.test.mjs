import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { startSite, withPage } from "./fixture.mjs";

describe("static hosting below a repository path", { timeout: 180_000 }, () => {
  let site;
  before(async () => {
    site = await startSite({ basePath: "/cfgb-example", static: true });
  });
  after(async () => {
    await site?.close();
  });

  test("entry, navigation, translations and aliases work without a Worker or JavaScript", async () => {
    await withPage(site, { javaScriptEnabled: false }, async (page) => {
      await page.goto(`${site.origin}/cfgb-example/`);
      await page.waitForURL("**/cfgb-example/ja/");
      await page.getByRole("link", { name: "English", exact: true }).click();
      await page.waitForURL("**/cfgb-example/en/");
      await page
        .getByRole("link", { name: "Browser interactions", exact: true })
        .click();
      await page.waitForURL("**/cfgb-example/en/posts/browser/");
      assert.equal(
        await page.locator('link[rel="canonical"]').getAttribute("href"),
        "https://example.invalid/cfgb-example/en/posts/browser/",
      );
      await page
        .getByRole("link", { name: "Open translation", exact: true })
        .click();
      await page.waitForURL("**/cfgb-example/ja/posts/browser/");
      await page.goto(`${site.origin}/cfgb-example/en/posts/old-browser/`);
      await page.waitForURL("**/cfgb-example/en/posts/browser/");
      assert.ok(
        (
          await page.locator("#responsive-density").getAttribute("src")
        ).startsWith("/cfgb-example/media/"),
      );
    });
  });

  test("search and interactive resources stay under the hosting prefix", async () => {
    await withPage(site, {}, async (page) => {
      await page.goto(`${site.origin}/cfgb-example/en/search/`);
      await page.getByRole("textbox").fill("Browserneedle");
      const result = page
        .locator(".pagefind-ui__result-link")
        .filter({ hasText: "Browser interactions" });
      await result.waitFor();
      assert.ok(
        (await result.getAttribute("href")).startsWith(
          "/cfgb-example/en/posts/browser/",
        ),
      );
      await result.click();
      await page.waitForURL("**/cfgb-example/en/posts/browser/**");
      await page.locator("pre.mermaid svg").waitFor();
      await page.getByRole("button", { name: "Theme", exact: true }).click();
      await page.getByRole("option", { name: "Dark", exact: true }).click();
      assert.equal(
        await page.locator("html").getAttribute("data-theme"),
        "dark",
      );
      assert.ok(
        await page
          .locator("#responsive-density")
          .evaluate((image) => image.complete && image.naturalWidth > 0),
      );
    });
  });

  test("feeds and sitemap metadata carry the prefix once", async () => {
    const request = await site.browser.newContext();
    try {
      for (const file of [
        "en/feed.xml",
        "sitemap-index.xml",
        "sitemap-0.xml",
        "robots.txt",
      ]) {
        const response = await request.request.get(
          `${site.origin}/cfgb-example/${file}`,
        );
        assert.equal(response.status(), 200);
        const text = await response.text();
        assert.ok(text.includes("https://example.invalid/cfgb-example/"));
        assert.equal(text.includes("/cfgb-example/cfgb-example/"), false);
        assert.equal(text.includes("https://example.invalid/en/"), false);
      }
    } finally {
      await request.close();
    }
  });
});
