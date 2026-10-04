import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { copyFor, loadSite, resetSiteCache } from "../src/lib/load-site.mjs";

test("routes and aliases come from the Go metadata index", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-index-"));
  try {
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: {
          protobuf: { en: "Protocol Buffers", ja: "Protocol Buffers" },
        },
        posts: [
          {
            id: "posts/2026/guide/ja",
            file: path.join(dir, "ja.md"),
            body: "本文\n\n---\n",
            group: "2026/guide",
            year: "2026",
            articleKey: "guide",
            locale: "ja",
            data: {
              title: "ガイド",
              slug: "protobuf-schema-guide",
              publishedAt: "2026-09-19T13:12:40+09:00",
              topics: ["protobuf"],
              aliases: ["/ja/posts/old-protobuf-guide/"],
            },
          },
        ],
        prose: [],
      }),
    );
    const sitePath = path.join(dir, "site.json");
    writeFileSync(
      sitePath,
      JSON.stringify({
        title: "Example",
        baseUrl: "https://example.invalid",
        defaultLocale: "ja",
        timezone: "Asia/Tokyo",
        locales: { ja: { label: "日本語" }, en: { label: "English" } },
        contentRoot: dir,
        topicsFile: path.join(dir, "topics.yaml"),
        linkcardsDir: dir,
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    process.env.CFGB_SITE_JSON = sitePath;
    resetSiteCache();
    const site = loadSite();
    const post = site.posts[0];
    assert.equal(post.slug, "protobuf-schema-guide");
    assert.equal(post.body, "本文\n\n---\n");
    assert.deepEqual(post.topics, ["protobuf"]);
    assert.deepEqual(post.aliases, ["/ja/posts/old-protobuf-guide/"]);
    assert.equal(post.url, "/ja/posts/protobuf-schema-guide/");
    assert.equal(site.topics.protobuf.ja, "Protocol Buffers");
    assert.equal(site.routes.has("/ja/posts/protobuf-schema-guide/"), true);
    assert.equal(site.routes.has("/ja/topics/protobuf/"), true);
    assert.equal(site.routes.has("/en/topics/protobuf/"), false);
  } finally {
    rmSync(dir, { recursive: true, force: true });
    resetSiteCache();
  }
});

test("language support is exact catalog membership", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-locale-"));
  try {
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({ topics: {}, posts: [], prose: [] }),
    );
    const sitePath = path.join(dir, "site.json");
    for (const locale of [
      "pt-BR",
      "EN",
      "JA",
      "en-US",
      "ja-JP",
      "../../escape",
      "%2e%2e",
      "constructor",
      "toString",
      "__proto__",
      "",
    ]) {
      writeFileSync(
        sitePath,
        JSON.stringify({
          title: "Example",
          baseUrl: "https://example.invalid",
          defaultLocale: locale,
          timezone: "UTC",
          locales: { [locale]: { label: "Label" } },
          metadataFile: metadataPath,
        }),
      );
      process.env.CFGB_SITE_JSON = sitePath;
      resetSiteCache();
      assert.throws(() => loadSite(), /not supported/, locale);
      assert.throws(() => copyFor(locale), /not supported/, locale);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
    resetSiteCache();
  }
});
