/* Theme, palette, code copy, and table-of-contents highlight for the visual mockup. */
(function () {
  var root = document.documentElement;
  root.classList.add("js");
  var themeKey = "cfgb-theme";
  var paletteKey = "cfgb-palette";
  var modes = { system: 1, light: 1, dark: 1 };
  var palettes = { classic: 1, cyber: 1, dope: 1, forest: 1, dusk: 1 };
  var named = { cyber: 1, dope: 1, forest: 1, dusk: 1 };
  var ja = document.documentElement.lang === "ja";
  var copyLabel = ja ? "コピー" : "Copy";
  var copiedLabel = ja ? "コピーしました" : "Copied";

  function currentMode() {
    var attr = root.getAttribute("data-theme");
    if (attr === "light" || attr === "dark") return attr;
    var saved = "system";
    try { saved = localStorage.getItem(themeKey) || "system"; } catch (e) {}
    return modes[saved] ? saved : "system";
  }

  function currentPalette() {
    var attr = root.getAttribute("data-palette");
    if (named[attr]) return attr;
    var saved = "classic";
    try { saved = localStorage.getItem(paletteKey) || "classic"; } catch (e) {}
    return palettes[saved] ? saved : "classic";
  }

  function applyMode(mode) {
    if (mode === "system") root.removeAttribute("data-theme");
    else root.setAttribute("data-theme", mode);
    var select = document.getElementById("theme");
    if (select) select.value = mode;
  }

  function applyPalette(name) {
    if (named[name]) root.setAttribute("data-palette", name);
    else root.removeAttribute("data-palette");
    var select = document.getElementById("palette");
    if (select) select.value = palettes[name] ? name : "classic";
  }

  applyMode(currentMode());
  applyPalette(currentPalette());

  var themeSelect = document.getElementById("theme");
  if (themeSelect) {
    themeSelect.addEventListener("change", function () {
      var next = modes[themeSelect.value] ? themeSelect.value : "system";
      try { localStorage.setItem(themeKey, next); } catch (e) {}
      applyMode(next);
    });
  }

  var paletteSelect = document.getElementById("palette");
  if (paletteSelect) {
    paletteSelect.addEventListener("change", function () {
      var next = palettes[paletteSelect.value] ? paletteSelect.value : "classic";
      try { localStorage.setItem(paletteKey, next); } catch (e) {}
      applyPalette(next);
    });
  }

  document.querySelectorAll(".copy").forEach(function (el) {
    el.textContent = copyLabel;
    el.addEventListener("click", function () {
      var code = el.closest(".code").querySelector("pre");
      var text = "";
      if (code) {
        var clone = code.cloneNode(true);
        clone.querySelectorAll(".ln").forEach(function (node) { node.remove(); });
        text = clone.innerText.replace(/\n$/, "");
      }
      var done = function () {
        el.textContent = copiedLabel;
        window.setTimeout(function () { el.textContent = copyLabel; }, 1200);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, done);
      } else {
        done();
      }
    });
  });

  var form = document.querySelector("[data-search-mock]");
  if (form) {
    var query = form.querySelector("[name=q]");
    var topic = form.querySelector("[name=topic]");
    var year = form.querySelector("[name=year]");
    var rows = Array.prototype.slice.call(document.querySelectorAll("[data-result]"));
    var applyFilter = function () {
      var q = (query.value || "").trim();
      rows.forEach(function (row) {
        var text = row.getAttribute("data-text") || "";
        var ok = (!q || text.indexOf(q) !== -1) &&
          (!topic.value || row.getAttribute("data-topic").split(" ").indexOf(topic.value) !== -1) &&
          (!year.value || row.getAttribute("data-year") === year.value);
        row.hidden = !ok;
      });
    };
    form.addEventListener("submit", function (event) {
      event.preventDefault();
      applyFilter();
    });
    topic.addEventListener("change", applyFilter);
    year.addEventListener("change", applyFilter);
    applyFilter();
  }

  var links = Array.prototype.slice.call(document.querySelectorAll(".toc a"));
  if (!links.length || !("IntersectionObserver" in window)) return;
  var byId = {};
  links.forEach(function (link) {
    var id = decodeURIComponent(link.getAttribute("href").slice(1));
    byId[id] = link;
  });
  var heads = Array.prototype.slice.call(document.querySelectorAll(".prose h2, .prose h3"));
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      if (!entry.isIntersecting) return;
      links.forEach(function (link) { link.removeAttribute("aria-current"); });
      var link = byId[entry.target.id];
      if (link) link.setAttribute("aria-current", "true");
    });
  }, { rootMargin: "0px 0px -70% 0px", threshold: 0 });
  heads.forEach(function (head) { observer.observe(head); });
})();
