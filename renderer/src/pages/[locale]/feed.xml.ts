import type { APIRoute } from "astro";
import { posts } from "../../lib/content";
import { absolute, loadSite } from "../../lib/load-site.mjs";

export function getStaticPaths() {
  const { site } = loadSite();
  return Object.keys(site.locales).map((locale) => ({ params: { locale } }));
}

function escapeXml(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

export const GET: APIRoute = async ({ params }) => {
  const locale = params.locale || "";
  const { site } = loadSite();
  const items = (await posts()).filter((post) => post.locale === locale);
  const body = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
<title>${escapeXml(site.title)}</title>
<link>${escapeXml(absolute(site, `/${locale}/`))}</link>
<description>${escapeXml(site.title)}</description>
${items
  .map(
    (post) => `<item>
<title>${escapeXml(post.data.title)}</title>
<link>${escapeXml(absolute(site, post.url))}</link>
<guid>${escapeXml(absolute(site, post.url))}</guid>
<pubDate>${new Date(post.data.publishedAt).toUTCString()}</pubDate>
<description>${escapeXml(post.data.summary || "")}</description>
</item>`,
  )
  .join("\n")}
</channel>
</rss>
`;
  return new Response(body, { headers: { "Content-Type": "application/rss+xml; charset=utf-8" } });
};
