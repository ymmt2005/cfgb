// Match the mockup's presentation using generated HTML nodes, without
// modifying authored raw HTML or literal Markdown examples.
export function rehypePresentation() {
  return (tree) => {
    function visit(parent) {
      if (!parent.children) return;
      for (let index = 0; index < parent.children.length; index++) {
        const node = parent.children[index];
        if (node.type === "element") {
          if (
            node.tagName === "table" &&
            !parent.properties?.className?.includes("table-wrap")
          ) {
            parent.children[index] = {
              type: "element",
              tagName: "div",
              properties: { className: ["table-wrap"] },
              children: [node],
            };
          }
          if (
            node.tagName === "sup" &&
            node.children?.some(
              (child) =>
                child.tagName === "a" &&
                Object.hasOwn(child.properties ?? {}, "dataFootnoteRef"),
            )
          )
            addClass(node, "footnote-ref");
          if (
            node.tagName === "a" &&
            Object.hasOwn(node.properties ?? {}, "dataFootnoteBackref")
          )
            addClass(node, "footnote-back");
        }
        visit(node);
      }
    }
    visit(tree);
  };
}

function addClass(node, name) {
  node.properties ??= {};
  const classes = node.properties.className ?? [];
  node.properties.className = [
    ...classes.filter((value) => value !== name),
    name,
  ];
}
