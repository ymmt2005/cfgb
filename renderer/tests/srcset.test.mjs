import assert from "node:assert/strict";
import test from "node:test";
import { transformSrcset } from "../src/lib/srcset.mjs";

const map = (url) => url.replace(/^\.\/assets\//, "/media/article/");

test("srcset preserves density/width descriptors and ASCII spacing", () => {
  for (const [input, expected] of [
    [
      "./assets/a.png 1x, ./assets/b.png 2x",
      "/media/article/a.png 1x, /media/article/b.png 2x",
    ],
    [
      "\t./assets/a.png\n320w,\r\n./assets/b.png\f640w ",
      "\t/media/article/a.png\n320w,\r\n/media/article/b.png\f640w ",
    ],
    [
      "./assets/a.png, ./assets/b.png 2x,",
      "/media/article/a.png, /media/article/b.png 2x,",
    ],
    ["", ""],
  ])
    assert.equal(transformSrcset(input, map), expected);
});

test("srcset keeps commas inside URLs and leaves remote/data URLs intact", () => {
  const input =
    "data:image/svg+xml,%3Csvg%3E,%3C/svg%3E 1x, ./assets/a,b.svg 2x, https://example.invalid/a,b.svg?q=1#view 3x";
  const visited = [];
  assert.equal(
    transformSrcset(input, (url) => {
      visited.push(url);
      return map(url);
    }),
    input.replace("./assets/", "/media/article/"),
  );
  assert.deepEqual(visited, [
    "data:image/svg+xml,%3Csvg%3E,%3C/svg%3E",
    "./assets/a,b.svg",
    "https://example.invalid/a,b.svg?q=1#view",
  ]);
  // A comma without following whitespace remains part of the URL in HTML.
  assert.equal(
    transformSrcset("./assets/a.png,./assets/b.png", map),
    "/media/article/a.png,./assets/b.png",
  );
});

test("srcset does not mistake descriptor parentheses for candidates", () => {
  assert.equal(
    transformSrcset("./assets/a.png future(1, 2), ./assets/b.png 2x", map),
    "/media/article/a.png future(1, 2), /media/article/b.png 2x",
  );
});
