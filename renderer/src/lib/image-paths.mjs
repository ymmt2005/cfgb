import { createHash } from "node:crypto";
import { existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { localAsset } from "./content-urls.mjs";

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

// Vite import specifiers cannot represent every valid source filename.
// Stage a safe, content-addressed import in the toolchain workspace. Originals
// and public-media URLs stay unchanged.
export function imageWorkspace(rendererRoot) {
  const staging = path.join(rendererRoot, ".astro", "cfgb-images");
  return function prepareImage(url, source) {
    const asset = localAsset(url, source);
    if (!asset) return null;
    const original = asset.filename;
    const extension = path.extname(original).toLowerCase();
    if (!imageExtensions.has(extension))
      throw new Error(`Unsupported local image format: ${url}`);
    const bytes = readFileSync(original);
    const digest = createHash("sha256").update(bytes).digest("hex");
    const staged = path.join(staging, `${digest}${extension}`);
    mkdirSync(staging, { recursive: true });
    if (!existsSync(staged)) writeFileSync(staged, bytes);
    return {
      url: path
        .relative(path.dirname(source), staged)
        .split(path.sep)
        .join("/"),
    };
  };
}
