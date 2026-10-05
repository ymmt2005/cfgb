import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, symlinkSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { parse } from "parse5";
import sharp from "sharp";
import { renderSocialImage } from "../src/lib/social-images.mjs";

const rendererRoot = fileURLToPath(new URL("..", import.meta.url));

test("built articles publish explicit and fallback PNGs with canonical social metadata", { timeout: 180_000 }, async () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-social-images-"));
  try {
    const renderer = path.join(work, "renderer");
    cpSync(rendererRoot, renderer, {
      recursive: true,
      filter: (src) => !["node_modules", "dist", ".astro"].includes(path.relative(rendererRoot, src).split(path.sep)[0]),
    });
    symlinkSync(path.join(rendererRoot, "node_modules"), path.join(renderer, "node_modules"));
    const content = path.join(work, "content");
    const title = '日本語の画像 <タグ> & "引用"';
    const site = { title: "CFGB & Example", image: path.join(work, "site-image"), baseUrl: "https://example.invalid/blog/", defaultLocale: "ja", locales: { ja: { label: "日本語" }, en: { label: "English" } }, latestPosts: 5 };
    writeFileSync(site.image, '<svg xmlns="http://www.w3.org/2000/svg" width="32" height="16"><rect width="32" height="16" fill="#24537a"/></svg>');
    const posts = [
      article("ja", "explicit-svg", title, "./assets/図%23.svg"),
      article("en", "explicit-png", "Explicit PNG", "./assets/picture.png"),
      article("ja", "fallback", title),
      article("en", "fallback", "An English translation"),
    ];
    for (const post of posts) {
      post.file = path.join(content, "posts", post.group, `${post.locale}.md`);
      const assets = path.join(path.dirname(post.file), "assets");
      mkdirSync(assets, { recursive: true });
      writeFileSync(post.file, post.body);
      writeFileSync(path.join(assets, "図#.svg"), '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="#ff0000"/></svg>');
      await sharp({ create: { width: 240, height: 120, channels: 3, background: "#00ff00" } }).png().toFile(path.join(assets, "picture.png"));
    }
    const metadataFile = path.join(work, "metadata.json");
    writeFileSync(metadataFile, JSON.stringify({ posts, prose: [], topics: {} }));
    const siteFile = path.join(work, "site.json");
    writeFileSync(siteFile, JSON.stringify({ ...site, contentRoot: content, metadataFile }));
    execFileSync(process.execPath, [path.join(rendererRoot, "node_modules/astro/bin/astro.mjs"), "build"], {
      cwd: renderer, env: { ...process.env, CFGB_SITE_JSON: siteFile }, stdio: "pipe",
    });
    const dist = path.join(renderer, "dist");
    const images = new Set();
    for (const post of posts) {
      const html = readFileSync(path.join(dist, post.locale, "posts", post.data.slug, "index.html"), "utf8");
      const document = parse(html);
      const meta = (name) => attribute(document, "meta", name.includes(":") && !name.startsWith("twitter:") ? "property" : "name", name, "content");
      assert.equal(meta("og:title"), post.data.title);
      assert.equal(meta("twitter:title"), post.data.title);
      assert.equal(meta("og:description"), post.data.summary);
      assert.equal(meta("twitter:description"), post.data.summary);
      assert.equal(meta("twitter:card"), "summary_large_image");
      assert.equal(meta("og:url"), `${site.baseUrl}${post.locale}/posts/${post.data.slug}/`);
      const image = new URL(meta("og:image"));
      assert.equal(image.origin, "https://example.invalid");
      assert.ok(image.pathname.startsWith("/blog/og/"), image.href);
      assert.equal(meta("twitter:image"), image.href);
      assert.equal(meta("og:image:alt"), post.data.title);
      assert.equal(meta("twitter:image:alt"), post.data.title);
      assert.equal(meta("og:image:type"), "image/png");
      const bytes = readFileSync(path.join(dist, image.pathname.slice("/blog/".length)));
      const info = await sharp(bytes).metadata();
      assert.equal(info.format, "png");
      const dimensions = post.data.slug === "explicit-svg" ? [320, 180] : post.data.slug === "explicit-png" ? [240, 120] : [1200, 630];
      assert.deepEqual([info.width, info.height], dimensions);
      assert.equal(meta("og:image:width"), String(info.width));
      assert.equal(meta("og:image:height"), String(info.height));
      assert.equal(meta("article:published_time"), post.data.publishedAt);
      assert.equal(meta("article:modified_time"), post.data.updatedAt);
      const jsonLd = JSON.parse(find(document, (node) => node.tagName === "script" && attr(node, "type") === "application/ld+json").childNodes[0].value);
      assert.equal(jsonLd.image, image.href);
      images.add(image.href);
      if (post.data.ogImage) {
        const { data } = await sharp(bytes).removeAlpha().raw().toBuffer({ resolveWithObject: true });
        assert.deepEqual([...data.subarray(0, 3)], post.data.slug === "explicit-svg" ? [255, 0, 0] : [0, 255, 0]);
      } else {
        const data = await sharp(bytes).extract({ left: 144, top: 526, width: 1, height: 1 }).removeAlpha().raw().toBuffer();
        assert.deepEqual([...data], [36, 83, 122], "site image appears in fallback branding");
      }
    }
    assert.equal(images.size, posts.length, "each locale/article has its own image");
    assert.equal(readFileSync(path.join(dist, "sitemap-0.xml"), "utf8").includes("/og/"), false);
    const home = parse(readFileSync(path.join(dist, "ja/index.html"), "utf8"));
    assert.equal(attribute(home, "meta", "name", "twitter:card", "content"), "summary");
    assert.equal(find(home, (node) => node.tagName === "meta" && attr(node, "property") === "og:image"), undefined);
    for (const [name, rel, size] of [["favicon", "icon", 32], ["apple-touch-icon", "apple-touch-icon", 180]]) {
      assert.equal(attribute(home, "link", "rel", rel, "href"), `/blog/${name}.png`);
      const icon = readFileSync(path.join(dist, `${name}.png`));
      const info = await sharp(icon).metadata();
      assert.deepEqual([info.format, info.width, info.height], ["png", size, size]);
      const centre = await sharp(icon).extract({ left: size / 2, top: size / 2, width: 1, height: 1 }).raw().toBuffer();
      assert.deepEqual([...centre], [36, 83, 122, 255]);
      const corner = await sharp(icon).extract({ left: 0, top: 0, width: 1, height: 1 }).raw().toBuffer();
      assert.equal(corner[3], 0, "rectangular branding is padded rather than cropped");
    }
    // The font is a build dependency, not a published web font.
    assert.equal(findFile(dist, (name) => /\.(?:otf|ttf)$/.test(name)), undefined);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});

