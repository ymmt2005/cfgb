import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cpSync, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const rendererRoot = fileURLToPath(new URL("..", import.meta.url));
const portrait = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32"><rect width="32" height="32" fill="currentColor"/></svg>\n`;
const prose = [
  "![Markdown portrait](./assets/portrait.svg)",
  "",
  '<img src="./assets/portrait.svg" alt="Raw HTML portrait" width="32">',
  "",
  "[Markdown asset link](./assets/portrait.svg)",
  "",
  '<a href="./assets/portrait.svg">Raw HTML asset link</a>',
  "",
].join("\n");

test("prose pages publish local images and untranslated articles stay in their group", { timeout: 180_000 }, () => {
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
    const article = (locale, key, title) => ({
      id: `posts/2026/${key}/${locale}`,
      file: path.join(content, "posts", "2026", key, `${locale}.md`),
      body: "Body\n",
      group: `2026/${key}`,
      year: "2026",
      articleKey: key,
      locale,
      data: {
        title,
        slug: "shared",
        publishedAt: "2026-09-20T00:00:00Z",
        topics: ["notes"],
        summary: title,
      },
    });
    for (const post of [
      ["ja", "ja-only", "日本語だけ"],
      ["en", "en-only", "English only"],
    ]) {
      const file = path.join(content, "posts", "2026", post[1], `${post[0]}.md`);
      mkdirSync(path.dirname(file), { recursive: true });
      writeFileSync(file, "Body\n");
    }
    for (const scope of ["home", "about"]) {
      const dir = path.join(work, "renderer", "public", "media", scope);
      mkdirSync(dir, { recursive: true });
      writeFileSync(path.join(dir, "portrait.svg"), portrait);
    }
    const metadataPath = path.join(work, "metadata.json");
    writeFileSync(
      metadataPath,
      JSON.stringify({
        topics: { notes: { ja: "メモ", en: "Notes" } },
        posts: [article("ja", "ja-only", "日本語だけ"), article("en", "en-only", "English only")],
        prose: ["ja", "en"].flatMap((locale) => [
          { id: `home/${locale}`, kind: "home", locale, file: path.join(content, "home", `${locale}.md`), body: prose },
          { id: `pages/about/${locale}`, kind: "about", locale, file: path.join(content, "pages", "about", `${locale}.md`), body: prose },
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
      assert.equal(raw.split("?")[0], `/media/${scope}/portrait.svg`);
      assert.equal(markdownLink, `/media/${scope}/portrait.svg`);
      assert.equal(rawLink, `/media/${scope}/portrait.svg`);
      for (const url of [markdown, raw, markdownLink, rawLink]) assertPublished(dist, url);
    }
    const jaArticle = readFileSync(path.join(dist, "ja/posts/shared/index.html"), "utf8");
    const enArticle = readFileSync(path.join(dist, "en/posts/shared/index.html"), "utf8");
    assert.match(jaArticle, /href="\/__locale\?lang=en&amp;next=%2Fen%2F"/);
    assert.equal(jaArticle.includes("next=%2Fen%2Fposts%2Fshared%2F"), false);
    assert.match(enArticle, /href="\/__locale\?lang=ja&amp;next=%2Fja%2F"/);
    assert.equal(enArticle.includes("next=%2Fja%2Fposts%2Fshared%2F"), false);
    assert.match(jaArticle, /このページの対訳はありません/);
    assert.match(enArticle, /This page has no translation/);
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
