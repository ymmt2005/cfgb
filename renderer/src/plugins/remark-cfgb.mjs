import path from "node:path";
import { parseFragment } from "parse5";
import { cardKey, loadSite } from "../lib/load-site.mjs";

const alerts = {
  NOTE: "note",
  TIP: "tip",
  IMPORTANT: "important",
  WARNING: "warning",
  CAUTION: "caution",
};

export function remarkCfgb() {
  const corpus = loadSite();
  const byFile = new Map(corpus.posts.map((post) => [path.resolve(post.file), post]));
  return (tree, file) => {
    const source = file.path ? path.resolve(file.path) : "";
    walk(tree, (node, parent, index) => {
      if (node.type === "code" && node.lang === "mermaid" && parent) {
        parent.children[index] = html(mermaidBlock(node.value || ""));
        return;
      }
      if (node.type === "blockquote" && parent) {
        markAlert(node);
        return;
      }
      if (node.type === "html" && typeof node.value === "string") {
        node.value = rewriteHtml(node.value, source, byFile);
      }
      if (node.type === "paragraph" && parent) {
        const card = cardBlock(node, corpus.linkcards);
        if (card) parent.children[index] = html(card);
        return;
      }
      if ((node.type === "link" || node.type === "definition") && node.url) {
        node.url = rewriteLink(node.url, source, byFile);
      }
      if (node.type === "image" && node.url) {
        node.url = rewriteImage(node.url, source);
      }
    });
  };
}

function walk(node, visit, parent = null, index = 0) {
  visit(node, parent, index);
  if (!node.children) return;
  for (let i = 0; i < node.children.length; i++) walk(node.children[i], visit, node, i);
}

function html(value) {
  return { type: "html", value };
}

function escapeHtml(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;");
}

function mermaidBlock(source) {
  const text = escapeHtml(source.replace(/\n$/, ""));
  return `<div class="diagram-block"><pre class="mermaid">${text}</pre><pre class="mermaid-source"><code>${text}</code></pre></div>`;
}

function markAlert(node) {
  const first = node.children?.[0];
  if (!first || first.type !== "paragraph" || !first.children?.length) return;
  const lead = first.children[0];
  if (lead.type !== "text") return;
  const match = lead.value.match(/^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*\n?/);
  if (!match) return;
  lead.value = lead.value.slice(match[0].length);
  if (!lead.value) first.children.shift();
  if (first.children.length === 0) node.children.shift();
  const kind = alerts[match[1]];
  const label = match[1][0] + match[1].slice(1).toLowerCase();
  node.children.unshift({
    type: "paragraph",
    children: [{ type: "text", value: label }],
    data: { hProperties: { className: ["alert-label"] } },
  });
  node.data = {
    ...(node.data || {}),
    hName: "div",
    hProperties: { className: ["alert", `alert-${kind}`] },
  };
}

function rewriteHtml(value, source, byFile) {
  const fragment = parseFragment(value, { sourceCodeLocationInfo: true });
  const edits = [];
  walkElements(fragment, (node) => {
    const locations = node.sourceCodeLocation?.attrs;
    if (!locations) return;
    for (const attr of node.attrs || []) {
      const name = attr.name.toLowerCase();
      if (name !== "href" && name !== "src") continue;
      const next = name === "src" ? rewriteImage(attr.value, source) : rewriteLink(attr.value, source, byFile);
      if (next === attr.value) continue;
      const range = attributeValueRange(value, locations[name]);
      if (!range) continue;
      edits.push({ start: range.start, end: range.end, value: encodeAttributeValue(next, range.quote) });
    }
  });
  if (edits.length === 0) return value;
  edits.sort((a, b) => b.start - a.start);
  let out = value;
  for (const edit of edits) out = out.slice(0, edit.start) + edit.value + out.slice(edit.end);
  return out;
}

// attributeValueRange locates the value inside one attribute. The span from
// parse5 covers the whole attribute, including its name and quotes.
function attributeValueRange(source, loc) {
  if (!loc) return null;
  const raw = source.slice(loc.startOffset, loc.endOffset);
  const eq = raw.indexOf("=");
  if (eq < 0) return null;
  let i = eq + 1;
  while (i < raw.length && " \t\n\r\f".includes(raw[i])) i++;
  if (i >= raw.length) return null;
  const quote = raw[i];
  if (quote === '"' || quote === "'") {
    const end = raw.lastIndexOf(quote);
    if (end <= i) return null;
    return { start: loc.startOffset + i + 1, end: loc.startOffset + end, quote };
  }
  return { start: loc.startOffset + i, end: loc.endOffset, quote: "" };
}

function encodeAttributeValue(value, quote) {
  if (quote !== '"' && quote !== "'") {
    if (value !== "" && !/[\s"'=<>`]/.test(value)) return value.replaceAll("&", "&amp;");
    quote = '"';
    return `"${value.replaceAll("&", "&amp;").replaceAll('"', "&quot;")}"`;
  }
  const escaped = value.replaceAll("&", "&amp;").replaceAll(quote, quote === '"' ? "&quot;" : "&apos;");
  return escaped;
}

function walkElements(node, visit) {
  if (node.tagName) visit(node);
  for (const child of node.childNodes || []) walkElements(child, visit);
}

function textOf(node) {
  if (node.type === "text" || node.type === "inlineCode") return node.value || "";
  if (!node.children) return "";
  return node.children.map(textOf).join("");
}

function cardBlock(node, cards) {
  const meaningful = (node.children || []).filter((child) => !(child.type === "text" && !child.value.trim()));
  if (meaningful.length !== 1 || meaningful[0].type !== "link") return "";
  const link = meaningful[0];
  const label = textOf(link).trim();
  if (!/^https?:\/\//.test(link.url)) return "";
  if (label !== link.url && label !== link.url.replace(/\/$/, "")) return "";
  const card = cards.get(cardKey(link.url));
  if (!card) return "";
  return `<a class="card" href="${escapeHtml(link.url)}"><span class="card-site">${escapeHtml(card.siteName || "")}</span><span class="card-title">${escapeHtml(card.title || link.url)}</span><span class="card-desc">${escapeHtml(card.description || "")}</span></a>`;
}

function rewriteLink(url, source, byFile) {
  const hashIndex = url.indexOf("#");
  const pathPart = hashIndex >= 0 ? url.slice(0, hashIndex) : url;
  const hash = hashIndex >= 0 ? url.slice(hashIndex) : "";
  if (!pathPart.endsWith(".md")) return url;
  if (!source) return url;
  const resolved = path.resolve(path.dirname(source), pathPart);
  const target = byFile.get(resolved);
  if (!target) return url;
  return `${target.url}${hash}`;
}

function rewriteImage(url, source) {
  if (!url.startsWith("./assets/") || !source) return url;
  const match = source.match(/posts\/(\d{4})\/([^/]+)\/[^/]+\.md$/);
  if (!match) return url;
  const name = url.slice("./assets/".length);
  return `/media/${match[1]}/${match[2]}/${name.split("/").map(encodeURIComponent).join("/")}`;
}
