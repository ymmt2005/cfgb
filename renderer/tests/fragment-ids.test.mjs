import assert from "node:assert/strict";
import test from "node:test";
import { namespaceFragment } from "../src/lib/fragment-ids.mjs";

const html = [
  '<h2 id="shared-heading">Shared heading</h2>',
  '<p>See <sup><a href="#user-content-fn-note" id="user-content-fnref-note" data-footnote-ref="" aria-describedby="footnote-label">1</a></sup>.</p>',
  '<p><a href="/ja/posts/other/#shared-heading">article anchor</a></p>',
  '<section data-footnotes="" class="footnotes"><h2 id="footnote-label">Footnotes</h2>',
  '<ol><li id="user-content-fn-note"><p>Aside note <a href="#user-content-fnref-note" data-footnote-backref="" aria-label="Back to reference 1" class="data-footnote-backref">↩</a></p></li></ol>',
  "</section>",
].join("");

test("aside ids and references move together and article anchors stay put", () => {
  const namespaced = namespaceFragment(html, "aside-");
  assert.match(namespaced, /id="aside-shared-heading"/);
  assert.match(namespaced, /id="aside-user-content-fn-note"/);
  assert.match(namespaced, /id="aside-user-content-fnref-note"/);
  assert.match(namespaced, /id="aside-footnote-label"/);
  assert.match(namespaced, /href="#aside-user-content-fn-note"/);
  assert.match(namespaced, /href="#aside-user-content-fnref-note"/);
  assert.match(namespaced, /aria-describedby="aside-footnote-label"/);
  assert.match(namespaced, /href="\/ja\/posts\/other\/#shared-heading"/);
  assert.equal(namespaced.includes('id="shared-heading"'), false);
  assert.equal(namespaced.includes('id="footnote-label"'), false);
  assert.equal(namespaced.includes('href="#user-content-fn-note"'), false);
});

test("HTML and ARIA ID references follow authored aside IDs", () => {
  const source = [
    '<label for="choice">Choice</label><input id="choice" list="choices" form="signup">',
    '<datalist id="choices"></datalist><form id="signup"></form>',
    '<span id="details">Details</span><span id="extra">Extra</span>',
    '<div itemscope itemref="details extra outside"></div>',
    '<button popovertarget="details" commandfor="extra">Open</button>',
    '<button aria-details="details" aria-errormessage="extra" aria-activedescendant="choice"',
    ' aria-owns="details extra" aria-flowto="details outside" aria-controls="choice outside"',
    ' aria-labelledby="details extra" aria-describedby="extra">More</button>',
    '<table><tr><th id="column">Column</th><td headers="column">Cell</td></tr></table>',
  ].join("");
  const result = namespaceFragment(source, "aside-");
  for (const [name, value] of [
    ["for", "aside-choice"],
    ["list", "aside-choices"],
    ["form", "aside-signup"],
    ["aria-details", "aside-details"],
    ["aria-errormessage", "aside-extra"],
    ["aria-activedescendant", "aside-choice"],
    ["aria-owns", "aside-details aside-extra"],
    ["aria-flowto", "aside-details outside"],
    ["aria-controls", "aside-choice outside"],
    ["aria-labelledby", "aside-details aside-extra"],
    ["aria-describedby", "aside-extra"],
    ["headers", "aside-column"],
    ["itemref", "aside-details aside-extra outside"],
    ["popovertarget", "aside-details"],
    ["commandfor", "aside-extra"],
  ])
    assert.ok(result.includes(`${name}="${value}"`), result);
});

test("fragment encoding, entities and quote styles retain correct targets", () => {
  const result = namespaceFragment(
    [
      '<h2 id="見出し">Heading</h2><a href="#%E8%A6%8B%E5%87%BA%E3%81%97">Go</a>',
      '<p id="a&amp;b">Target</p><a href="#a&amp;b">Go</a>',
      "<span id='single'></span><a href='#single'>Go</a>",
      "<span id=bare></span><a href=#bare>Go</a>",
      '<a href="#outside">Outside</a><a href="/en/posts/other/#bare">Other</a>',
    ].join(""),
    "aside-",
  );
  assert.ok(result.includes('id="aside-見出し"'));
  assert.ok(result.includes('href="#aside-%E8%A6%8B%E5%87%BA%E3%81%97"'));
  assert.ok(result.includes('href="#aside-a&amp;b"'));
  assert.ok(result.includes("href='#aside-single'"));
  assert.ok(result.includes("href=#aside-bare"));
  assert.ok(result.includes('href="#outside"'));
  assert.ok(result.includes('href="/en/posts/other/#bare"'));
});
