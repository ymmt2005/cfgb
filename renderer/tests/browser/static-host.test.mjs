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
      await page.locator(".language-switch summary").click();
      await page
        .locator(".language-list")
        .getByRole("link", { name: "English", exact: true })
        .click();
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
        .locator(".translation")
        .getByRole("link", { name: "日本語", exact: true })
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

  test("language dropdown supports keyboard, dismissal and narrow dark layouts", async () => {
    await withPage(
      site,
      { viewport: { width: 375, height: 812 }, colorScheme: "dark" },
      async (page) => {
        await page.goto(`${site.origin}/cfgb-example/en/posts/browser/`);
        const selector = page.locator(".language-switch");
        const summary = selector.locator("summary");
        await summary.focus();
        await page.keyboard.press("Enter");
        assert.equal(await selector.getAttribute("open"), "");
        const links = selector.getByRole("link");
        assert.deepEqual(
          await links
            .allTextContents()
            .then((values) => values.map((value) => value.trim())),
          ["日本語", "English✓", "简体中文", "한국어"],
        );
        assert.equal(
          await selector
            .locator('[aria-current="true"]')
            .getAttribute("hreflang"),
          "en",
        );
        await page.keyboard.press("Tab");
        assert.equal(
          await links
            .first()
            .evaluate((link) => link === document.activeElement),
          true,
        );
        await page.keyboard.press("Escape");
        assert.equal(await selector.getAttribute("open"), null);
        assert.equal(
          await summary.evaluate((node) => node === document.activeElement),
          true,
        );
        await summary.click();
        const box = await selector.locator(".language-list").boundingBox();
        assert.ok(box.x >= 0 && box.x + box.width <= 375);
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth <= innerWidth,
          ),
          true,
        );
        await page.locator("h1").click();
        assert.equal(await selector.getAttribute("open"), null);
      },
    );
  });

  test("all article translations and missing-language home destinations work without JavaScript", async () => {
    await withPage(site, { javaScriptEnabled: false }, async (page) => {
      for (const language of ["zh-Hans", "ko"]) {
        await page.goto(`${site.origin}/cfgb-example/en/posts/browser/`);
        assert.equal(await page.locator(".translation a").count(), 3);
        await page.locator(`.translation a[hreflang="${language}"]`).click();
        await page.waitForURL(`**/cfgb-example/${language}/posts/browser/`);
        assert.equal(await page.locator("html").getAttribute("lang"), language);
        assert.equal(
          await page.locator('link[rel="canonical"]').getAttribute("href"),
          `https://example.invalid/cfgb-example/${language}/posts/browser/`,
        );
        assert.equal(
          await page.locator('link[rel="alternate"][hreflang]').count(),
          4,
        );
      }
      await page.goto(`${site.origin}/cfgb-example/ja/posts/partial-ja/`);
      assert.equal(
        await page
          .locator('.translation a[hreflang="ko"]')
          .getAttribute("href"),
        "/cfgb-example/ko/posts/partial-ko/",
      );
      assert.equal(
        await page
          .locator(".translation")
          .textContent()
          .then((text) => text.includes("ありません")),
        false,
      );
      await page.locator(".language-switch summary").click();
      const missing = page.locator('.language-list a[hreflang="en"]');
      assert.equal(await missing.getAttribute("href"), "/cfgb-example/en/");
      assert.equal(await missing.locator("small").textContent(), "ホーム");
      await missing.click();
      await page.waitForURL("**/cfgb-example/en/");
    });
  });

  test("new languages search native text through Pagefind", async () => {
    await withPage(site, {}, async (page) => {
      for (const [language, query, placeholder] of [
        ["zh-Hans", "语言", "搜索"],
        ["ko", "한국어", "검색어"],
      ]) {
        await page.goto(`${site.origin}/cfgb-example/${language}/search/`);
        assert.equal(
          await page.getByRole("textbox").getAttribute("placeholder"),
          placeholder,
        );
        await page.getByRole("textbox").fill(query);
        const result = page.locator(".pagefind-ui__result-link").first();
        await result.waitFor();
        assert.ok(
          (await result.getAttribute("href")).includes(
            `/${language}/posts/browser/`,
          ),
        );
      }
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
