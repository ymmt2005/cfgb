// Sitemap membership is based on the route's role. Slugs may contain words
// such as "404" or "sitemap" without dropping the article.
export function canonicalSitemapPath(pathname) {
  if (pathname === "/robots.txt" || pathname === "/404.html" || pathname.endsWith("/404.html")) return false;
  if (pathname.endsWith("/feed.xml")) return false;
  if (/^\/sitemap(?:-index|-\d+)?\.xml$/.test(pathname)) return false;
  const parts = pathname.split("/").filter(Boolean);
  if (parts.length === 2 && parts[1] === "search") return false;
  return true;
}

export function pagePath(kind, locale) {
  if (kind === "home") return `/${locale}/`;
  return `/${locale}/about/`;
}

// Home and about exist for every configured locale, whether or not optional
// prose files were authored.
export function localePageAlternates(locales) {
  const alternates = {};
  for (const kind of ["home", "about"]) {
    alternates[kind] = Object.fromEntries(locales.map((locale) => [locale, pagePath(kind, locale)]));
  }
  return alternates;
}
