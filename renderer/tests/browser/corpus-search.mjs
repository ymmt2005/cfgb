import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { serveBuiltSite } from "./site-server.mjs";
import { withPage } from "./fixture.mjs";

// Called by the Go corpus tests with their actual completed artifact and
// source-derived metadata. Pagefind and its language WASM run in Chromium.
const { schemaVersion, cases, corpus } = JSON.parse(readFileSync(0, "utf8"));
assert.equal(schemaVersion, 1);
assert(cases.length > 0, "the search corpus must execute queries");
const basePath = new URL(corpus.baseURL).pathname.replace(/\/$/, "");
const articles = new Map(corpus.articles.map((article) => [article.url, article]));
const locales = [...new Set(corpus.articles.map((article) => article.locale))];
const resultLinks = ".pagefind-ui__result-inner > .pagefind-ui__result-title > a";
const failures = [];

function canonicalPath(value, origin) {
  const url = new URL(value, origin);
  assert.equal(url.origin, origin, "result must navigate on the served site");
  assert.equal(url.search, "", "canonical result has no query");
  assert.equal(url.hash, "", "article result has no fragment");
  assert(url.pathname.startsWith(`${basePath}/`), "result must retain the hosting prefix");
  return url.pathname.slice(basePath.length);
}

function checkData(data, locale, filters, origin) {
  const route = canonicalPath(data.url, origin);
  const article = articles.get(route);
  assert(article, `result must be a canonical article: ${route}`);
  assert.equal(article.locale, locale, `foreign-locale result: ${route}`);
  assert.deepEqual(data.filters.locale, [locale]);
  assert.deepEqual(new Set(data.filters.topic), new Set(article.data.topics));
  assert.deepEqual(data.filters.year, [article.year]);
  assert.equal(data.meta.title, article.data.title);
  assert.equal(data.meta.summary, article.data.summary);
  assert.equal(new Date(data.meta.published).toISOString(), new Date(article.data.publishedAt).toISOString());
  for (const [name, value] of Object.entries(filters || {})) {
    assert(data.filters[name]?.includes(value), `${route} violates ${name}=${value}`);
  }
  return route;
}

async function search(page, locale, query, filters = {}) {
  return page.evaluate(async ({ basePath, locale, query, filters }) => {
    const pagefind = await import(`${basePath}/pagefind/pagefind.js`);
    const result = await pagefind.search(query, { filters: { ...filters, locale } });
    return Promise.all(result.results.map((entry) => entry.data()));
  }, { basePath, locale, query, filters });
}

async function setFilter(page, name, value, checked = true) {
  const input = page.locator(`.pagefind-ui__filter-checkbox[name="${name}"][value="${value}"]`);
  const group = input.locator("xpath=ancestor::details");
  if (!(await group.evaluate((node) => node.open))) await group.locator("summary").click();
  await input.setChecked(checked);
}

async function waitForResults(page, query, paths, topK, expected) {
  // Observe the rendered query and canonical article links, including empty
  // results. Nested heading links are not separate ranked article results.
  await page.waitForFunction(({ query, paths, topK, expected, resultLinks, basePath }) => {
    const message = document.querySelector(".pagefind-ui__message")?.textContent || "";
    const links = [...document.querySelectorAll(resultLinks)];
    if (!message.includes(query)) return false;
    if (paths.length === 0) return links.length === 0;
    if (links.length < Math.min(paths.length, topK) || links.length > paths.length) return false;
    const actual = links.map((link) => new URL(link.href).pathname);
    return actual.every((route) => paths.some((path) => route === basePath + path)) &&
      expected.every((route) => actual.slice(0, topK).includes(basePath + route));
  }, { query, paths, topK, expected, resultLinks, basePath });
}