test("explicit OG source failures keep their cause and do not become fallback images", async () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-social-errors-"));
  try {
    mkdirSync(path.join(work, "assets"));
    const post = { id: "posts/2026/guide/ja", file: path.join(work, "ja.md"), title: "Guide", ogImage: "./assets/missing.png" };
    const site = { title: "Example", baseUrl: "https://example.invalid" };
    await assert.rejects(renderSocialImage(post, site, rendererRoot), (error) => error.message.includes(post.id) && error.cause.code === "ENOENT");
    writeFileSync(path.join(work, "assets", "broken.png"), "not an image");
    await assert.rejects(renderSocialImage({ ...post, ogImage: "./assets/broken.png" }, site, rendererRoot), /Render OG image.*unsupported image format/i);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});

test("fallbacks fit long multilingual titles and branding with escaped literal markup", async () => {
  const site = { title: 'CFGB & <span foreground="red">brand</span> '.repeat(8), baseUrl: "https://example.invalid" };
  const post = { id: "long-title", title: '日本語 中文 한국어 English & <title> '.repeat(16), ogImage: "" };
  const image = await renderSocialImage(post, site, rendererRoot);
  assert.deepEqual([image.width, image.height], [1200, 630]);
  const stats = await sharp(image.bytes).extract({ left: 80, top: 100, width: 1040, height: 330 }).stats();
  assert.ok(stats.channels[0].min < 100, "heading text is present");
  assert.ok(stats.channels[0].max > 200, "heading retains surrounding background");
  const translated = await renderSocialImage({ ...post, id: "translation", title: "A different title" }, site, rendererRoot);
  assert.notDeepEqual(image.bytes, translated.bytes, "fallbacks use each article's title");
});

test("explicit JPEG images retain their oriented dimensions", async () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-social-jpeg-"));
  try {
    mkdirSync(path.join(work, "assets"));
    await sharp({ create: { width: 16, height: 8, channels: 3, background: "#ff00ff" } })
      .withMetadata({ orientation: 6 }).jpeg().toFile(path.join(work, "assets", "photo.jpg"));
    const image = await renderSocialImage({ id: "photo", file: path.join(work, "en.md"), title: "Photograph", ogImage: "./assets/photo.jpg" }, { title: "Example", baseUrl: "https://example.invalid" }, rendererRoot);
    const info = await sharp(image.bytes).metadata();
    assert.deepEqual([info.format, info.width, info.height], ["png", 8, 16]);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});

function article(locale, slug, title, ogImage) {
  return {
    id: `posts/2026/${slug}/${locale}`, group: `2026/${slug}`, year: "2026", articleKey: slug, locale,
    archive: { year: "2026", month: "09" }, body: "Article body.\n",
    data: { title, slug, publishedAt: "2026-09-20T00:00:00Z", updatedAt: "2026-09-21T00:00:00Z", topics: [], summary: 'A summary & <literal> "quotation".', ...(ogImage ? { ogImage } : {}) },
  };
}

function attr(node, name) { return node.attrs?.find((item) => item.name === name)?.value; }
function find(node, predicate) {
  if (predicate(node)) return node;
  for (const child of node.childNodes || []) { const result = find(child, predicate); if (result) return result; }
}
function attribute(document, tag, key, value, name) {
  const node = find(document, (node) => node.tagName === tag && attr(node, key) === value);
  assert.ok(node, `${tag} ${key}=${value}`);
  return attr(node, name);
}

function findFile(root, predicate) {
  for (const entry of readdirSync(root, { withFileTypes: true })) {
    if (predicate(entry.name)) return path.join(root, entry.name);
    if (entry.isDirectory()) { const found = findFile(path.join(root, entry.name), predicate); if (found) return found; }
  }
}
