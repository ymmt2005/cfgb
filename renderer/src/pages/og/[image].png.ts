import type { APIRoute } from "astro";
import { loadSite } from "../../lib/load-site.mjs";
import { socialImageFor, socialImageRoute } from "../../lib/social-images.mjs";

export function getStaticPaths() {
  return loadSite().posts.map((post) => ({
    params: { image: socialImageRoute(post).slice("/og/".length, -".png".length) },
    props: { id: post.id },
  }));
}

export const GET: APIRoute = async ({ props }) => {
  const image = await socialImageFor(props.id);
  return new Response(new Uint8Array(image.bytes), { headers: { "Content-Type": image.type } });
};
