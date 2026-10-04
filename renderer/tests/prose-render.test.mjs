import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { createIndex } from "pagefind";
import { parseFragment } from "parse5";

const rendererRoot = fileURLToPath(new URL("..", import.meta.url));
const portrait = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" fill="currentColor"/></svg>\n`;
const wide = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32" viewBox="0 0 64 32"><rect width="64" height="32" fill="currentColor"/></svg>\n`;
const pixel = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64",
);
const prose = [
  "![Markdown portrait](./assets/portrait.svg)",
  "",
  "![Reference portrait][picture]",
  "",
  "[picture]: ./assets/portrait.svg",
  "",
  '<img src="./assets/portrait.svg" alt="Raw HTML portrait" width="32">',
  "",
  "[Markdown asset link](./assets/portrait.svg)",
  "",
  '<a href="./assets/portrait.svg">Raw HTML asset link</a>',
  "",
].join("\n");
const articleBody = [
  "## Shared heading",
  "",
  "See [^note].",
  "",
  "![Inline article](./assets/picture.svg)",
  "",
  "![Reference article][picture]",
  "",
  "[picture]: ./assets/picture.svg",
  "",
  "![Raster article](./assets/pixel.png)",
  "",
  "[Raster file](./assets/pixel.png)",
  "",
  "[^note]: Article note.",
  "",
].join("\n");
const asideBody = [
  "## Shared heading",
  "",
  "See [^note].",
  "",
  "![Reference aside][picture]",
  "",
  "[picture]: ./assets/picture.svg",
  "",
  "[^note]: Aside note.",
  "",
].join("\n");

