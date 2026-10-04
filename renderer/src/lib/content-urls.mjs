import path from "node:path";

export function splitUrl(url) {
  const index = url.search(/[?#]/);
  return index < 0
    ? { pathname: url, suffix: "" }
    : { pathname: url.slice(0, index), suffix: url.slice(index) };
}

// Local assets name files, not requests. Markdown article links can carry a
// heading fragment; public/root-relative and remote URLs are left alone.
export function localAsset(url, source) {
  if (!source || !url.startsWith("./assets/")) return null;
  const { pathname, suffix } = splitUrl(url);
  if (suffix)
    throw new Error(
      `Local asset references must not contain a query or fragment: ${url}`,
    );
  const parts = pathname
    .slice("./assets/".length)
    .split("/")
    .map((part) => decodeURIComponent(part));
  if (parts.some((part) => /[/\\\0]/.test(part)))
    throw new Error(`Invalid local asset path: ${url}`);
  const assetRoot = path.join(path.dirname(source), "assets");
  const filename = path.resolve(assetRoot, ...parts);
  const relative = path.relative(assetRoot, filename);
  if (
    !relative ||
    relative === ".." ||
    relative.startsWith(`..${path.sep}`) ||
    path.isAbsolute(relative)
  ) {
    throw new Error(`Local asset escapes its assets directory: ${url}`);
  }
  return {
    filename,
    encodedPath: relative.split(path.sep).map(encodeURIComponent).join("/"),
  };
}

export function contentUrls(corpus) {
  const sources = new Map();
  for (const post of corpus.posts)
    sources.set(path.resolve(post.file), {
      url: post.url,
      media: `/media/${post.group.split("/").map(encodeURIComponent).join("/")}/`,
    });
  for (const entry of corpus.prose)
    sources.set(path.resolve(entry.file), { media: `/media/${entry.kind}/` });

  const media = (url, source) => {
    const asset = localAsset(url, source);
    if (!asset) return url;
    const entry = sources.get(source);
    if (!entry) throw new Error(`Missing source metadata for ${source}`);
    return entry.media + asset.encodedPath;
  };
  return {
    media,
    link(url, source) {
      if (url.startsWith("./assets/")) return media(url, source);
      const { pathname, suffix } = splitUrl(url);
      if (!source || !pathname || /^(?:[a-z][a-z0-9+.-]*:|\/)/i.test(pathname))
        return url;
      let decoded;
      try {
        decoded = decodeURIComponent(pathname);
      } catch {
        return url;
      }
      if (!decoded.endsWith(".md")) return url;
      const target = sources.get(path.resolve(path.dirname(source), decoded));
      return target?.url ? target.url + suffix : url;
    },
  };
}
