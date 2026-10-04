// watchSystemTheme redraws theme-dependent content when the operating system
// color scheme changes. An explicit light or dark choice does not follow that
// change; only system mode, which has no data-theme attribute, does.
export function watchSystemTheme(media, isSystemMode, redraw) {
  const onChange = () => {
    if (isSystemMode()) redraw();
  };
  if (typeof media.addEventListener === "function") {
    media.addEventListener("change", onChange);
  } else if (typeof media.addListener === "function") {
    media.addListener(onChange);
  }
  return () => {
    if (typeof media.removeEventListener === "function") media.removeEventListener("change", onChange);
    else if (typeof media.removeListener === "function") media.removeListener(onChange);
  };
}
