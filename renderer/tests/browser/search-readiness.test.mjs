import assert from "node:assert/strict";
import { test } from "node:test";
import { startSite, withPage } from "./fixture.mjs";
import { waitForSearch } from "./search-ui.mjs";

test("search completion waits for the actual index response and empty results", { timeout: 90_000 }, async () => {
  const requested = Promise.withResolvers();
  const delivery = Promise.withResolvers();
  const site = await startSite({
    enabledLanguages: ["zh-Hans"],
    async beforeResponse(path) {
      if (path.endsWith(".pf_meta")) {
        requested.resolve();
        await delivery.promise;
      }
    },
  });
  try {
    await withPage(site, {}, async page => {
      await page.goto(`${site.origin}/zh-Hans/search/`);
      const query = "zzqvnonexistentsearchtokenzzqv";
      await page.getByRole("textbox").fill(query);
      await requested.promise;
      await page.waitForFunction(query => document.querySelector(".pagefind-ui__message")?.textContent.includes(query), query);
      const ready = waitForSearch(page, query);
      const first = await Promise.race([
        ready.then(() => "complete"),
        page.evaluate(() => "index response pending"),
      ]);
      delivery.resolve();
      await ready;
      assert.equal(first, "index response pending", "a loading message is not a completed search");
      assert.equal(await page.locator(".pagefind-ui__results").count(), 1);
      assert.equal(await page.locator(".pagefind-ui__result-title > a").count(), 0);
    });
    await assert.rejects(withPage(site, {}, async page => {
      await page.goto(`${site.origin}/zh-Hans/search/`);
      await page.evaluate(async () => { await fetch("/missing-resource.js"); });
      throw new Error("original browser action failure");
    }), error => {
      assert(error instanceof AggregateError);
      assert.equal(error.errors[0].message, "original browser action failure");
      assert(error.errors.some(item => item.message.includes("404") && item.message.includes("missing-resource.js")));
      return true;
    });
  } finally {
    delivery.resolve();
    await site.close();
  }
});
