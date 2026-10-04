(function () {
  if (!window.PagefindUI) return;
  const lang = document.documentElement.lang;
  const search = new window.PagefindUI({
    element: "#search",
    showSubResults: true,
    bundlePath: `${document.documentElement.dataset?.cfgbBasePath || ""}/pagefind/`,
    baseUrl: `${document.documentElement.dataset?.cfgbBasePath || ""}/`,
  });
  if (lang) search.triggerFilters({ locale: [lang] });
})();
