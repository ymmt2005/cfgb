import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";
import expressiveCode from "astro-expressive-code";
import { unified } from "@astrojs/markdown-remark";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { fallbackHtml } from "./src/lib/fallback-html.mjs";
import { absolute, loadSite } from "./src/lib/load-site.mjs";
import { alternateMembers, groupCounterparts } from "./src/lib/locale-link.mjs";
import { canonicalSitemapPath, localePageAlternates, pagePath } from "./src/lib/sitemap.mjs";
import { remarkCfgb } from "./src/plugins/remark-cfgb.mjs";
import { finishImageUrls, imageWorkspace } from "./src/lib/image-paths.mjs";

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
        theme.type === "dark" ? '[data-theme="dark"]' : '[data-theme="light"]',
      useStyleReset: false,
    }),
    sitemap({
      filenameBase: "sitemap",
      entryLimit: 45000,
      filter(page) {
        const pathname = new URL(page).pathname;
        if (!canonicalSitemapPath(pathname)) return false;
        const directory = pathname.endsWith("/") ? pathname : `${pathname}/`;
        return routes.has(pathname) || routes.has(directory);
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
      remarkPlugins: [[remarkCfgb, { prepareImage: imageWorkspace(fileURLToPath(new URL(".", import.meta.url))) }]],
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

function sitemapManifest({ site, posts }) {
  const map = new Map();
  const locales = Object.keys(site.locales);
  const groups = new Map();
  for (const post of posts) {
    if (!groups.has(post.group)) groups.set(post.group, []);
    groups.get(post.group).push(post);
  }
  for (const [group, members] of groups) {
    const counterparts = groupCounterparts(members, group);
    const alternates = alternateMembers(counterparts);
    const links = alternates.length
      ? Object.fromEntries(alternates.map((item) => [item.locale, item.href]))
      : null;
    for (const post of members) {
      map.set(post.url, {
        lastmod: post.updatedAt || post.publishedAt,
        alternates: links,
      });
    }
  }
  if (locales.length > 1) {
    const pages = localePageAlternates(locales);
    for (const kind of ["home", "about"]) {
      for (const locale of locales) map.set(pagePath(kind, locale), { alternates: pages[kind] });
    }
  }
  return map;
}

function fallbackPages(corpus) {
  return {
    name: "cfgb-fallbacks",
    hooks: {
      "astro:build:done": ({ dir }) => {
        const root = fileURLToPath(dir);
        finishImageUrls(root);
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
  // Pagefind 1.5.2 instantiates WebAssembly and starts the same-origin
  // pagefind/pagefind-worker.js. script-src therefore includes
  // 'wasm-unsafe-eval'. That release does not create a blob worker, so the
  // script-src fallback covers the worker and blob: is not required.
  // img-src includes https: because remote article images stay external.
  return `/*
  X-Content-Type-Options: nosniff
  Referrer-Policy: strict-origin-when-cross-origin
  X-Frame-Options: DENY
  Permissions-Policy: camera=(), microphone=(), geolocation=()
  Content-Security-Policy: default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; script-src 'self' 'wasm-unsafe-eval'
`;
}
