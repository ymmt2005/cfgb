import { parseFragment } from "parse5";
import { transformAttributes, walkElements } from "./html-attributes.mjs";

const referenceAttributes = new Set([
  "href",
  "for",
  "list",
  "form",
  "headers",
  "itemref",
  "popovertarget",
  "commandfor",
  "aria-activedescendant",
  "aria-controls",
  "aria-describedby",
  "aria-details",
  "aria-errormessage",
  "aria-flowto",
  "aria-labelledby",
  "aria-owns",
]);

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
  return transformAttributes(
    html,
    (attr) => {
      const name = attr.name.toLowerCase();
      if (name !== "id" && !referenceAttributes.has(name)) return;
      return name === "id"
        ? prefixId(attr.value, ids, prefix)
        : prefixReferences(name, attr.value, ids, prefix);
    },
    fragment,
  );
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
  return (
    node.attrs?.find((attr) => attr.name.toLowerCase() === name)?.value ?? ""
  );
}
