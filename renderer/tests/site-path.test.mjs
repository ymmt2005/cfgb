import assert from "node:assert/strict";
import test from "node:test";
import {
  absolute,
  basePath,
  routePath,
  sitePath,
} from "../src/lib/site-path.mjs";
import { contentUrls } from "../src/lib/content-urls.mjs";

test("hosting prefixes apply at URL boundaries while domain routes remain local", () => {
  for (const baseUrl of [
    "https://example.invalid",
    "https://example.invalid/blog",
    "https://example.invalid/blog/",
  ]) {
    const site = { baseUrl };
    const prefix = baseUrl.includes("/blog") ? "/blog" : "";
    assert.equal(basePath(site), prefix);
    assert.equal(
      sitePath(site, "/en/posts/guide/#heading"),
      `${prefix}/en/posts/guide/#heading`,
    );
    assert.equal(
      routePath(site, `${prefix}/en/posts/guide/`),
      "/en/posts/guide/",
    );
    assert.equal(
      absolute(site, "/en/posts/guide/"),
      `https://example.invalid${prefix}/en/posts/guide/`,
    );
    for (const external of [
      "https://other.invalid/picture.svg",
      "//other.invalid/picture.svg",
      "#heading",
      "mailto:author@example.invalid",
    ])
      assert.equal(sitePath(site, external), external);
    const urls = contentUrls({
      site,
      posts: [
        {
          file: "/content/posts/group/en.md",
          group: "2026/group",
          url: "/en/posts/guide/",
        },
      ],
      prose: [],
    });
    assert.equal(
      urls.media("./assets/picture.svg", "/content/posts/group/en.md"),
      `${prefix}/media/2026/group/picture.svg`,
    );
    assert.equal(
      urls.link("en.md#heading", "/content/posts/group/en.md"),
      `${prefix}/en/posts/guide/#heading`,
    );
    assert.equal(
      urls.link("/en/posts/guide/#heading", "/content/posts/group/en.md"),
      `${prefix}/en/posts/guide/#heading`,
    );
  }
});