test("prose pages publish local images and untranslated articles stay in their group", { timeout: 180_000 }, async () => {
  const astroBin = path.join(rendererRoot, "node_modules", "astro", "bin", "astro.mjs");
  if (!existsSync(astroBin)) {
    throw new Error("renderer dependencies are not installed");
  }
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-prose-render-"));
  const content = path.join(work, "content");
  try {
    cpSync(rendererRoot, path.join(work, "renderer"), {
      recursive: true,
      filter(src) {
        const rel = path.relative(rendererRoot, src);
        if (!rel) return true;
        const top = rel.split(path.sep)[0];
        return top !== "node_modules" && top !== "dist" && top !== ".astro";
      },
    });
    symlinkSync(path.join(rendererRoot, "node_modules"), path.join(work, "renderer", "node_modules"));
    const bodies = {
      "home/ja.md": prose,
      "home/en.md": prose,
      "pages/about/ja.md": prose,
      "pages/about/en.md": prose,
    };
    for (const [name, body] of Object.entries(bodies)) {
      const file = path.join(content, name);
      mkdirSync(path.dirname(file), { recursive: true });
      writeFileSync(file, body);
      const assets = path.join(path.dirname(file), "assets");
      mkdirSync(assets, { recursive: true });
      writeFileSync(path.join(assets, "portrait.svg"), portrait);
    }
    const article = (locale, key, title, body, year = "2026", slug = "shared") => ({
      id: `posts/${year}/${key}/${locale}`,
      file: path.join(content, "posts", year, key, `${locale}.md`),
      body,
      group: `${year}/${key}`,
      year,
      articleKey: key,
      locale,
      data: {
        title,
        slug,
        publishedAt: "2026-09-20T00:00:00Z",
        topics: ["notes"],
        summary: title,
      },
    });
    const posts = [
      article("ja", "ja-only", "日本語だけ", articleBody),
      article("en", "en-only", "English only", "Body\n"),
      article("ja", "guide", "Later group", "Body\n", "2026", "later-group"),
      article("ja", "guide", "Earlier group", "Body\n", "2025", "earlier-group"),
    ];
    for (const post of posts) {
      mkdirSync(path.dirname(post.file), { recursive: true });
      writeFileSync(post.file, post.body);
    }
    const articleAssets = path.join(content, "posts", "2026", "ja-only", "assets");
    mkdirSync(articleAssets, { recursive: true });
    writeFileSync(path.join(articleAssets, "picture.svg"), wide);
    writeFileSync(path.join(articleAssets, "pixel.png"), pixel);
    for (const locale of ["ja", "en"]) {
      const file = path.join(content, "aside", `${locale}.md`);
      mkdirSync(path.dirname(file), { recursive: true });
      writeFileSync(file, asideBody);
      const assets = path.join(content, "aside", "assets");
      mkdirSync(assets, { recursive: true });
      writeFileSync(path.join(assets, "picture.svg"), wide);
    }
    for (const scope of ["home", "about"]) {
      const dir = path.join(work, "renderer", "public", "media", scope);
      mkdirSync(dir, { recursive: true });
      writeFileSync(path.join(dir, "portrait.svg"), portrait);
    }
    const articleMedia = path.join(work, "renderer", "public", "media", "2026", "ja-only");
    mkdirSync(articleMedia, { recursive: true });
    writeFileSync(path.join(articleMedia, "pixel.png"), pixel);
    const metadataPath = path.join(work, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: { notes: { ja: "メモ", en: "Notes" } },
        posts,
        prose: ["ja", "en"].flatMap((locale) => [
          { id: `home/${locale}`, kind: "home", locale, file: path.join(content, "home", `${locale}.md`), body: prose },
          { id: `pages/about/${locale}`, kind: "about", locale, file: path.join(content, "pages", "about", `${locale}.md`), body: prose },
          { id: `aside/${locale}`, kind: "aside", locale, file: path.join(content, "aside", `${locale}.md`), body: asideBody },
        ]),
      }),
    );
    const sitePath = path.join(work, "site.json");
    writeFileSync(
      sitePath,
      JSON.stringify({
        title: "Example",
        baseUrl: "https://example.invalid",
        defaultLocale: "ja",
        timezone: "UTC",
        locales: { ja: { label: "日本語" }, en: { label: "English" } },
        contentRoot: content,
        topicsFile: path.join(work, "topics.yaml"),
        linkcardsDir: path.join(work, "linkcards"),
        metadataFile: metadataPath,
        latestPosts: 5,
      }),
    );
    mkdirSync(path.join(work, "linkcards"));
    execFileSync(process.execPath, [astroBin, "build"], {
      cwd: path.join(work, "renderer"),
      env: { ...process.env, CFGB_SITE_JSON: sitePath },
      stdio: "pipe",
    });
    assert.ok(existsSync(path.join(work, "renderer", ".astro", "vite")), "Vite cache belongs to this workspace");
    const dist = path.join(work, "renderer", "dist");
    for (const page of ["ja/index.html", "en/index.html", "ja/about/index.html", "en/about/index.html"]) {
      const html = readFileSync(path.join(dist, page), "utf8");
      assert.equal(html.includes("__ASTRO_IMAGE_"), false, page);
      const markdown = imgSrc(html, "Markdown portrait");
      const raw = imgSrc(html, "Raw HTML portrait");
      const markdownLink = hrefForText(html, "Markdown asset link");
      const rawLink = hrefForText(html, "Raw HTML asset link");
      const scope = page.includes("/about/") ? "about" : "home";
      assert.equal(markdown.startsWith("/"), true, markdown);
      assert.equal(markdown.includes("./assets/"), false, markdown);
      assert.equal(markdown.includes("/media/"), false, markdown);
      assertProcessedImage(html, "Markdown portrait", "32", "32", dist);
      assertProcessedImage(html, "Reference portrait", "32", "32", dist);
      assert.equal(raw.split("?")[0], `/media/${scope}/portrait.svg`);
      assert.equal(markdownLink, `/media/${scope}/portrait.svg`);
      assert.equal(rawLink, `/media/${scope}/portrait.svg`);
      for (const url of [markdown, raw, markdownLink, rawLink]) assertPublished(dist, url);
    }
    const home = readFileSync(path.join(dist, "ja/index.html"), "utf8");
    const feed = readFileSync(path.join(dist, "ja/feed.xml"), "utf8");
    for (const html of [home, feed]) {
      assert.ok(html.indexOf("Earlier group") >= 0);
      assert.ok(html.indexOf("Later group") > html.indexOf("Earlier group"), "year/group tie-break");
    }
    const jaArticle = readFileSync(path.join(dist, "ja/posts/shared/index.html"), "utf8");
    const enArticle = readFileSync(path.join(dist, "en/posts/shared/index.html"), "utf8");
    assert.match(jaArticle, /href="\/__locale\?lang=en&amp;next=%2Fen%2F"/);
    assert.equal(jaArticle.includes("next=%2Fen%2Fposts%2Fshared%2F"), false);
    assert.match(enArticle, /href="\/__locale\?lang=ja&amp;next=%2Fja%2F"/);
    assert.equal(enArticle.includes("next=%2Fja%2Fposts%2Fshared%2F"), false);
    assert.match(jaArticle, /このページの対訳はありません/);
    assert.match(enArticle, /This page has no translation/);
    assert.equal(jaArticle.includes("__ASTRO_IMAGE_"), false);
    assertProcessedImage(jaArticle, "Inline article", "64", "32", dist);
    assertProcessedImage(jaArticle, "Reference article", "64", "32", dist);
    assertProcessedImage(jaArticle, "Raster article", "1", "1", dist);
    assertProcessedImage(jaArticle, "Reference aside", "64", "32", dist);
    const rasterLink = hrefForText(jaArticle, "Raster file");
    assert.equal(rasterLink, "/media/2026/ja-only/pixel.png");
    assertPublished(dist, rasterLink);
    assert.equal(jaArticle.includes('data-pagefind-meta="published[datetime]"'), true);
    assert.equal(jaArticle.includes("published[2026-09-20T00:00:00Z]"), false);
    const created = await createIndex();
    assert.deepEqual(created.errors, []);
    const added = await created.index.addHTMLFile({ url: "/ja/posts/shared/", content: jaArticle });
    assert.deepEqual(added.errors, []);
    assert.equal(added.file.meta.published, "2026-09-20T00:00:00Z");
    assertFootnotes(jaArticle);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});

