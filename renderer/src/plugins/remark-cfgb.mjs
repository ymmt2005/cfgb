import path from "node:path";
import { transformAttributes } from "../lib/html-attributes.mjs";
import { contentUrls } from "../lib/content-urls.mjs";
import { transformSrcset } from "../lib/srcset.mjs";
import { cardKey, loadSite } from "../lib/load-site.mjs";

const alerts = {
  NOTE: "note",
  TIP: "tip",
  IMPORTANT: "important",
  WARNING: "warning",
  CAUTION: "caution",
};

export function remarkCfgb(options = {}) {
  const corpus = loadSite();
  const urls = contentUrls(corpus);
  return (tree, file) => {
    const source = file.path ? path.resolve(file.path) : "";
    const definitions = new Map();
    walk(tree, (node) => {
      if (node.type === "definition" && node.identifier) {
        const identifier = referenceIdentifier(node.identifier);
        if (!definitions.has(identifier))
          definitions.set(identifier, { url: node.url, title: node.title });
      }
    });
    // 1. Expand references before any URL or block consumer sees them.
    walk(tree, (node, parent, index) => {
      if (
        (node.type === "imageReference" || node.type === "linkReference") &&
        parent
      ) {
        const definition = definitions.get(
          referenceIdentifier(node.identifier),
        );
        if (definition) {
          const image = node.type === "imageReference";
          node = parent.children[index] = {
            type: image ? "image" : "link",
            url: definition.url,
            ...(node.position ? { position: node.position } : {}),
            title: definition.title,
            ...(image ? { alt: node.alt } : { children: node.children }),
            ...(node.data ? { data: node.data } : {}),
          };
        }
      }
    });
    // 2. Normalize URLs once, with separate image-import and public-link paths.
    walk(tree, (node) => {
      if (node.type === "image" && node.url) {
        // Enforce the same file-reference contract even without image staging.
        urls.media(node.url, source);
        const prepared = options.prepareImage?.(node.url, source);
        if (prepared) node.url = prepared.url;
      } else if (node.type === "link" && node.url) {
        node.url = urls.link(node.url, source);
      } else if (node.type === "html" && typeof node.value === "string") {
        node.value = transformAttributes(node.value, (attr) => {
          if (attr.name === "src") return urls.media(attr.value, source);
          if (attr.name === "srcset" || attr.name === "imagesrcset")
            return transformSrcset(attr.value, (url) =>
              urls.media(url, source),
            );
          if (attr.name === "href") return urls.link(attr.value, source);
        });
      }
    });
    // 3. Block transforms see resolved children (including reference links).
    walk(tree, (node, parent, index) => {
      if (node.type === "code" && node.lang === "mermaid" && parent) {
        parent.children[index] = html(mermaidBlock(node.value || ""));
        return;
      }
      if (node.type === "blockquote" && parent) {
        markAlert(node);
        return;
      }
      if (node.type === "paragraph" && parent) {
        const card = cardBlock(node, corpus.linkcards);
        if (card) parent.children[index] = html(card);
        return;
      }
    });
  };
}

function referenceIdentifier(value) {
  return String(value || "")
    .replace(/[\t\n\r ]+/g, " ")
    .trim()
    .toLowerCase()
    .toUpperCase();
}

function walk(node, visit, parent = null, index = 0) {
  visit(node, parent, index);
  const current = parent ? parent.children[index] : node;
  if (!current.children) return;
  for (let i = 0; i < current.children.length; i++)
    walk(current.children[i], visit, current, i);
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
  return `<div class="diagram-block"><pre class="mermaid">${text}</pre></div>`;
}

function markAlert(node) {
  const first = node.children?.[0];
  if (!first || first.type !== "paragraph" || !first.children?.length) return;
  const lead = first.children[0];
  if (lead.type !== "text") return;
  const match = lead.value.match(
    /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*\n?/,
  );
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

function textOf(node) {
  if (node.type === "text" || node.type === "inlineCode")
    return node.value || "";
  if (!node.children) return "";
  return node.children.map(textOf).join("");
}

function cardBlock(node, cards) {
  const meaningful = (node.children || []).filter(
    (child) => !(child.type === "text" && !child.value.trim()),
  );
  if (meaningful.length !== 1 || meaningful[0].type !== "link") return "";
  const link = meaningful[0];
  const label = textOf(link).trim();
  if (!/^https?:\/\//.test(link.url)) return "";
  if (label !== link.url && label !== link.url.replace(/\/$/, "")) return "";
  const card = cards.get(cardKey(link.url));
  if (!card) return "";
  return `<a class="card" href="${escapeHtml(link.url)}"><span class="card-site">${escapeHtml(card.siteName || "")}</span><span class="card-title">${escapeHtml(card.title || link.url)}</span><span class="card-desc">${escapeHtml(card.description || "")}</span></a>`;
}
