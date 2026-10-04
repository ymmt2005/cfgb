import assert from "node:assert/strict";
import test from "node:test";
import { transformAttributes } from "../src/lib/html-attributes.mjs";
import { namespaceFragment } from "../src/lib/fragment-ids.mjs";

test("source edits keep split opening tags and quote-sensitive values intact", () => {
  assert.equal(
    transformAttributes('<a href="old">', (a) =>
      a.name === "href" ? "/new?a=1&b=2" : undefined,
    ),
    '<a href="/new?a=1&amp;b=2">',
  );
  assert.equal(
    transformAttributes("<a href='old'>", (a) =>
      a.name === "href" ? "/new's" : undefined,
    ),
    "<a href='/new&apos;s'>",
  );
  assert.equal(
    transformAttributes("<img src=old alt=unchanged>", (a) =>
      a.name === "src" ? "/a b.png" : undefined,
    ),
    '<img src="/a b.png" alt=unchanged>',
  );
  const literal =
    '<!-- <a href="old"> --> <script>"<a href=old>"</script><code>&lt;a href=old&gt;</code>';
  assert.equal(
    transformAttributes(literal, () => "rewritten"),
    literal,
  );
});

test("foreign and template attributes use their actual source locations", () => {
  const html =
    '<svg><a xlink:href="#target">Link</a><g id="target"></g></svg><template><label for="field">Label</label><input id="field"></template>';
  const result = namespaceFragment(html, "aside-");
  assert.ok(result.includes('xlink:href="#aside-target"'));
  assert.ok(result.includes('id="aside-target"'));
  assert.ok(result.includes('for="aside-field"'));
  assert.ok(result.includes('id="aside-field"'));
});
