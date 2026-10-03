import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import expressiveCode from "astro-expressive-code";
import { unified } from "@astrojs/markdown-remark";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { absolute, loadSite } from "./src/lib/load-site.mjs";
import { remarkCfgb } from "./src/plugins/remark-cfgb.mjs";

const corpus = loadSite();
const { site, posts, routes } = corpus;
const manifest = sitemapManifest(corpus);

const redirects = {};
for (const post of posts) {
  for (const alias of post.aliases) redirects[alias] = post.url;
}

export default defineConfig({
  site: site.baseUrl,
  trailingSlash: "always",
  output: "static",
  redirects,
  integrations: [
    expressiveCode({
      themes: ["github-light", "github-dark"],
      themeCssSelector: (theme) =>
        theme.type === "dark"
          ? '[data-theme="dark"]'
          : ':root:not([data-theme="dark"])',
      useStyleReset: false,
    }),
    sitemap({
      filenameBase: "sitemap",
      entryLimit: 45000,
      filter(page) {
        const route = new URL(page).pathname.replace(/\/$/, "/") ;
        const normalized = route.endsWith("/") || route.endsWith(".xml") || route.endsWith(".txt") ? route : `${route}/`;
        if (normalized.includes("/search/")) return false;
        if (normalized.includes("feed.xml")) return false;
        if (normalized.includes("robots.txt")) return false;
        if (normalized.includes("sitemap")) return false;
        if (normalized.includes("404")) return false;
        return routes.has(normalized) || routes.has(route);
      },
      serialize(item) {
        const route = new URL(item.url).pathname;
        const meta = manifest.get(route);
        if (meta?.lastmod) item.lastmod = meta.lastmod;
        else delete item.lastmod;
        if (meta?.alternates) {
          item.links = Object.entries(meta.alternates).map(([lang, href]) => ({
            lang,
            url: absolute(site, href),
          }));
        }
        return item;
      },
    }),
    fallbackPages(corpus),
  ],
  markdown: {
    processor: unified({
      gfm: true,
      remarkPlugins: [remarkCfgb],
    }),
  },
  vite: {
    resolve: {
      alias: {
        "@cfgb": fileURLToPath(new URL("./src/lib", import.meta.url)),
      },
    },
  },
});

function sitemapManifest({ site, posts, prose }) {
  const map = new Map();
  const locales = Object.keys(site.locales);
  const groups = new Map();
  for (const post of posts) {
    if (!groups.has(post.group)) groups.set(post.group, {});
    groups.get(post.group)[post.locale] = post;
  }
  for (const group of groups.values()) {
    const localesInGroup = Object.keys(group);
    const alternates = localesInGroup.length > 1
      ? Object.fromEntries(localesInGroup.map((locale) => [locale, group[locale].url]))
      : null;
    for (const post of Object.values(group)) {
      map.set(post.url, {
        lastmod: post.updatedAt || post.publishedAt,
        alternates,
      });
    }
  }
  for (const kind of ["home", "about"]) {
    const present = locales.filter((locale) => prose[kind][locale] != null);
    if (present.length < 2) continue;
    const alternates = Object.fromEntries(present.map((locale) => [locale, pageUrl(kind, locale)]));
    for (const locale of present) map.set(pageUrl(kind, locale), { alternates });
  }
  return map;
}

function pageUrl(kind, locale) {
  if (kind === "home") return `/${locale}/`;
  return `/${locale}/about/`;
}

function fallbackPages(corpus) {
  return {
    name: "cfgb-fallbacks",
    hooks: {
      "astro:build:done": ({ dir }) => {
        const root = fileURLToPath(dir);
        if (process.env.CFGB_ROUTES_OUT) {
          writeFileSync(process.env.CFGB_ROUTES_OUT, JSON.stringify([...corpus.routes].sort(), null, 2));
        }
        writeFileSync(path.join(root, "_headers"), headers());
        writeFileSync(path.join(root, "_redirects"), redirectFile(corpus.posts));
        writeFileSync(path.join(root, "404.html"), fallbackHtml(corpus, "both"));
        for (const locale of Object.keys(corpus.site.locales)) {
          const localeDir = path.join(root, locale);
          mkdirSync(localeDir, { recursive: true });
          writeFileSync(path.join(localeDir, "404.html"), fallbackHtml(corpus, locale));
        }
      },
    },
  };
}

function redirectFile(posts) {
  const lines = [];
  for (const post of posts) {
    for (const alias of post.aliases) lines.push(`${alias} ${post.url} 301`);
  }
  return lines.join("\n") + (lines.length ? "\n" : "");
}

function headers() {
  return `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  X-Frame-Options: DENY
  Permissions-Policy: camera=(), microphone=(), geolocation=()
  Content-Security-Policy: default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'
`;
}

function fallbackHtml(corpus, locale) {
  const bilingual = locale === "both";
  const lang = bilingual ? corpus.site.defaultLocale : locale;
  const copy = bilingual
    ? `<p><a href="/ja/">日本語</a> · <a href="/ja/search/">検索</a></p><p><a href="/en/">English</a> · <a href="/en/search/">Search</a></p>`
    : `<p><a href="/${locale}/">${corpus.site.locales[locale].label}</a> · <a href="/${locale}/search/">${locale === "ja" ? "検索" : "Search"}</a></p>`;
  const title = bilingual ? "Page not found" : locale === "ja" ? "ページが見つかりません" : "Page not found";
  return `<!DOCTYPE html>
<html lang="${lang}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title} · ${escapeHtml(corpus.site.title)}</title>
<link rel="stylesheet" href="/assets/site.css">
</head>
<body>
<main id="content" class="wrap page-narrow">
<h1>${title}</h1>
${copy}
</main>
</body>
</html>
`;
}

function escapeHtml(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}