function imgSrc(html, alt) {
  const start = html.indexOf(`alt="${alt}"`);
  assert.ok(start > 0, alt);
  const tagStart = html.lastIndexOf("<img", start);
  const tag = html.slice(tagStart, html.indexOf(">", start) + 1);
  const src = tag.match(/\bsrc="([^"]+)"/);
  assert.ok(src, tag);
  return src[1].replaceAll("&amp;", "&");
}

function hrefForText(html, text) {
  const match = html.match(new RegExp(`<a\\b[^>]*href="([^"]+)"[^>]*>${text}</a>`));
  assert.ok(match, text);
  return match[1].replaceAll("&amp;", "&");
}

function assertPublished(dist, url) {
  const pathname = url.split(/[?#]/)[0];
  const file = path.join(dist, pathname);
  assert.equal(existsSync(file), true, file);
}

function assertProcessedImage(html, alt, width, height, dist) {
  const src = imgSrc(html, alt);
  const tag = imgTag(html, alt);
  assert.equal(src.includes("/media/"), false, src);
  assert.equal(src.includes("./assets/"), false, src);
  assert.equal(tag.includes(`width="${width}"`), true, tag);
  assert.equal(tag.includes(`height="${height}"`), true, tag);
  assertPublished(dist, src);
}

function imgTag(html, alt) {
  const start = html.indexOf(`alt="${alt}"`);
  assert.ok(start > 0, alt);
  const tagStart = html.lastIndexOf("<img", start);
  return html.slice(tagStart, html.indexOf(">", start) + 1);
}

function assertFootnotes(html) {
  const root = parseFragment(html);
  const ids = new Map();
  const refs = [];
  const backs = [];
  walk(root, [], (node, ancestors) => {
    const id = attr(node, "id");
    if (id) {
      assert.equal(ids.has(id), false, id);
      ids.set(id, ancestors);
    }
    if (node.attrs?.some((item) => item.name === "data-footnote-ref")) {
      refs.push({ href: attr(node, "href"), describedBy: attr(node, "aria-describedby"), ancestors });
    }
    if (node.attrs?.some((item) => item.name === "data-footnote-backref")) {
      backs.push({ href: attr(node, "href"), ancestors });
    }
  });
  assert.equal(ids.has("shared-heading"), true, [...ids.keys()].join(" "));
  assert.equal(ids.has("aside-shared-heading"), true, [...ids.keys()].join(" "));
  assert.ok(ids.get("shared-heading").includes("article"));
  assert.ok(ids.get("aside-shared-heading").includes("aside"));
  assert.equal(ids.has("user-content-fn-note"), true);
  assert.equal(ids.has("aside-user-content-fn-note"), true);
  assert.ok(ids.get("user-content-fn-note").includes("article"));
  assert.ok(ids.get("aside-user-content-fn-note").includes("aside"));
  assert.equal(ids.has("user-content-fnref-note"), true);
  assert.equal(ids.has("aside-user-content-fnref-note"), true);
  const articleRef = refs.find((item) => item.href === "#user-content-fn-note");
  const asideRef = refs.find((item) => item.href === "#aside-user-content-fn-note");
  assert.ok(articleRef);
  assert.ok(asideRef);
  assert.equal(articleRef.describedBy, "footnote-label");
  assert.equal(asideRef.describedBy, "aside-footnote-label");
  assert.ok(articleRef.ancestors.includes("article"));
  assert.ok(asideRef.ancestors.includes("aside"));
  assert.ok(ids.get("footnote-label").includes("article"));
  assert.ok(ids.get("aside-footnote-label").includes("aside"));
  const articleBack = backs.find((item) => item.href === "#user-content-fnref-note");
  const asideBack = backs.find((item) => item.href === "#aside-user-content-fnref-note");
  assert.ok(articleBack);
  assert.ok(asideBack);
  assert.ok(articleBack.ancestors.includes("article"));
  assert.ok(asideBack.ancestors.includes("aside"));
  assert.equal(ids.get("user-content-fnref-note").includes("article"), true);
  assert.equal(ids.get("aside-user-content-fnref-note").includes("aside"), true);
}

function attr(node, name) {
  return node.attrs?.find((item) => item.name === name)?.value ?? "";
}

function walk(node, ancestors, visit) {
  const next = node.tagName ? [...ancestors, node.tagName] : ancestors;
  if (node.tagName) visit(node, next);
  for (const child of node.childNodes || []) walk(child, next, visit);
}
