import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import catalog from "./locales.json" with { type: "json" };

const localeIdentifier = new RegExp(catalog.identifier);

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
    if (seen.has(id)) throw new Error(`duplicate slug ${post.slug} in ${post.locale}`);
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
  if (!data || typeof data !== "object" || Array.isArray(data)) {
    throw new Error(`article metadata must be a mapping in ${post.file || post.id}`);
  }
  for (const key of ["title", "slug", "publishedAt"]) {
    if (typeof data[key] !== "string" || !data[key]) {
      throw new Error(`${key} is required in ${post.file || post.id}`);
    }
  }
  if (!Array.isArray(data.topics) || data.topics.some((topic) => typeof topic !== "string")) {
    throw new Error(`topics must be an array of strings in ${post.file || post.id}`);
  }
  if (data.aliases !== undefined && (!Array.isArray(data.aliases) || data.aliases.some((alias) => typeof alias !== "string"))) {
    throw new Error(`aliases must be an array of strings in ${post.file || post.id}`);
  }
  return {
    ...post,
    title: data.title,
    slug: data.slug,
    publishedAt: data.publishedAt,
    updatedAt: data.updatedAt || "",
    topics: data.topics,
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
  return a.articleKey < b.articleKey ? -1 : a.articleKey > b.articleKey ? 1 : 0;
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
      const parts = archiveParts(post.publishedAt, site.timezone);
      months.add(`${parts.year}/${parts.month}`);
      for (const topic of post.topics) usedTopics.add(topic);
    }
    for (const month of months) routes.add(`/${locale}/archive/${month}/`);
    for (const topic of usedTopics) {
      if (topics[topic]) routes.add(`/${locale}/topics/${topic}/`);
    }
  }
  routes.add("/robots.txt");
  routes.add("/sitemap-index.xml");
  routes.add("/sitemap-0.xml");
  return routes;
}

export function archiveParts(iso, timeZone) {
  const parts = new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
  }).formatToParts(new Date(iso));
  const year = parts.find((part) => part.type === "year").value;
  const month = parts.find((part) => part.type === "month").value;
  return { year, month };
}

export function formatDate(iso, locale, timeZone) {
  const entry = localeEntry(locale);
  const date = new Date(iso);
  if (entry.date.form === "ymd-kanji") {
    const parts = new Intl.DateTimeFormat(entry.date.intl, {
      timeZone,
      year: "numeric",
      month: "numeric",
      day: "numeric",
    }).formatToParts(date);
    const value = (type) => parts.find((part) => part.type === type).value;
    return `${value("year")}年${value("month")}月${value("day")}日`;
  }
  if (entry.date.form === "intl-medium") {
    return new Intl.DateTimeFormat(entry.date.intl, {
      timeZone,
      day: "numeric",
      month: "short",
      year: "numeric",
    }).format(date);
  }
  throw new Error(`locale ${locale} has no date form`);
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

function localeEntry(locale) {
  const entry = catalog.locales[locale];
  if (!entry) throw new Error(`locale ${locale} is not supported`);
  return entry;
}

function assertConfiguredLocales(site) {
  const locales = site.locales;
  if (!locales || typeof locales !== "object" || Array.isArray(locales) || Object.keys(locales).length === 0) {
    throw new Error("locales must be a nonempty map");
  }
  for (const [locale, item] of Object.entries(locales)) {
    if (!localeIdentifier.test(locale)) throw new Error(`locale ${locale} is not a path-safe language tag`);
    if (!catalog.locales[locale]) throw new Error(`locale ${locale} is not supported`);
    if (typeof item?.label !== "string" || !item.label.trim()) throw new Error(`locale ${locale} requires a nonempty label`);
  }
  if (!Object.hasOwn(locales, site.defaultLocale)) {
    throw new Error(`defaultLocale ${site.defaultLocale} is not one of the configured locales`);
  }
}

export function absolute(site, route) {
  return site.baseUrl.replace(/\/$/, "") + route;
}
