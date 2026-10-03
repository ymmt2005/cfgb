import assert from "node:assert/strict";
import test from "node:test";
import { splitFrontmatter } from "../src/lib/load-site.mjs";

test("frontmatter keeps inline arrays, block arrays, folded scalars, and quoted text", () => {
  const inline = splitFrontmatter('---\ntopics: [protobuf, oss]\naliases: ["/ja/posts/old/"]\n---\nbody\n');
  assert.deepEqual(inline.data.topics, ["protobuf", "oss"]);
  assert.deepEqual(inline.data.aliases, ["/ja/posts/old/"]);
  assert.equal(inline.body, "body\n");

  const block = splitFrontmatter("---\ntopics:\n- protobuf\n- oss\naliases:\n- /ja/posts/old-protobuf-guide/\n---\n");
  assert.deepEqual(block.data.topics, ["protobuf", "oss"]);
  assert.deepEqual(block.data.aliases, ["/ja/posts/old-protobuf-guide/"]);

  const folded = splitFrontmatter("---\nsummary: >\n  hello\n  world\n---\n");
  assert.equal(folded.data.summary, "hello world\n");

  const quoted = splitFrontmatter('---\ntitle: "say \\"hi\\""\n---\n');
  assert.equal(quoted.data.title, 'say "hi"');

  const dated = splitFrontmatter("---\npublishedAt: '2026-09-19T13:12:40+09:00'\n---\n");
  assert.equal(typeof dated.data.publishedAt, "string");
  assert.equal(dated.data.publishedAt, "2026-09-19T13:12:40+09:00");
});
