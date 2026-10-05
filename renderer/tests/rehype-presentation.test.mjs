import assert from "node:assert/strict";
import test from "node:test";
import { unified } from "@astrojs/markdown-remark";
import { parseFragment } from "parse5";
import { rehypePresentation } from "../src/plugins/rehype-presentation.mjs";

test("presentation preserves table semantics, repeated footnote targets and authored HTML", async () => {
  const processor = unified({ rehypePlugins: [rehypePresentation] });
  const renderer = await processor.createRenderer({ syntaxHighlight: false });
  const markdown = [
    "| Name | Value |\n| --- | --- |\n| **Markdown cell** | `code` |",
    "An ordinary superscript <sup>2</sup> and repeated notes[^n] [^n].",
    "[^n]: A note with *Markdown* content.",
    '<table id="authored"><caption>Authored caption</caption><tbody><tr><td>Raw cell</td></tr></tbody></table>',
    '```html\n<table id="literal"><tr><td>Example</td></tr></table>\n```',
  ].join("\n\n");
  const { code } = await renderer.render(markdown);
  const tree = parseFragment(code);
  const nodes = [];
  function walk(node) {
    nodes.push(node);
    for (const child of node.childNodes ?? []) walk(child);
  }
  walk(tree);
  const attr = (node, name) => node.attrs?.find((a) => a.name === name)?.value;
  const wrappers = nodes.filter((n) => attr(n, "class") === "table-wrap");
  assert.equal(wrappers.length, 1);
  assert.equal(
    wrappers[0].childNodes.find((node) => node.tagName)?.tagName,
    "table",
  );
  assert.equal(nodes.filter((n) => n.tagName === "th").length, 2);
  assert.ok(nodes.some((n) => n.tagName === "strong"));
  const refs = nodes.filter((n) => attr(n, "data-footnote-ref") !== undefined);
  assert.equal(refs.length, 2);
  for (const ref of refs) {
    assert.equal(attr(ref.parentNode, "class"), "footnote-ref");
    assert.ok(nodes.some((n) => attr(n, "id") === attr(ref, "href").slice(1)));
  }
  const back = nodes.filter(
    (n) => attr(n, "data-footnote-backref") !== undefined,
  );
  assert.equal(back.length, 2);
  for (const link of back) {
    assert.ok(attr(link, "class").split(" ").includes("footnote-back"));
    assert.ok(
      refs.some((ref) => attr(ref, "id") === attr(link, "href").slice(1)),
    );
    assert.ok(attr(link, "aria-label"));
  }
  const ordinary = nodes.find(
    (n) => n.tagName === "sup" && attr(n, "class") === undefined,
  );
  assert.equal(ordinary.childNodes[0].value, "2");
  const authored = nodes.find((n) => attr(n, "id") === "authored");
  assert.equal(authored.parentNode, tree);
  assert.equal(authored.childNodes[0].tagName, "caption");
  assert.equal(
    nodes.some((n) => attr(n, "id") === "literal"),
    false,
  );
  const literal = nodes.find(
    (node) =>
      node.tagName === "code" && attr(node, "class") === "language-html",
  );
  assert.match(literal.childNodes[0].value, /<table id="literal"/);
});
