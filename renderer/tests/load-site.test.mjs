import assert from "node:assert/strict";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import {
  formatDate,
  comparePosts,
  copyFor,
  loadSite,
  resetSiteCache,
} from "../src/lib/load-site.mjs";

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
            archive: { year: "2026", month: "10" },
            data: {
              title: "ガイド",
              slug: "protobuf-schema-guide",
              publishedAt: "2026-09-30T16:30:00Z",
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
    assert.equal(post.data.publishedAt, "2026-09-30T16:30:00Z");
    assert.deepEqual(post.archive, { year: "2026", month: "10" });
    assert.equal(site.topics.protobuf.ja, "Protocol Buffers");
    assert.equal(site.routes.has("/ja/posts/protobuf-schema-guide/"), true);
    assert.equal(site.routes.has("/ja/topics/protobuf/"), true);
    assert.equal(site.routes.has("/ja/archive/2026/10/"), true);
    assert.equal(site.routes.has("/ja/archive/2026/09/"), false);
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

test("publication ordering includes the full article group identity", () => {
  const entry = (group, publishedAt = "2026-01-01T00:00:00Z") => ({
    group,
    articleKey: group.split("/").at(-1),
    publishedAt,
  });
  const old = entry("2025/guide");
  const next = entry("2026/guide", "2026-01-01T09:00:00+09:00");
  assert.ok(comparePosts(old, next) < 0);
  assert.ok(comparePosts(next, old) > 0);
  assert.equal(comparePosts(old, { ...old }), 0);
  assert.deepEqual(
    [next, entry("2024/zebra"), old, entry("2026/alpha")]
      .sort(comparePosts)
      .map((p) => p.group),
    ["2026/alpha", "2025/guide", "2026/guide", "2024/zebra"],
  );
  assert.ok(comparePosts(entry("2026/zebra", "2026-01-02T00:00:00Z"), old) < 0);
});

test("static date fallback is UTC regardless of the build host timezone", () => {
  const previous = process.env.TZ;
  try {
    for (const host of ["Pacific/Honolulu", "Asia/Tokyo"]) {
      process.env.TZ = host;
      assert.equal(
        formatDate("2026-09-30T16:30:00Z", "ja", "UTC"),
        "2026年9月30日",
      );
      assert.equal(
        formatDate("2026-09-30T16:30:00Z", "en", "UTC"),
        "30 Sept 2026",
      );
    }
  } finally {
    if (previous === undefined) delete process.env.TZ;
    else process.env.TZ = previous;
  }
});
