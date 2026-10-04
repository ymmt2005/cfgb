import { defineCollection, z } from "astro:content";
import { cfgbLoader } from "./lib/content-loader.mjs";
import { loadSite } from "./lib/load-site.mjs";

const { posts, prose } = loadSite();

// Go validates article.schema.json before emitting metadata. This shape
// supplies Astro collection types; structural rules live in that schema.
const article = z.object({
  title: z.string().min(1),
  slug: z.string().min(1),
  publishedAt: z.string().min(1),
  updatedAt: z.string().min(1).optional(),
  topics: z.array(z.string()).min(1),
  summary: z.string().optional(),
  ogImage: z.string().optional(),
  aliases: z.array(z.string()).optional(),
});

export const collections = {
  posts: defineCollection({
    loader: cfgbLoader(
      "cfgb-posts",
      posts.map((post) => ({
        id: post.id,
        file: post.file,
        body: post.body,
        data: post.data,
      })),
    ),
    schema: article,
  }),
  prose: defineCollection({
    loader: cfgbLoader(
      "cfgb-prose",
      prose.map((entry) => ({
        id: entry.id,
        file: entry.file,
        body: entry.body,
        data: {},
      })),
    ),
  }),
};
