import { createHash } from "node:crypto";
import {
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  writeFileSync,
} from "node:fs";
import path from "node:path";
import { parse } from "parse5";

const imageExtensions = new Set([
  ".jpeg",
  ".jpg",
  ".png",
  ".apng",
  ".tiff",
  ".webp",
  ".gif",
  ".svg",
  ".avif",
]);

// Vite import specifiers cannot represent every valid source filename, and
// Astro's content-image imports require an extension without a URL suffix.
// Stage a safe, content-addressed import in the toolchain workspace. Originals
// and public-media URLs stay unchanged.
export function imageWorkspace(rendererRoot) {
  const cache = path.join(rendererRoot, ".astro", "cfgb-images");
  return function prepareImage(url, source) {
    if (!source || !url.startsWith("./assets/")) return null;
    const split = url.search(/[?#]/);
    const pathname = split < 0 ? url : url.slice(0, split);
    const suffix = split < 0 ? "" : url.slice(split);
    const decoded = pathname
      .split("/")
      .map((part) => decodeURIComponent(part))
      .join("/");
    const original = path.resolve(path.dirname(source), decoded);
    const assetRoot = path.join(path.dirname(source), "assets");
    const relative = path.relative(assetRoot, original);
    if (
      relative === ".." ||
      relative.startsWith(`..${path.sep}`) ||
      path.isAbsolute(relative)
    ) {
      throw new Error(`Local image escapes its assets directory: ${url}`);
    }
    const extension = path.extname(original).toLowerCase();
    if (!imageExtensions.has(extension))
      throw new Error(`Unsupported local image format: ${url}`);
    const bytes = readFileSync(original);
    const digest = createHash("sha256").update(bytes).digest("hex");
    const staged = path.join(cache, `${digest}${extension}`);
    mkdirSync(cache, { recursive: true });
    if (!existsSync(staged)) writeFileSync(staged, bytes);
    return {
      url: path
        .relative(path.dirname(source), staged)
        .split(path.sep)
        .join("/"),
      suffix,
    };
  };
}

// Run after Astro has replaced content-image markers with processed images.
// The temporary attribute carries the authored suffix across that replacement.
export function restoreImageSuffixes(html) {
  if (
    !html.includes("data-cfgb-image-suffix") &&
    !html.includes("__ASTRO_IMAGE_")
  )
    return html;
  const document = parse(html, { sourceCodeLocationInfo: true });
  const edits = [];
  walk(document, (node) => {
    if (node.tagName !== "img") return;
    if (node.attrs?.some((attr) => attr.name === "__astro_image_")) {
      throw new Error(
        "Astro left an unresolved Markdown image in the rendered page",
      );
    }
    const suffix = node.attrs?.find(
      (attr) => attr.name === "data-cfgb-image-suffix",
    );
    if (!suffix) return;
    const locations = node.sourceCodeLocation?.attrs;
    if (!locations?.[suffix.name]) return;
    const src = node.attrs.find((attr) => attr.name === "src");
    if (!src) throw new Error("Processed Markdown image has no src");
    for (const attr of node.attrs) {
      if (attr.name !== "src" && attr.name !== "srcset") continue;
      const location = locations[attr.name];
      if (!location) continue;
      const value =
        attr.name === "src"
          ? attr.value + suffix.value
          : attr.value
              .split(",")
              .map((candidate) =>
                candidate
                  .trim()
                  .replace(
                    /^(\S+)/,
                    (url) => url + suffix.value.replaceAll(",", "%2C"),
                  ),
              )
              .join(", ");
      edits.push({
        start: location.startOffset,
        end: location.endOffset,
        value: `${attr.name}="${escapeAttribute(value)}"`,
      });
    }
    edits.push({
      start: locations[suffix.name].startOffset,
      end: locations[suffix.name].endOffset,
      value: "",
    });
  });
  edits.sort((left, right) => right.start - left.start);
  for (const edit of edits)
    html = html.slice(0, edit.start) + edit.value + html.slice(edit.end);
  return html;
}

export function finishImageUrls(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) finishImageUrls(filename);
    else if (entry.isFile() && entry.name.endsWith(".html")) {
      const html = readFileSync(filename, "utf8");
      const updated = restoreImageSuffixes(html);
      if (updated !== html) writeFileSync(filename, updated);
    }
  }
}

function escapeAttribute(value) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function walk(node, visit) {
  visit(node);
  for (const child of node.childNodes || []) walk(child, visit);
}
