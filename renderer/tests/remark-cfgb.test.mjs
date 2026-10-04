import assert from "node:assert/strict";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { unified } from "@astrojs/markdown-remark";
import { resetSiteCache } from "../src/lib/load-site.mjs";
import { remarkCfgb } from "../src/plugins/remark-cfgb.mjs";

test("alerts keep markdown children and raw HTML links use the route", async (t) => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-"));
  try {
    const target = path.join(
      dir,
      "content",
      "posts",
      "2026",
      "2026-09-19-protobuf-guide",
    );
    const source = path.join(
      dir,
      "content",
      "posts",
      "2026",
      "2026-09-20-markdown-showcase",
    );
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
                {
                  type: "emphasis",
                  children: [{ type: "text", value: "this" }],
                },
                {
                  type: "link",
                  url: "https://example.invalid/docs",
                  children: [{ type: "text", value: "docs" }],
                },
              ],
            },
            {
              type: "list",
              children: [
                {
                  type: "listItem",
                  children: [
                    {
                      type: "paragraph",
                      children: [{ type: "text", value: "item" }],
                    },
                  ],
                },
              ],
            },
            { type: "code", lang: "js", value: "const n = 1;\n" },
          ],
        },
        {
          type: "html",
          value:
            '<a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a>',
        },
        {
          type: "html",
          value:
            "<a href=../2026-09-19-protobuf-guide/en.md#field-numbers>Field numbers</a>",
        },
        {
          type: "html",
          value:
            '<!-- <a href="../2026-09-19-protobuf-guide/en.md#field-numbers">hidden</a> -->',
        },
        {
          type: "code",
          value:
            '<a href="../2026-09-19-protobuf-guide/en.md#field-numbers">Field numbers</a>',
        },
        {
          type: "code",
          lang: "mermaid",
          value: "graph TD\nA-->B\n",
        },
        {
          type: "link",
          url: "./assets/diagram.png",
          children: [{ type: "text", value: "diagram" }],
        },
        { type: "imageReference", identifier: "picture", alt: "Reference" },
        {
          type: "definition",
          identifier: "picture",
          url: "./assets/picture.svg",
        },
        {
          type: "definition",
          identifier: "fig",
          url: "./assets/figures/one.png",
        },
        { type: "html", value: '<a href="./assets/a b.png">asset</a>' },
        {
          type: "link",
          url: "./assets/a%20b.png",
          children: [{ type: "text", value: "download" }],
        },
        { type: "image", url: "./assets/diagram.svg", alt: "Diagram" },
        { type: "html", value: '<img src="./assets/a%20b.png" alt="encoded">' },
      ],
    };
    remarkCfgb()(tree, { path: sourceFile });
    const alert = tree.children[0];
    assert.equal(alert.data.hName, "div");
    assert.deepEqual(alert.data.hProperties.className, ["alert", "alert-note"]);
    assert.equal(alert.children[0].children[0].value, "Note");
    assert.equal(alert.children[1].children[1].type, "emphasis");
    assert.equal(
      alert.children[1].children[2].url,
      "https://example.invalid/docs",
    );
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
    assert.match(
      tree.children[3].value,
      /2026-09-19-protobuf-guide\/en\.md#field-numbers/,
    );
    assert.match(
      tree.children[4].value,
      /2026-09-19-protobuf-guide\/en\.md#field-numbers/,
    );
    assert.equal(
      tree.children[5].value,
      '<div class="diagram-block"><pre class="mermaid">graph TD\nA--&gt;B</pre></div>',
    );
    assert.equal(
      tree.children[6].url,
      "/media/2026/2026-09-20-markdown-showcase/diagram.png",
    );
    assert.equal(tree.children[7].type, "image");
    assert.equal(tree.children[7].url, "./assets/picture.svg");
    assert.equal(tree.children[8].url, "./assets/picture.svg");
    assert.equal(tree.children[9].url, "./assets/figures/one.png");
    assert.equal(
      tree.children[10].value,
      '<a href="/media/2026/2026-09-20-markdown-showcase/a%20b.png">asset</a>',
    );
    assert.equal(
      tree.children[11].url,
      "/media/2026/2026-09-20-markdown-showcase/a%20b.png",
    );
    assert.equal(tree.children[12].url, "./assets/diagram.svg");
    assert.equal(
      tree.children[13].value,
      '<img src="/media/2026/2026-09-20-markdown-showcase/a%20b.png" alt="encoded">',
    );
    t.mock.method(console, "error", () => {});
    const processor = unified({ remarkPlugins: [remarkCfgb] });
    const renderer = await processor.createRenderer({ syntaxHighlight: false });
    for (const markdown of [
      "![Image](./assets/picture.svg?v=1)",
      "![Image][pic]\n\n[pic]: ./assets/picture.svg#detail",
      "[Download](./assets/picture.svg?v=1)",
      '<img src="./assets/picture.svg#detail">',
      '<a href="./assets/picture.svg?v=1">Download</a>',
    ])
      await assert.rejects(
        renderer.render(markdown, { fileURL: pathToFileURL(sourceFile) }),
        /query or fragment/,
      );
    // Unused definitions and literal examples are not file consumers.
    await renderer.render(
      "Example.\n\n[unused]: ./assets/missing.svg?v=1\n\n`![Image](./assets/missing.svg?v=1)`",
      { fileURL: pathToFileURL(sourceFile) },
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("markdown parsing keeps rewritten anchor labels inside the anchor", async () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-md-"));
  try {
    const target = path.join(
      dir,
      "content",
      "posts",
      "2026",
      "2026-09-19-protobuf-guide",
    );
    const source = path.join(
      dir,
      "content",
      "posts",
      "2026",
      "2026-09-20-markdown-showcase",
    );
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
        prose: [{ file: sourceFile, kind: "aside" }],
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
      "[download](./assets/a%20b.png)",
      "",
      "![diagram](./assets/diagram.svg)",
      "",
      '<img src="./assets/a%20b.png" alt="encoded">',
      "",
      '<a href="./assets/a%20b.png">file</a>',
      "",
    ].join("\n");
    const processor = unified({
      gfm: true,
      remarkPlugins: [remarkCfgb],
      smartypants: false,
    });
    const renderer = await processor.createRenderer({
      syntaxHighlight: false,
      gfm: true,
      smartypants: false,
    });
    const { code: html } = await renderer.render(markdown, {
      fileURL: pathToFileURL(sourceFile),
    });
    const anchor =
      '<a href="/en/posts/reading-protobuf-schemas/#field-numbers">Field numbers</a>';
    assert.equal(html.split(anchor).length - 1, 2);
    assert.equal(html.includes(anchor.replace(">", "></a>")), false);
    const media = "/media/aside";
    assert.equal(html.includes(`${media}/a%2520b.png`), false);
    assert.equal(html.includes("diagram.svg%23view"), false);
    assert.equal(html.includes(`${media}/a%20b.png`), true);
    assert.equal(html.includes("./assets/diagram.svg"), true);
    assert.equal(html.includes(`${media}/diagram.svg`), false);
    assert.equal(html.includes(`${media}/a%20b.png`), true);
    assert.equal(html.includes(`${media}/a%20b.png">file</a>`), true);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("prose links and raw HTML publish assets while Markdown images stay local", () => {
  const dir = mkdtempSync(path.join(tmpdir(), "cfgb-remark-prose-"));
  try {
    const home = path.join(dir, "content", "home");
    const about = path.join(dir, "content", "pages", "about");
    mkdirSync(home, { recursive: true });
    mkdirSync(about, { recursive: true });
    const homeFile = path.join(home, "ja.md");
    const aboutFile = path.join(about, "en.md");
    writeFileSync(homeFile, "Home\n");
    writeFileSync(aboutFile, "About\n");
    mkdirSync(path.join(dir, "linkcards"));
    const metadataPath = path.join(dir, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: {},
        posts: [],
        prose: [
          { file: homeFile, kind: "home" },
          { file: aboutFile, kind: "about" },
        ],
      }),
    );
    const sitePath = path.join(dir, "site.json");
    writeFileSync(
      sitePath,
      JSON.stringify({
        title: "Example",
        baseUrl: "https://example.invalid",
        defaultLocale: "ja",
        timezone: "UTC",
        locales: { ja: { label: "日本語" }, en: { label: "English" } },
        contentRoot: path.join(dir, "content"),
        topicsFile: path.join(dir, "topics.yaml"),
        linkcardsDir: path.join(dir, "linkcards"),
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    process.env.CFGB_SITE_JSON = sitePath;
    resetSiteCache();
    const children = [
      { type: "image", url: "./assets/portrait.svg", alt: "Markdown portrait" },
      {
        type: "imageReference",
        identifier: "picture",
        alt: "Reference portrait",
      },
      {
        type: "definition",
        identifier: "picture",
        url: "./assets/portrait.svg",
      },
      {
        type: "html",
        value: '<img src="./assets/portrait.svg" alt="Raw HTML portrait">',
      },
      {
        type: "link",
        url: "./assets/portrait.svg",
        children: [{ type: "text", value: "Markdown asset link" }],
      },
      {
        type: "html",
        value: '<a href="./assets/portrait.svg">Raw HTML asset link</a>',
      },
    ];
    const homeTree = { type: "root", children: structuredClone(children) };
    remarkCfgb()(homeTree, { path: homeFile });
    assert.equal(homeTree.children[0].url, "./assets/portrait.svg");
    assert.equal(homeTree.children[1].type, "image");
    assert.equal(homeTree.children[1].url, "./assets/portrait.svg");
    assert.equal(homeTree.children[2].url, "./assets/portrait.svg");
    assert.equal(
      homeTree.children[3].value,
      '<img src="/media/home/portrait.svg" alt="Raw HTML portrait">',
    );
    assert.equal(homeTree.children[4].url, "/media/home/portrait.svg");
    assert.equal(
      homeTree.children[5].value,
      '<a href="/media/home/portrait.svg">Raw HTML asset link</a>',
    );
    const aboutTree = { type: "root", children: structuredClone(children) };
    remarkCfgb()(aboutTree, { path: aboutFile });
    assert.equal(aboutTree.children[0].url, "./assets/portrait.svg");
    assert.equal(aboutTree.children[1].url, "./assets/portrait.svg");
    assert.equal(aboutTree.children[2].url, "./assets/portrait.svg");
    assert.equal(aboutTree.children[4].url, "/media/about/portrait.svg");
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
