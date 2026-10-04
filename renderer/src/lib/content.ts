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
  const metadata = new Map(loadSite().posts.map((post) => [post.id, post]));
  return loaded
    .map((entry) => {
      const post = metadata.get(entry.id);
      if (!post) throw new Error(`Missing article metadata for ${entry.id}`);
      return {
        ...entry,
        locale: post.locale,
        articleKey: post.articleKey,
        group: post.group,
        url: post.url,
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
