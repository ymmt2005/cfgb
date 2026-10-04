(function () {
  try {
    var query = new URLSearchParams(location.search);
    var theme = query.get("theme");
    if (theme !== "light" && theme !== "dark") theme = localStorage.getItem("cfgb-theme");
    if (theme === "light" || theme === "dark") document.documentElement.setAttribute("data-theme", theme);
  } catch (error) {}
})();
