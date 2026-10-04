import assert from "node:assert/strict";
import test from "node:test";
import { parse } from "parse5";
import { restoreImageSuffixes } from "../src/lib/image-paths.mjs";

test("processed src and srcset retain suffixes without leaking temporary attributes", () => {
  const source =
    '<!doctype html><img alt="Diagram" src="/_astro/diagram.svg" srcset="/_astro/small.svg 1x, /_astro/large.svg 2x" data-cfgb-image-suffix="?v=a,b&amp;ref=1#detail">';
  const result = restoreImageSuffixes(source);
  const nodes = [];
  function walk(node) {
    if (node.tagName) nodes.push(node);
    for (const child of node.childNodes || []) walk(child);
  }
  walk(parse(result));
  const image = nodes.find((node) => node.tagName === "img");
  const attribute = (name) =>
    image.attrs.find((item) => item.name === name)?.value;
  assert.equal(attribute("src"), "/_astro/diagram.svg?v=a,b&ref=1#detail");
  assert.equal(
    attribute("srcset"),
    "/_astro/small.svg?v=a%2Cb&ref=1#detail 1x, /_astro/large.svg?v=a%2Cb&ref=1#detail 2x",
  );
  assert.equal(attribute("alt"), "Diagram");
  assert.equal(attribute("data-cfgb-image-suffix"), undefined);
  assert.equal(restoreImageSuffixes(result), result);
});

test("literal markers in examples are preserved, actual unresolved images fail", () => {
  const examples =
    '<pre>&lt;img __ASTRO_IMAGE_="example"&gt;</pre><!-- <img __ASTRO_IMAGE_="comment"> -->';
  assert.equal(restoreImageSuffixes(examples), examples);
  assert.throws(
    () => restoreImageSuffixes('<img __ASTRO_IMAGE_="unresolved">'),
    /unresolved Markdown image/,
  );
});
