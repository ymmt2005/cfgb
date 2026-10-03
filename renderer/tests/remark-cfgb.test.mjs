import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { unified } from "@astrojs/markdown-remark";
import { resetSiteCache } from "../src/lib/load-site.mjs";
import { remarkCfgb } from "../src/plugins/remark-cfgb.mjs";

test("alerts keep markdown children and raw HTML links use the route", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-"));
  try {
    const target = path.join(dir, "content", "posts", "2026", "2026-09-19-protobuf-guide");
    const source = path.join(dir, "content", "posts", "2026", "2026-09-20-markdown-showcase");
    mkdirSync(target, { recursive: true });
    mkdirSync(source, { recursive: true });
    const targetFile = path.join(target, "en.md");
    const sourceFile = path.join(source, "en.md");
    writeFileSync(targetFile, "Body\n");
    writeFileSync(sourceFile, "Body\n");
    mkdirSync(path.join(dir, "linkcards"));
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: { protobuf: { en: "Protocol Buffers" } },
        posts: [
          {
            id: "posts/2026/2026-09-19-protobuf-guide/en",
            file: targetFile,
            body: "Body\n",
            group: "2026/2026-09-19-protobuf-guide",
            year: "2026",
            articleKey: "2026-09-19-protobuf-guide",
            locale: "en",
            data: {
              title: "Reading",
              slug: "reading-protobuf-schemas",
              publishedAt: "2026-09-20T09:00:00+09:00",
              topics: ["protobuf"],
            },
          },
          {
            id: "posts/2026/2026-09-20-markdown-showcase/en",
            file: sourceFile,
            body: "Body\n",
            group: "2026/2026-09-20-markdown-showcase",
            year: "2026",
            articleKey: "2026-09-20-markdown-showcase",
            locale: "en",
            data: {
              title: "Showcase",
              slug: "markdown-rendering-showcase",
              publishedAt: "2026-09-21T10:00:00+09:00",
              topics: ["protobuf"],
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
        defaultLocale: "en",
        timezone: "UTC",
        locales: { en: { label: "English" } },
        contentRoot: path.join(dir, "content"),
        topicsFile: path.join(dir, "topics.yaml"),
        linkcardsDir: path.join(dir, "linkcards"),
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    process.env.CFGB_SITE_JSON = sitePath;
    resetSiteCache();
    const tree = {
      type: "root",
      children: [
        {
          type: "blockquote",
          children: [
            {
              type: "paragraph",
              children: [
                { type: "text", value: "[!NOTE]\nSee " },
                { type: "emphasis", children: [{ type: "text", value: "this" }] },
                { type: "link", url: "https://example.invalid/docs", children: [{ type: "text", value: "docs" }] },
              ],
            },
            {
              type: "list",
              children: [
                { type: "listItem", children: [{ type: "paragraph", children: [{ type: "text", value: "item" }] }] },
              ],
            },
            { type: "code", lang: "js", value: "const n = 1;\n" },
          ],
        },
        {
          type: "html",
          value: '<a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a>',
        },
        {
          type: "html",
          value: "<a href=../2026-09-19-protobuf-guide/en.md#field-numbers>Field numbers</a>",
        },
        {
          type: "html",
          value: '<!-- <a href="../2026-09-19-protobuf-guide/en.md#field-numbers">hidden</a> -->',
        },
        {
          type: "code",
          value: '<a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a>',
        },
      ],
    };
    remarkCfgb()(tree, { path: sourceFile });
    const alert = tree.children[0];
    assert.equal(alert.data.hName, "div");
    assert.deepEqual(alert.data.hProperties.className, ["alert", "alert-note"]);
    assert.equal(alert.children[0].children[0].value, "Note");
    assert.equal(alert.children[1].children[1].type, "emphasis");
    assert.equal(alert.children[1].children[2].url, "https://example.invalid/docs");
    assert.equal(alert.children[2].type, "list");
    assert.equal(alert.children[3].lang, "js");
    assert.equal(alert.children[3].value, "const n = 1;\n");
    assert.equal(
      tree.children[1].value,
      '<a href="/en/posts/reading-protobuf-schemas/#field-numbers">Field numbers</a>',
    );
    assert.equal(
      tree.children[2].value,
      "<a href=/en/posts/reading-protobuf-schemas/#field-numbers>Field numbers</a>",
    );
    assert.match(tree.children[3].value, /2026-09-19-protobuf-guide\/en\.md#field-numbers/);
    assert.match(tree.children[4].value, /2026-09-19-protobuf-guide\/en\.md#field-numbers/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("markdown parsing keeps rewritten anchor labels inside the anchor", async () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-md-"));
  try {
    const target = path.join(dir, "content", "posts", "2026", "2026-09-19-protobuf-guide");
    const source = path.join(dir, "content", "posts", "2026", "2026-09-20-markdown-showcase");
    mkdirSync(target, { recursive: true });
    mkdirSync(source, { recursive: true });
    const targetFile = path.join(target, "en.md");
    const sourceFile = path.join(source, "en.md");
    writeFileSync(targetFile, "Body\n");
    writeFileSync(sourceFile, "Body\n");
    mkdirSync(path.join(dir, "linkcards"));
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: { protobuf: { en: "Protocol Buffers" } },
        posts: [
          {
            id: "posts/2026/2026-09-19-protobuf-guide/en",
            file: targetFile,
            body: "Body\n",
            group: "2026/2026-09-19-protobuf-guide",
            year: "2026",
            articleKey: "2026-09-19-protobuf-guide",
            locale: "en",
            data: {
              title: "Reading",
              slug: "reading-protobuf-schemas",
              publishedAt: "2026-09-20T09:00:00+09:00",
              topics: ["protobuf"],
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
        defaultLocale: "en",
        timezone: "UTC",
        locales: { en: { label: "English" } },
        contentRoot: path.join(dir, "content"),
        topicsFile: path.join(dir, "topics.yaml"),
        linkcardsDir: path.join(dir, "linkcards"),
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    process.env.CFGB_SITE_JSON = sitePath;
    resetSiteCache();
    const markdown = [
      'Paragraph with <a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a> inside.',
      "",
      '<a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a>',
      "",
    ].join("\n");
    const processor = unified({ gfm: true, remarkPlugins: [remarkCfgb], smartypants: false });
    const renderer = await processor.createRenderer({ syntaxHighlight: false, gfm: true, smartypants: false });
    const { code: html } = await renderer.render(markdown, { fileURL: pathToFileURL(sourceFile) });
    const anchor = '<a href="/en/posts/reading-protobuf-schemas/#field-numbers">Field numbers</a>';
    assert.equal(html.split(anchor).length - 1, 2);
    assert.equal(html.includes(anchor.replace(">", "></a>")), false);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("asset paths rewrite when the source path uses Windows separators", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-win-"));
  try {
    const source = path.join(dir, "content", "posts", "2026", "article-key");
    mkdirSync(source, { recursive: true });
    const sourceFile = path.join(source, "en.md");
    writeFileSync(sourceFile, "Body\n");
    mkdirSync(path.join(dir, "linkcards"));
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: {},
        posts: [
          {
            id: "posts/2026/article-key/en",
            file: sourceFile,
            body: "Body\n",
            group: "2026/article-key",
            year: "2026",
            articleKey: "article-key",
            locale: "en",
            data: {
              title: "Assets",
              slug: "assets",
              publishedAt: "2026-09-20T09:00:00+09:00",
              topics: [],
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
        defaultLocale: "en",
        timezone: "UTC",
        locales: { en: { label: "English" } },
        contentRoot: path.join(dir, "content"),
        topicsFile: path.join(dir, "topics.yaml"),
        linkcardsDir: path.join(dir, "linkcards"),
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    process.env.CFGB_SITE_JSON = sitePath;
    resetSiteCache();
    const tree = {
      type: "root",
      children: [
        { type: "image", url: "./assets/diagram.png", alt: "Diagram" },
        { type: "image", url: "./assets/figures/one.png", alt: "One" },
      ],
    };
    const windowsPath = ["C:", "content", "posts", "2026", "article-key", "en.md"].join("\\");
    remarkCfgb()(tree, { path: windowsPath });
    assert.equal(tree.children[0].url, "/media/2026/article-key/diagram.png");
    assert.equal(tree.children[1].url, "/media/2026/article-key/figures/one.png");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
