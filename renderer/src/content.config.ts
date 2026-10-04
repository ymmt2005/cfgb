import { defineCollection, z } from "astro:content";
import { cfgbLoader } from "./lib/content-loader.mjs";
import { loadSite } from "./lib/load-site.mjs";

const { posts, prose } = loadSite();

// Go decodes metadata into typed fields. This shape supplies Astro collection
// types without imposing extra front-matter constraints.
const article = z.object({
  title: z.string(),
  slug: z.string(),
  publishedAt: z.string(),
  updatedAt: z.string().optional(),
  topics: z.array(z.string()).default([]),
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
