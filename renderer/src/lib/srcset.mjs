// Rewrite URL spans using the HTML srcset splitting/tokenizer rules:
// https://html.spec.whatwg.org/multipage/images.html#parse-a-srcset-attribute
// Commas inside URLs (notably data URLs) are not candidate separators. Keep
// descriptors and their spacing verbatim; the browser owns their validation.
export function transformSrcset(value, transform) {
  const edits = [];
  let position = 0;
  while (position < value.length) {
    while (
      position < value.length &&
      (whitespace(value[position]) || value[position] === ",")
    )
      position++;
    if (position === value.length) break;
    const start = position;
    while (position < value.length && !whitespace(value[position])) position++;
    let end = position;
    while (value[end - 1] === ",") end--;
    const url = value.slice(start, end);
    const next = transform(url);
    if (next !== url) edits.push({ start, end, value: next });
    // A trailing comma ends a descriptor-less candidate immediately.
    if (end !== position) continue;
    let inParens = false;
    while (position < value.length) {
      const char = value[position++];
      if (inParens) {
        if (char === ")") inParens = false;
      } else if (char === "(") {
        inParens = true;
      } else if (char === ",") {
        break;
      }
    }
  }
  for (const edit of edits.reverse())
    value = value.slice(0, edit.start) + edit.value + value.slice(edit.end);
  return value;
}

function whitespace(char) {
  return " \t\n\r\f".includes(char);
}
