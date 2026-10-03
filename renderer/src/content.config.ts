import { defineCollection, z } from "astro:content";
import { glob } from "astro/loaders";
import { loadSite } from "./lib/load-site.mjs";

const { site } = loadSite();

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
    loader: glob({
      pattern: "posts/*/*/{ja,en}.md",
      base: site.contentRoot,
      generateId: ({ entry }) => entry.replace(/\.md$/, ""),
    }),
    schema: article,
  }),
  prose: defineCollection({
    loader: glob({
      pattern: ["home/{ja,en}.md", "pages/about/{ja,en}.md", "aside/{ja,en}.md"],
      base: site.contentRoot,
      generateId: ({ entry }) => entry.replace(/\.md$/, ""),
    }),
  }),
};
