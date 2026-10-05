import type { APIRoute } from "astro";
import { loadSite } from "../lib/load-site.mjs";
import { faviconImage } from "../lib/social-images.mjs";

export function getStaticPaths() {
  if (!loadSite().site.image) return [];
  return [
    { params: { icon: "favicon" }, props: { size: 32 } },
    { params: { icon: "apple-touch-icon" }, props: { size: 180 } },
  ];
}

export const GET: APIRoute = async ({ props }) => {
  const bytes = await faviconImage(loadSite().site, props.size);
  return new Response(new Uint8Array(bytes), { headers: { "Content-Type": "image/png" } });
};