const site = await serveBuiltSite(corpus.siteRoot, { basePath });
try {
  for (const locale of locales) {
    await withPage(site, {
      // Document language, not browser UI language, selects the search index.
      locale: locale === "ja" ? "en-US" : "ja-JP",
      viewport: { width: 1280, height: 800 },
    }, async (page) => {
      const wasm = [];
      const remote = [];
      page.on("response", (response) => {
        if (new URL(response.url()).pathname.startsWith(`${basePath}/pagefind/wasm.`)) wasm.push(response.status());
      });
      page.on("request", (request) => {
        const url = new URL(request.url());
        if (["http:", "https:"].includes(url.protocol) && url.origin !== site.origin) remote.push(url.href);
      });
      const searchURL = `${site.origin}${basePath}/${locale}/search/`;
      const localeCases = cases.filter((item) => item.locale === locale);
      assert(localeCases.length > 0, `no query coverage for ${locale}`);
      const previousFailures = failures.length;
      let navigationExample;
      await page.goto(searchURL);
      const indexed = await search(page, locale, null);
      const indexedPaths = indexed.map((data) => checkData(data, locale, {}, site.origin));
      assert.equal(new Set(indexedPaths).size, indexedPaths.length, "duplicate canonical search result");
      assert.deepEqual(new Set(indexedPaths), new Set(corpus.articles.filter((article) => article.locale === locale).map((article) => article.url)), "complete article-only index membership");

      for (const item of localeCases) {
        try {
          assert(Number.isInteger(item.topK) && item.topK > 0);
          for (const route of item.expected) assert.equal(articles.get(route)?.locale, locale, `unknown or foreign expected article: ${route}`);
          const data = await search(page, locale, item.query, item.filters);
          const paths = data.map((entry) => checkData(entry, locale, item.filters, site.origin));
          if (item.expected.length === 0) assert.deepEqual(paths, [], "expected no search results");
          for (const route of item.expected) assert(paths.slice(0, item.topK).includes(route), `${route} missing from top ${item.topK}: ${JSON.stringify(paths)}`);

          await page.goto(searchURL);
          await page.getByRole("textbox").fill(item.query);
          await page.waitForFunction((query) => document.querySelector(".pagefind-ui__message")?.textContent.includes(query), item.query);
          for (const [name, value] of Object.entries(item.filters || {})) await setFilter(page, name, value);
          await waitForResults(page, item.query, paths, item.topK, item.expected);
          assert(await page.locator(`.pagefind-ui__filter-checkbox[name="locale"][value="${locale}"]`).isChecked());
          if (!navigationExample && item.expected.length && !item.filters) navigationExample = item;
          console.log(`${locale} ${JSON.stringify(item.query)} ${JSON.stringify(item.filters || {})}: top-k, metadata, locale and UI passed`);
        } catch (error) {
          failures.push(new Error(`${locale} query ${JSON.stringify(item.query)} filters ${JSON.stringify(item.filters || {})}: ${error.message}`, { cause: error }));
        }
      }

      // Follow a real corpus result and compare its destination to source,
      // independently of the index URL assertions above.
      if (navigationExample) {
        await page.goto(searchURL);
        await page.getByRole("textbox").fill(navigationExample.query);
        const article = articles.get(navigationExample.expected[0]);
        const link = page.locator(`${resultLinks}[href="${basePath}${article.url}"]`);
        await link.click();
        await page.waitForURL(`${site.origin}${basePath}${article.url}`);
        assert.equal(await page.locator("html").getAttribute("lang"), locale);
        assert.equal(await page.locator("h1.article-title").textContent(), article.data.title);
        assert.equal(await page.locator('link[rel="canonical"]').getAttribute("href"), corpus.baseURL.replace(/\/$/, "") + article.url);
      } else if (failures.length === previousFailures) {
        assert.fail(`locale ${locale} has no navigation query`);
      }
      assert(wasm.length > 0, `no actual WASM response for ${locale}`);
      assert(wasm.every((status) => status === 200));
      assert.deepEqual(remote, [], "unexpected remote request while using local search");
    });
  }
  assert(cases.every((item) => locales.includes(item.locale)), "query locale missing from the corpus");
  if (failures.length) throw new AggregateError(failures, "Pagefind query corpus failed");
} finally {
  await site.close();
}
