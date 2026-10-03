import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

let cached;

export function loadSite() {
  if (cached) return cached;
  const configPath = process.env.CFGB_SITE_JSON;
  if (!configPath) throw new Error("CFGB_SITE_JSON is required");
  const site = JSON.parse(readFileSync(configPath, "utf8"));
  const topics = parseTopics(readFileSync(site.topicsFile, "utf8"));
  const posts = discoverPosts(site.contentRoot);
  const prose = discoverProse(site.contentRoot);
  const linkcards = loadLinkcards(site.linkcardsDir);
  const routes = buildRoutes(site, posts, topics);
  cached = { site, topics, posts, prose, linkcards, routes };
  return cached;
}

export function resetSiteCache() {
  cached = undefined;
}

function parseTopics(text) {
  const topics = {};
  let current = null;
  for (const line of text.split(/\r?\n/)) {
    if (!line.trim() || line.trim().startsWith("#")) continue;
    if (!/^\s/.test(line)) {
      current = line.replace(/:.*/, "").trim();
      topics[current] = {};
      continue;
    }
    const match = line.match(/^\s+([a-z]+):\s*(.*)$/);
    if (current && match) topics[current][match[1]] = unquote(match[2]);
  }
  return topics;
}

function unquote(value) {
  const text = value.trim();
  if (
    (text.startsWith('"') && text.endsWith('"')) ||
    (text.startsWith("'") && text.endsWith("'"))
  ) {
    return text.slice(1, -1);
  }
  return text;
}

export function splitFrontmatter(text) {
  const normalized = text.replace(/^\uFEFF/, "");
  if (!normalized.startsWith("---")) return { data: {}, body: normalized };
  const match = normalized.match(/^---\r?\n([\s\S]*?)\r?\n---\r?\n?/);
  if (!match) return { data: {}, body: normalized };
  return { data: parseFrontmatter(match[1]), body: normalized.slice(match[0].length) };
}

function parseFrontmatter(raw) {
  const data = {};
  const lines = raw.split(/\r?\n/);
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i].match(/^([A-Za-z][A-Za-z0-9]*):\s*(.*)$/);
    if (!match) continue;
    const key = match[1];
    const inline = match[2].trim();
    if (!inline) {
      const items = [];
      while (i + 1 < lines.length && /^\s*-\s+/.test(lines[i + 1])) {
        i++;
        items.push(unquote(lines[i].replace(/^\s+-\s+/, "")));
      }
      data[key] = items;
      continue;
    }
    data[key] = unquote(inline);
  }
  return data;
}

function discoverPosts(contentRoot) {
  const postsRoot = path.join(contentRoot, "posts");
  if (!existsSync(postsRoot)) return [];
  const posts = [];
  for (const year of readdirSync(postsRoot)) {
    if (!/^\d{4}$/.test(year)) continue;
    const yearDir = path.join(postsRoot, year);
    if (!statSync(yearDir).isDirectory()) continue;
    for (const articleKey of readdirSync(yearDir)) {
      const groupDir = path.join(yearDir, articleKey);
      if (!statSync(groupDir).isDirectory()) continue;
      for (const locale of ["ja", "en"]) {
        const file = path.join(groupDir, `${locale}.md`);
        if (!existsSync(file)) continue;
        const parsed = splitFrontmatter(readFileSync(file, "utf8"));
        const data = parsed.data;
        if (!data.slug || !data.title || !data.publishedAt) {
          throw new Error(`missing required frontmatter in ${file}`);
        }
        posts.push({
          group: `${year}/${articleKey}`,
          year,
          articleKey,
          locale,
          file,
          title: data.title,
          slug: data.slug,
          publishedAt: data.publishedAt,
          updatedAt: data.updatedAt || "",
          topics: Array.isArray(data.topics) ? data.topics : [],
          summary: data.summary || "",
          ogImage: data.ogImage || "",
          aliases: Array.isArray(data.aliases) ? data.aliases : [],
          url: `/${locale}/posts/${data.slug}/`,
        });
      }
    }
  }
  const seen = new Set();
  for (const post of posts) {
    const id = `${post.locale}:${post.slug}`;
    if (seen.has(id)) throw new Error(`duplicate slug ${post.slug} in ${post.locale}`);
    seen.add(id);
  }
  posts.sort(comparePosts);
  return posts;
}

export function comparePosts(a, b) {
  const left = Date.parse(a.publishedAt);
  const right = Date.parse(b.publishedAt);
  if (left !== right) return right - left;
  return a.articleKey < b.articleKey ? -1 : a.articleKey > b.articleKey ? 1 : 0;
}

function discoverProse(contentRoot) {
  const specs = [
    ["home", "home"],
    ["about", path.join("pages", "about")],
    ["aside", "aside"],
  ];
  const prose = {};
  for (const [kind, dir] of specs) {
    prose[kind] = {};
    for (const locale of ["ja", "en"]) {
      const file = path.join(contentRoot, dir, `${locale}.md`);
      if (!existsSync(file)) continue;
      prose[kind][locale] = splitFrontmatter(readFileSync(file, "utf8")).body;
    }
  }
  return prose;
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
  const date = new Date(iso);
  if (locale === "ja") {
    const parts = new Intl.DateTimeFormat("ja-JP", {
      timeZone,
      year: "numeric",
      month: "numeric",
      day: "numeric",
    }).formatToParts(date);
    const value = (type) => parts.find((part) => part.type === type).value;
    return `${value("year")}年${value("month")}月${value("day")}日`;
  }
  return new Intl.DateTimeFormat("en-GB", {
    timeZone,
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(date);
}

export function absolute(site, route) {
  return site.baseUrl.replace(/\/$/, "") + route;
}

export const ui = {
  ja: {
    skip: "本文へ",
    home: "ホーム",
    posts: "記事",
    archive: "アーカイブ",
    topics: "トピック",
    search: "検索",
    about: "About",
    theme: "テーマ",
    system: "システムに合わせる",
    light: "ライト",
    dark: "ダーク",
    toc: "目次",
    language: "言語",
    sections: "サイト",
    hasTranslation: "対訳があります。",
    openTranslation: "対訳を開く",
    noTranslation: "このページの対訳はありません。",
    latest: "最近の記事",
    postsTitle: "記事",
    archiveTitle: "アーカイブ",
    searchTitle: "検索",
    searchHint: "このサイトの記事を検索します。",
    notFound: "ページが見つかりません",
    notFoundBody: "アドレスを確認するか、ホームまたは検索へ戻ってください。",
    feed: "RSS",
  },
  en: {
    skip: "Skip to content",
    home: "Home",
    posts: "Posts",
    archive: "Archive",
    topics: "Topics",
    search: "Search",
    about: "About",
    theme: "Theme",
    system: "Match system",
    light: "Light",
    dark: "Dark",
    toc: "Contents",
    language: "Language",
    sections: "Site",
    hasTranslation: "A translation exists.",
    openTranslation: "Open translation",
    noTranslation: "This page has no translation.",
    latest: "Latest posts",
    postsTitle: "Posts",
    archiveTitle: "Archive",
    searchTitle: "Search",
    searchHint: "Search articles on this site.",
    notFound: "Page not found",
    notFoundBody: "Check the address, or return home or to search.",
    feed: "RSS",
  },
};
