import assert from "node:assert/strict";
import test from "node:test";
import { canonicalSitemapPath, localePageAlternates } from "../src/lib/sitemap.mjs";

test("sitemap classification ignores words inside slugs", () => {
  assert.equal(canonicalSitemapPath("/ja/posts/http-404-guide/"), true);
  assert.equal(canonicalSitemapPath("/en/posts/build-a-sitemap/"), true);
  assert.equal(canonicalSitemapPath("/ja/posts/protobuf-schema-guide/"), true);
  assert.equal(canonicalSitemapPath("/ja/search/"), false);
  assert.equal(canonicalSitemapPath("/en/feed.xml"), false);
  assert.equal(canonicalSitemapPath("/robots.txt"), false);
  assert.equal(canonicalSitemapPath("/404.html"), false);
  assert.equal(canonicalSitemapPath("/ja/404.html"), false);
  assert.equal(canonicalSitemapPath("/sitemap-index.xml"), false);
  assert.equal(canonicalSitemapPath("/sitemap-0.xml"), false);
});

test("home and about alternates follow configured locales without prose", () => {
  const pages = localePageAlternates(["ja", "en"]);
  assert.deepEqual(pages.home, { ja: "/ja/", en: "/en/" });
  assert.deepEqual(pages.about, { ja: "/ja/about/", en: "/en/about/" });
});
