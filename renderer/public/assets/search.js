(function () {
  if (!window.PagefindUI) return;
  const lang = document.documentElement.lang;
  const search = new window.PagefindUI({
    element: "#search",
    showSubResults: true,
    bundlePath: "/pagefind/",
  });
  if (lang) search.triggerFilters({ locale: [lang] });
})();
