import { parseFragment } from "parse5";

const referenceAttributes = new Set(["href", "aria-describedby", "aria-labelledby", "aria-controls", "headers"]);

// namespaceFragment prefixes ids in one rendered fragment and the references
// that point at those ids. Other links are left unchanged. The original tags
// stay in place; only attribute values are rewritten.
export function namespaceFragment(html, prefix) {
  if (!html || !prefix) return html;
  const fragment = parseFragment(html, { sourceCodeLocationInfo: true });
  const ids = new Set();
  walkElements(fragment, (node) => {
    const id = attribute(node, "id");
    if (id) ids.add(id);
  });
  if (ids.size === 0) return html;
  const edits = [];
  walkElements(fragment, (node) => {
    const locations = node.sourceCodeLocation?.attrs;
    if (!locations) return;
    for (const attr of node.attrs || []) {
      const name = attr.name.toLowerCase();
      if (name !== "id" && !referenceAttributes.has(name)) continue;
      const next = name === "id" ? prefixId(attr.value, ids, prefix) : prefixReferences(name, attr.value, ids, prefix);
      if (next === attr.value) continue;
      const range = attributeValueRange(html, locations[name]);
      if (!range) continue;
      edits.push({ start: range.start, end: range.end, value: encodeAttributeValue(next, range.quote) });
    }
  });
  if (edits.length === 0) return html;
  edits.sort((left, right) => right.start - left.start);
  let out = html;
  for (const edit of edits) out = out.slice(0, edit.start) + edit.value + out.slice(edit.end);
  return out;
}

function prefixId(value, ids, prefix) {
  return ids.has(value) ? `${prefix}${value}` : value;
}

function prefixReferences(name, value, ids, prefix) {
  if (name === "href") return prefixFragment(value, ids, prefix);
  return value
    .split(/\s+/)
    .map((part) => (ids.has(part) ? `${prefix}${part}` : part))
    .join(" ");
}

function prefixFragment(value, ids, prefix) {
  if (!value.startsWith("#")) return value;
  const raw = value.slice(1);
  let decoded = raw;
  try {
    decoded = decodeURIComponent(raw);
  } catch {
    decoded = raw;
  }
  const id = ids.has(raw) ? raw : ids.has(decoded) ? decoded : "";
  if (!id) return value;
  return raw === id ? `#${prefix}${id}` : `#${encodeURIComponent(prefix + id)}`;
}

function attribute(node, name) {
  return node.attrs?.find((attr) => attr.name.toLowerCase() === name)?.value ?? "";
}

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
  return value.replaceAll("&", "&amp;").replaceAll(quote, quote === '"' ? "&quot;" : "&apos;");
}

function walkElements(node, visit) {
  if (node.tagName) visit(node);
  for (const child of node.childNodes || []) walkElements(child, visit);
}
