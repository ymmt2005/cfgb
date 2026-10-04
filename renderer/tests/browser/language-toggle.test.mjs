import assert from "node:assert/strict";
import { after, before, describe, test } from "node:test";
import { startSite, withPage } from "./fixture.mjs";

describe(
  "a language pair uses a segmented toggle",
  { timeout: 180_000 },
  () => {
    let site;
    before(async () => {
      site = await startSite({
        basePath: "/blog",
        static: true,
        enabledLanguages: ["ja", "en"],
      });
    });
    after(async () => {
      await site?.close();
    });

    for (const javaScriptEnabled of [true, false]) {
      test(`toggle preserves the article and current language with JavaScript ${javaScriptEnabled}`, async () => {
        await withPage(
          site,
          { javaScriptEnabled, viewport: { width: 375, height: 812 } },
          async (page) => {
            await page.goto(`${site.origin}/blog/ja/posts/browser/`);
            assert.equal(await page.locator(".language-switch").count(), 0);
            const toggle = page.locator(".language-toggle");
            assert.equal(await toggle.getByRole("link").count(), 2);
            assert.equal(
              await toggle.locator('[aria-current="true"]').textContent(),
              "日本語",
            );
            const english = toggle.getByRole("link", {
              name: "English",
              exact: true,
            });
            await english.focus();
            await page.keyboard.press("Enter");
            await page.waitForURL("**/blog/en/posts/browser/");
            assert.equal(
              await toggle.locator('[aria-current="true"]').textContent(),
              "English",
            );
            await toggle
              .getByRole("link", { name: "日本語", exact: true })
              .click();
            await page.waitForURL("**/blog/ja/posts/browser/");
            assert.equal(
              await page.evaluate(
                () => document.documentElement.scrollWidth <= innerWidth,
              ),
              true,
            );
          },
        );
      });
    }

    test("a missing counterpart goes to the other language home", async () => {
      await withPage(site, { javaScriptEnabled: false }, async (page) => {
        await page.goto(`${site.origin}/blog/en/posts/diagram-error/`);
        const japanese = page.locator('.language-toggle a[hreflang="ja"]');
        assert.equal(await japanese.getAttribute("href"), "/blog/ja/");
        assert.equal(await japanese.locator("small").textContent(), "Home");
        await japanese.click();
        await page.waitForURL("**/blog/ja/");
      });
    });
  },
);
