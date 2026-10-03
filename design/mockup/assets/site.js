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

  function mark(list, value) {
    if (!list) return;
    list.querySelectorAll("[role=option]").forEach(function (option) {
      option.setAttribute("aria-selected", option.getAttribute("data-value") === value ? "true" : "false");
    });
  }

  function applyMode(mode) {
    if (mode === "system") root.removeAttribute("data-theme");
    else root.setAttribute("data-theme", mode);
    mark(document.getElementById("theme-list"), mode);
  }

  function applyPalette(name) {
    if (!palettes[name]) name = "classic";
    if (named[name]) root.setAttribute("data-palette", name);
    else root.removeAttribute("data-palette");
    mark(document.getElementById("palette-list"), name);
  }

  function closeMenus(except) {
    document.querySelectorAll(".menu-list").forEach(function (list) {
      if (list === except) return;
      list.hidden = true;
      var button = document.querySelector('[aria-controls="' + list.id + '"]');
      if (button) button.setAttribute("aria-expanded", "false");
    });
  }

  function bindMenu(buttonId, choose) {
    var button = document.getElementById(buttonId);
    if (!button) return;
    var list = document.getElementById(button.getAttribute("aria-controls"));
    if (!list) return;

    function open() {
      closeMenus(list);
      list.hidden = false;
      button.setAttribute("aria-expanded", "true");
      var current = list.querySelector('[aria-selected="true"]') || list.querySelector("[role=option]");
      if (current) current.focus();
    }

    function close(focusButton) {
      list.hidden = true;
      button.setAttribute("aria-expanded", "false");
      if (focusButton) button.focus();
    }

    button.addEventListener("click", function () {
      if (list.hidden) open();
      else close(false);
    });

    button.addEventListener("keydown", function (event) {
      if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
      event.preventDefault();
      open();
    });

    list.addEventListener("click", function (event) {
      var option = event.target.closest("[role=option]");
      if (!option) return;
      choose(option.getAttribute("data-value"));
      close(true);
    });

    list.addEventListener("keydown", function (event) {
      var options = Array.prototype.slice.call(list.querySelectorAll("[role=option]"));
      var index = options.indexOf(document.activeElement);
      if (event.key === "ArrowDown" || event.key === "ArrowUp") {
        event.preventDefault();
        var step = event.key === "ArrowDown" ? 1 : -1;
        options[(index + step + options.length) % options.length].focus();
      } else if (event.key === "Home") {
        event.preventDefault();
        options[0].focus();
      } else if (event.key === "End") {
        event.preventDefault();
        options[options.length - 1].focus();
      } else if (event.key === "Escape") {
        event.preventDefault();
        close(true);
      } else if (event.key === "Enter" || event.key === " ") {
        event.preventDefault();
        if (index >= 0) {
          choose(options[index].getAttribute("data-value"));
          close(true);
        }
      }
    });
  }

  applyMode(currentMode());
  applyPalette(currentPalette());

  bindMenu("theme", function (value) {
    var next = modes[value] ? value : "system";
    try { localStorage.setItem(themeKey, next); } catch (e) {}
    applyMode(next);
  });
  bindMenu("palette", function (value) {
    var next = palettes[value] ? value : "classic";
    try { localStorage.setItem(paletteKey, next); } catch (e) {}
    applyPalette(next);
  });

  document.addEventListener("click", function (event) {
    if (event.target.closest(".menu")) return;
    closeMenus(null);
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") closeMenus(null);
  });

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
