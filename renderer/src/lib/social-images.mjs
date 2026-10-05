import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import path from "node:path";
import sharp from "sharp";
import { localAsset } from "./content-urls.mjs";
import { absolute, loadSite } from "./load-site.mjs";

const rendered = new WeakMap();

// Stable source identity keeps images separate for translations and literal
// article directory names without putting those names into output filenames.
export function socialImageRoute(post) {
  const name = createHash("sha256").update(post.id).digest("hex");
  return `/og/${name}.png`;
}

export function socialImageFor(id) {
  const { posts, site } = loadSite();
  const post = posts.find((post) => post.id === id);
  if (!post) throw new Error(`Missing social image article: ${id}`);
  if (!rendered.has(post)) rendered.set(post, renderSocialImage(post, site));
  return rendered.get(post);
}

export async function renderSocialImage(post, site, rendererRoot = process.cwd()) {
  try {
    let image;
    if (post.ogImage) {
      // Read the captured content tree through the shared local-asset resolver.
      // Sharp decodes the source format, including SVG, and emits crawler-safe PNG.
      const asset = localAsset(post.ogImage, post.file);
      if (!asset) throw new Error(`Expected a local ./assets/ OG image: ${post.ogImage}`);
      image = await sharp(readFileSync(asset.filename)).rotate().png().toBuffer({ resolveWithObject: true });
    } else {
      image = await fallbackImage(post.title, site, rendererRoot);
    }
    return {
      bytes: image.data,
      url: absolute(site, socialImageRoute(post)),
      width: image.info.width,
      height: image.info.height,
      alt: post.title,
      type: "image/png",
    };
  } catch (cause) {
    throw new Error(`Render OG image for ${post.id}: ${cause.message}`, { cause });
  }
}

export async function faviconImage(site, size) {
  return sharp(readFileSync(site.image)).rotate().resize(size, size, {
    fit: "contain", background: { r: 0, g: 0, b: 0, alpha: 0 },
  }).png().toBuffer();
}

async function fallbackImage(title, site, rendererRoot) {
  // CLI builds and Astro run with the extracted renderer as their cwd. Keep
  // fonts in source assets so they are embedded but never copied to the site.
  const fontfile = path.join(rendererRoot, "src/lib/fonts/NotoSansCJKjp-Regular.otf");
  const text = async (value, colour, size, width, height) => {
    const options = {
      text: `<span foreground="${colour}">${escapeText(value)}</span>`,
      font: `Noto Sans CJK JP ${size}`,
      fontfile,
      width,
      rgba: true,
      wrap: "word-char",
    };
    const natural = await sharp({ text: options }).png().toBuffer({ resolveWithObject: true });
    // Shrink overflowing titles/branding without enlarging short text.
    if (natural.info.height <= height) return natural.data;
    return sharp({ text: { ...options, height } }).png().toBuffer();
  };
  const branding = site.title.trim() ? site.title : "CFGB";
  const heading = await text(title.trim() ? title : branding, "#1c1915", 64, 1040, 330);
  const brand = await text(branding, "#8a3c18", 32, site.image ? 880 : 1040, 60);
  const layers = [
    { input: heading, left: 80, top: 100 },
    { input: brand, left: site.image ? 240 : 80, top: 490 },
  ];
  if (site.image) layers.push({ input: await faviconImage(site, 128), left: 80, top: 462 });
  return sharp({ create: { width: 1200, height: 630, channels: 3, background: "#f3f0e8" } })
    .composite(layers)
    .png()
    .toBuffer({ resolveWithObject: true });
}

function escapeText(value) {
  return value.replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
}
