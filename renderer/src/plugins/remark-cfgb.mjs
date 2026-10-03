import path from "node:path";
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
        const alert = alertBlock(node);
        if (alert) parent.children[index] = html(alert);
        return;
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

function alertBlock(node) {
  const first = node.children?.[0];
  if (!first || first.type !== "paragraph") return "";
  const raw = textOf(first).replace(/^\s+/, "");
  const match = raw.match(/^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*/);
  if (!match) return "";
  const kind = alerts[match[1]];
  const label = match[1][0] + match[1].slice(1).toLowerCase();
  const rest = raw.slice(match[0].length).trim();
  const more = node.children.slice(1).map(paragraphText).filter(Boolean);
  const body = [rest, ...more].filter(Boolean).map((line) => `<p>${escapeHtml(line)}</p>`).join("");
  return `<div class="alert alert-${kind}"><p class="alert-label">${label}</p>${body}</div>`;
}

function paragraphText(node) {
  if (node.type !== "paragraph") return "";
  return textOf(node).trim();
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
