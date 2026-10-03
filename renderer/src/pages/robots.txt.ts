import type { APIRoute } from "astro";
import { absolute, loadSite } from "../lib/load-site.mjs";

export const GET: APIRoute = () => {
  const { site } = loadSite();
  const body = `User-agent: *\nAllow: /\n\nSitemap: ${absolute(site, "/sitemap-index.xml")}\n`;
  return new Response(body, { headers: { "Content-Type": "text/plain; charset=utf-8" } });
};
