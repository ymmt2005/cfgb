import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import catalog from "./locales.json" with { type: "json" };
import { localeEntry } from "./dates.mjs";
export { absolute, basePath, sitePath } from "./site-path.mjs";
export { formatDate } from "./dates.mjs";

let cached;

export function loadSite() {
  if (cached) return cached;
  const configPath = process.env.CFGB_SITE_JSON;
  if (!configPath) throw new Error("CFGB_SITE_JSON is required");
  const site = JSON.parse(readFileSync(configPath, "utf8"));
  assertConfiguredLocales(site);
  if (!site.metadataFile) throw new Error("metadataFile is required");
  const index = JSON.parse(readFileSync(site.metadataFile, "utf8"));
  const topics = index.topics || {};
  const posts = (index.posts || []).map(normalizePost);
  const seen = new Set();
  for (const post of posts) {
    const id = `${post.locale}:${post.slug}`;
    if (seen.has(id))
      throw new Error(`duplicate slug ${post.slug} in ${post.locale}`);
    seen.add(id);
  }
  posts.sort(comparePosts);
  const prose = index.prose || [];
  const linkcards = loadLinkcards(site.linkcardsDir);
  const routes = buildRoutes(site, posts, topics);
  cached = { site, topics, posts, prose, linkcards, routes };
  return cached;
}

export function resetSiteCache() {
  cached = undefined;
}

function normalizePost(post) {
  const data = post.data;
  return {
    ...post,
    title: data.title,
    slug: data.slug,
    publishedAt: data.publishedAt,
    updatedAt: data.updatedAt || "",
    topics: data.topics || [],
    summary: data.summary || "",
    ogImage: data.ogImage || "",
    aliases: data.aliases || [],
    url: `/${post.locale}/posts/${data.slug}/`,
  };
}

export function comparePosts(a, b) {
  const left = Date.parse(a.publishedAt);
  const right = Date.parse(b.publishedAt);
  if (left !== right) return right - left;
  if (a.articleKey !== b.articleKey)
    return a.articleKey < b.articleKey ? -1 : 1;
  return a.group < b.group ? -1 : a.group > b.group ? 1 : 0;
}

function loadLinkcards(dir) {
  const cards = new Map();
  if (!dir || !existsSync(dir)) return cards;
  for (const name of readdirSync(dir)) {
    if (!name.endsWith(".json")) continue;
    const card = JSON.parse(readFileSync(path.join(dir, name), "utf8"));
    if (card.url) cards.set(cardKey(card.url), card);
  }
  return cards;
}

export function cardKey(url) {
  const trimmed = url.trim().split("#")[0];
  return createHash("sha256").update(trimmed, "utf8").digest("hex");
}

function buildRoutes(site, posts, topics) {
  const routes = new Set();
  for (const locale of Object.keys(site.locales)) {
    routes.add(`/${locale}/`);
    routes.add(`/${locale}/posts/`);
    routes.add(`/${locale}/archive/`);
    routes.add(`/${locale}/search/`);
    routes.add(`/${locale}/about/`);
    routes.add(`/${locale}/feed.xml`);
  }
  for (const post of posts) routes.add(post.url);
  for (const locale of Object.keys(site.locales)) {
    const months = new Set();
    const usedTopics = new Set();
    for (const post of posts.filter((item) => item.locale === locale)) {
      const parts = post.archive;
      months.add(`${parts.year}/${parts.month}`);
      for (const topic of post.topics) usedTopics.add(topic);
    }
    for (const month of months) routes.add(`/${locale}/archive/${month}/`);
    for (const topic of usedTopics) {
      if (Object.hasOwn(topics, topic)) routes.add(`/${locale}/topics/${topic}/`);
    }
  }
  routes.add("/robots.txt");
  routes.add("/sitemap-index.xml");
  routes.add("/sitemap-0.xml");
  return routes;
}

export function archiveTitle(year, month, locale) {
  const entry = localeEntry(locale);
  if (entry.date.archive === "ymd-kanji") return `${year}年${Number(month)}月`;
  if (entry.date.archive === "iso-month") return `${year}-${month}`;
  throw new Error(`locale ${locale} has no archive title`);
}

export function copyFor(locale) {
  return localeEntry(locale).ui;
}

export function openGraphLocale(locale) {
  return localeEntry(locale).ogLocale;
}

export function rootNotFoundTitle() {
  return catalog.rootNotFound;
}

function assertConfiguredLocales(site) {
  const locales = site.locales;
  if (
    !locales ||
    typeof locales !== "object" ||
    Array.isArray(locales) ||
    Object.keys(locales).length === 0
  ) {
    throw new Error("locales must be a nonempty map");
  }
  for (const [locale, item] of Object.entries(locales)) {
    if (!Object.hasOwn(catalog.locales, locale))
      throw new Error(`locale ${locale} is not supported`);
    if (typeof item?.label !== "string" || !item.label.trim())
      throw new Error(`locale ${locale} requires a nonempty label`);
  }
  if (!Object.hasOwn(locales, site.defaultLocale)) {
    throw new Error(
      `defaultLocale ${site.defaultLocale} is not one of the configured locales`,
    );
  }
}
