// Mermaid expects resolved color values rather than CSS custom properties.
// Read the current page tokens on every draw so system/explicit themes agree
// with prose and the approved diagram frame.
export function mermaidTheme(document) {
  const root = document.documentElement;
  const style = document.defaultView.getComputedStyle(root);
  const token = (name) => style.getPropertyValue(`--${name}`).trim();
  const ink = token("ink");
  const line = token("line");
  const background = token("bg-raised");
  return {
    theme: "base",
    look: "classic",
    fontFamily: token("serif"),
    themeVariables: {
      darkMode: style.colorScheme === "dark",
      background,
      fontFamily: token("serif"),
      fontSize: "18px",
      dropShadow: "none",
      primaryColor: token("soft"),
      primaryBorderColor: line,
      primaryTextColor: ink,
      secondaryColor: background,
      secondaryBorderColor: line,
      secondaryTextColor: ink,
      tertiaryColor: token("code-bg"),
      tertiaryBorderColor: line,
      tertiaryTextColor: ink,
      textColor: ink,
      lineColor: token("muted"),
      edgeLabelBackground: background,
      clusterBkg: background,
      clusterBorder: line,
      titleColor: ink,
      actorBkg: token("soft"),
      actorBorder: line,
      actorTextColor: ink,
      actorLineColor: token("muted"),
      signalColor: token("muted"),
      signalTextColor: ink,
      labelBoxBkgColor: background,
      labelBoxBorderColor: line,
      labelTextColor: ink,
      loopTextColor: ink,
      noteBkgColor: token("code-bg"),
      noteBorderColor: line,
      noteTextColor: ink,
    },
  };
}
