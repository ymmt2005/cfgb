import { getCollection, render, type CollectionEntry } from "astro:content";
import { archiveParts, comparePosts, loadSite } from "./load-site.mjs";

export type PostEntry = CollectionEntry<"posts"> & {
  locale: string;
  articleKey: string;
  group: string;
  url: string;
};

export async function posts(): Promise<PostEntry[]> {
  const loaded = await getCollection("posts");
  return loaded
    .map((entry) => {
      const parts = entry.id.split("/");
      const locale = parts.at(-1) || "";
      const articleKey = parts.at(-2) || "";
      const year = parts.at(-3) || "";
      return {
        ...entry,
        locale,
        articleKey,
        group: `${year}/${articleKey}`,
        url: `/${locale}/posts/${entry.data.slug}/`,
      };
    })
    .sort((a, b) =>
      comparePosts(
        { publishedAt: a.data.publishedAt, articleKey: a.articleKey },
        { publishedAt: b.data.publishedAt, articleKey: b.articleKey },
      ),
    );
}

export async function proseEntry(kind: "home" | "about" | "aside", locale: string) {
  const id = kind === "about" ? `pages/about/${locale}` : `${kind}/${locale}`;
  const entries = await getCollection("prose");
  return entries.find((entry) => entry.id === id) ?? null;
}

export async function renderedProse(kind: "home" | "about" | "aside", locale: string) {
  const entry = await proseEntry(kind, locale);
  if (!entry) return null;
  return render(entry);
}

export function monthOf(iso: string) {
  const { site } = loadSite();
  return archiveParts(iso, site.timezone);
}

export function alternateFor(all: PostEntry[], post: PostEntry) {
  return all.find((item) => item.group === post.group && item.locale !== post.locale) ?? null;
}
