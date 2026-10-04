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
