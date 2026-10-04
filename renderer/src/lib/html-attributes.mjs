import { parseFragment } from "parse5";

// Edit only source attribute values. Serializing a parsed fragment would close
// opening tags that Markdown represents separately from their inline contents.
export function transformAttributes(
  html,
  transform,
  fragment = parseFragment(html, { sourceCodeLocationInfo: true }),
) {
  const edits = [];
  walkElements(fragment, (node) => {
    const locations = node.sourceCodeLocation?.attrs;
    if (!locations) return;
    for (const attr of node.attrs || []) {
      const next = transform(attr, node);
      if (next === undefined || next === attr.value) continue;
      const key = attr.prefix ? `${attr.prefix}:${attr.name}` : attr.name;
      const range = attributeValueRange(html, locations[key]);
      if (range)
        edits.push({
          ...range,
          value: encodeAttributeValue(next, range.quote),
        });
    }
  });
  edits.sort((left, right) => right.start - left.start);
  for (const edit of edits)
    html = html.slice(0, edit.start) + edit.value + html.slice(edit.end);
  return html;
}

export function walkElements(node, visit) {
  if (node.tagName) visit(node);
  for (const child of node.childNodes || []) walkElements(child, visit);
  // parse5 stores template contents in a separate document fragment.
  if (node.content) walkElements(node.content, visit);
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
    return {
      start: loc.startOffset + i + 1,
      end: loc.startOffset + end,
      quote,
    };
  }
  return { start: loc.startOffset + i, end: loc.endOffset, quote: "" };
}

function encodeAttributeValue(value, quote) {
  if (quote !== '"' && quote !== "'") {
    if (value !== "" && !/[\s"'=<>`]/.test(value))
      return value.replaceAll("&", "&amp;");
    return `"${value.replaceAll("&", "&amp;").replaceAll('"', "&quot;")}"`;
  }
  return value
    .replaceAll("&", "&amp;")
    .replaceAll(quote, quote === '"' ? "&quot;" : "&apos;");
}
