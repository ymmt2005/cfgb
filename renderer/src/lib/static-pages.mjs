import { mkdirSync, writeFileSync } from "node:fs";
import path from "node:path";
import { sitePath } from "./site-path.mjs";

// Static hosts have no locale Worker or _redirects interpreter. Emit native
// links and small redirect documents; do not index them as article content.
export function staticPages(root, { site, posts }) {
  const home = sitePath(site, `/${site.defaultLocale}/`);
  writeFileSync(path.join(root, "index.html"), redirectHtml(site, home));
  for (const post of posts) {
    for (const alias of post.aliases) {
      const pathname = new URL(alias, "https://static.invalid").pathname;
      const relative = path.posix
        .normalize("/" + decodeURIComponent(pathname))
        .replace(/^\/+/, "");
      const file = path.join(
        root,
        relative,
        pathname.endsWith("/") ? "index.html" : "",
      );
      mkdirSync(path.dirname(file), { recursive: true });
      writeFileSync(file, redirectHtml(site, sitePath(site, post.url)));
    }
  }
}

function redirectHtml(site, target) {
  const escaped = escapeHtml(target);
  return `<!doctype html><html lang="${escapeHtml(site.defaultLocale)}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta http-equiv="refresh" content="0;url=${escaped}"><title>${escapeHtml(site.title)}</title><link rel="canonical" href="${escapeHtml(new URL(target, site.baseUrl).href)}"></head><body data-pagefind-ignore="all"><a href="${escaped}">${escapeHtml(site.title)}</a></body></html>\n`;
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}
