// Called by Go corpus acceptance after the real CLI build. Resolve dependencies
// from its retained toolchain, so the source checkout needs no node_modules.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";
import { pathToFileURL } from "node:url";

const require = createRequire(path.join(process.argv[2], "package.json"));
const [{ parse }, { default: sharp }] = await Promise.all([
  import(pathToFileURL(require.resolve("parse5")).href),
  import(pathToFileURL(require.resolve("sharp")).href),
]);
const expected = JSON.parse(readFileSync(0, "utf8"));
const absolute = (route) => expected.baseURL.replace(/\/$/, "") + route;
const images = new Set();

for (const article of expected.articles) {
  const canonical = absolute(article.url);
  const document = parse(readFileSync(path.join(expected.siteRoot, article.url, "index.html"), "utf8"));
  const html = document.childNodes.find((node) => node.tagName === "html");
  assert.equal(attr(html, "lang"), article.locale, canonical);
  const head = html.childNodes.find((node) => node.tagName === "head");
  const nodes = head.childNodes;
  const single = (tag, key, value) => {
    const found = nodes.filter((node) => node.tagName === tag && (!key || attr(node, key) === value));
    assert.equal(found.length, 1, `${canonical}: expected one ${tag} ${key || ""}=${value || ""}`);
    return found[0];
  };
  const meta = (name) => attr(single("meta", name.startsWith("twitter:") || name === "description" ? "name" : "property", name), "content");
  assert.equal(text(single("title")), `${article.data.title} · ${expected.title}`, canonical);
  assert.equal(attr(single("link", "rel", "canonical"), "href"), canonical);
  assert.equal(meta("description"), article.data.summary);
  assert.equal(meta("og:url"), canonical);
  assert.equal(meta("og:type"), "article");
  assert.equal(meta("og:site_name"), expected.title);
  for (const name of ["og:title", "twitter:title"]) assert.equal(meta(name), article.data.title, `${canonical}: ${name}`);
  for (const name of ["og:description", "twitter:description"]) assert.equal(meta(name), article.data.summary, `${canonical}: ${name}`);
  assert.equal(meta("twitter:card"), "summary_large_image");

  const alternates = {};
  for (const node of nodes.filter((node) => node.tagName === "link" && attr(node, "rel") === "alternate" && attr(node, "hreflang"))) {
    const locale = attr(node, "hreflang");
    assert.equal(Object.hasOwn(alternates, locale), false, `${canonical}: duplicate alternate ${locale}`);
    alternates[locale] = attr(node, "href");
  }
  assert.deepEqual(alternates, Object.fromEntries(Object.entries(article.alternates).map(([locale, route]) => [locale, absolute(route)])), `${canonical}: translation alternates`);
  assert.equal(attr(single("link", "type", "application/rss+xml"), "href"), new URL(absolute(`/${article.locale}/feed.xml`)).pathname);

  const published = new Date(article.data.publishedAt).toISOString();
  const modified = new Date(article.data.updatedAt || article.data.publishedAt).toISOString();
  assert.equal(new Date(meta("article:published_time")).toISOString(), published, canonical);
  assert.equal(new Date(meta("article:modified_time")).toISOString(), modified, canonical);
  const image = meta("og:image");
  const url = new URL(image);
  assert.ok(image.startsWith(absolute("/og/")) && image.endsWith(".png"), `${canonical}: canonical PNG URL`);
  assert.equal(url.search + url.hash, "");
  assert.equal(images.has(image), false, `${canonical}: image reused across articles/locales`);
  images.add(image);
  assert.equal(meta("twitter:image"), image);
  assert.equal(meta("og:image:type"), "image/png");
  for (const name of ["og:image:alt", "twitter:image:alt"]) assert.equal(meta(name), article.data.title, `${canonical}: ${name}`);
  const prefix = new URL(expected.baseURL).pathname.replace(/\/$/, "");
  const bytes = readFileSync(path.join(expected.siteRoot, url.pathname.slice(prefix.length)));
  assert.equal((await sharp(bytes).metadata()).format, "png", image);
  const { info } = await sharp(bytes).raw().toBuffer({ resolveWithObject: true });
  assert.equal(meta("og:image:width"), String(info.width), image);
  assert.equal(meta("og:image:height"), String(info.height), image);
  if (article.data.ogImage) {
    const source = path.resolve(expected.root, path.dirname(article.path), decodeURIComponent(article.data.ogImage));
    const input = await sharp(source).metadata();
    const dimensions = input.orientation >= 5 ? [input.height, input.width] : [input.width, input.height];
    assert.deepEqual([info.width, info.height], dimensions, `${canonical}: explicit image dimensions`);
  } else {
    assert.deepEqual([info.width, info.height], [1200, 630], `${canonical}: fallback image dimensions`);
  }

  const data = JSON.parse(text(single("script", "type", "application/ld+json")));
  assert.equal(data["@context"], "https://schema.org");
  assert.equal(data["@type"], "BlogPosting");
  assert.equal(data.headline, article.data.title);
  assert.equal(data.description, article.data.summary);
  assert.equal(data.inLanguage, article.locale);
  assert.equal(data.mainEntityOfPage, canonical);
  assert.equal(data.image, image);
  assert.equal(new Date(data.datePublished).toISOString(), published);
  assert.equal(new Date(data.dateModified).toISOString(), modified);
}

function attr(node, name) {
  return node.attrs?.find((item) => item.name === name)?.value;
}

function text(node) {
  return (node.childNodes || []).map((child) => child.value || text(child)).join("");
}
