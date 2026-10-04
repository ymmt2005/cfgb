import assert from "node:assert/strict";
import {
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { imageWorkspace } from "../src/lib/image-paths.mjs";
import { contentUrls, localAsset } from "../src/lib/content-urls.mjs";

const corpus = {
  posts: [
    {
      file: path.resolve("input/source/en.md"),
      group: "2026/source",
      url: "/en/posts/source/",
    },
    {
      file: path.resolve("input/target/en.md"),
      group: "2026/target",
      url: "/en/posts/different-slug/",
    },
  ],
  prose: [{ file: path.resolve("input/intro/en.md"), kind: "home" }],
};
const source = corpus.posts[0].file;

test("source metadata owns media scopes and article routes", () => {
  const urls = contentUrls(corpus);
  assert.equal(
    urls.media("./assets/a%20b.svg", source),
    "/media/2026/source/a%20b.svg",
  );
  assert.equal(
    urls.media("./assets/a%2526b.svg", source),
    "/media/2026/source/a%2526b.svg",
  );
  assert.equal(
    urls.media("./assets/nested/../picture.svg", source),
    "/media/2026/source/picture.svg",
  );
  assert.equal(
    urls.media("./assets/portrait.svg", corpus.prose[0].file),
    "/media/home/portrait.svg",
  );
  assert.equal(
    urls.link("../target/en.md#heading", source),
    "/en/posts/different-slug/#heading",
  );
  for (const url of [
    "#heading",
    "/en/posts/other/#heading",
    "https://example.invalid/a?x=1#b",
    "mailto:name@example.invalid",
    "unrelated%ZZ.html",
  ])
    assert.equal(urls.link(url, source), url);
});

test("article group names are encoded as literal media path components", () => {
  const group = "2026/Protocol Buffers_図.#%";
  const source = path.resolve("input", group, "en.md");
  const urls = contentUrls({
    posts: [{ file: source, group, url: "/en/posts/protocol-buffers/" }],
    prose: [],
  });
  const url = urls.media("./assets/picture.svg", source);
  assert.equal(
    url,
    "/media/2026/Protocol%20Buffers_%E5%9B%B3.%23%25/picture.svg",
  );
  const parsed = new URL(url, "https://example.invalid");
  assert.equal(parsed.search, "");
  assert.equal(parsed.hash, "");
  assert.equal(
    decodeURIComponent(parsed.pathname),
    "/media/" + group + "/picture.svg",
  );
});

test("local assets reject request suffixes and escaping paths consistently", () => {
  const urls = contentUrls(corpus);
  for (const url of [
    "./assets/picture.svg?v=1",
    "./assets/picture.svg#view",
    "./assets/../outside.svg",
    "./assets/%2e%2e/outside.svg",
    "./assets/a%2Fb.svg",
    "./assets/a%5Cb.svg",
    "./assets/a%00b.svg",
  ]) {
    assert.throws(() => localAsset(url, source));
    assert.throws(() => urls.link(url, source));
    assert.throws(() => urls.media(url, source));
  }
  assert.equal(
    localAsset("./assets/a%23b.svg", source).encodedPath,
    "a%23b.svg",
  );
});

test("image imports use safe workspace names without modifying originals", () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-image-workspace-"));
  try {
    const source = path.join(work, "input/en.md");
    const assets = path.join(work, "input/assets");
    const renderer = path.join(work, "renderer");
    mkdirSync(assets, { recursive: true });
    const original =
      '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>';
    writeFileSync(path.join(assets, "a&b.svg"), original);
    writeFileSync(path.join(assets, "same.svg"), original);
    const prepare = imageWorkspace(renderer);
    const first = prepare("./assets/a%26b.svg", source);
    assert.match(first.url, /\.astro\/cfgb-images\/[a-f0-9]{64}\.svg$/);
    assert.equal(
      readFileSync(path.resolve(path.dirname(source), first.url), "utf8"),
      original,
    );
    assert.deepEqual(prepare("./assets/same.svg", source), first);
    assert.equal(readFileSync(path.join(assets, "a&b.svg"), "utf8"), original);
    assert.equal(prepare("https://example.invalid/remote.png", source), null);
    assert.throws(
      () => prepare("./assets/a%26b.svg?v=1", source),
      /query or fragment/,
    );
    assert.throws(() => prepare("./assets/missing.svg", source), /ENOENT/);
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});

test("workspace adaptation leaves format support to Astro", () => {
  const work = mkdtempSync(path.join(tmpdir(), "cfgb-image-format-"));
  try {
    const source = path.join(work, "input/en.md");
    const assets = path.join(work, "input/assets");
    mkdirSync(assets, { recursive: true });
    const bytes = Buffer.from("format handling belongs to the image service");
    writeFileSync(path.join(assets, "picture.unlisted"), bytes);
    const adapted = imageWorkspace(path.join(work, "renderer"))(
      "./assets/picture.unlisted",
      source,
    );
    assert.match(adapted.url, /\.astro\/cfgb-images\/[a-f0-9]{64}\.unlisted$/);
    assert.deepEqual(
      readFileSync(path.resolve(path.dirname(source), adapted.url)),
      bytes,
    );
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
});
