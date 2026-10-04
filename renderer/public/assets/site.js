(function () {
  var root = document.documentElement;
  root.classList.add("js");
  var themeKey = "cfgb-theme";
  var modes = new Set(["system", "light", "dark"]);

  function currentMode() {
    var attr = root.getAttribute("data-theme");
    if (attr === "light" || attr === "dark") return attr;
    var saved = "system";
    try { saved = localStorage.getItem(themeKey) || "system"; } catch (error) {}
    return modes.has(saved) ? saved : "system";
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
    mark(document.getElementById("cfgb-theme-list"), mode);
    document.dispatchEvent(new CustomEvent("cfgb-theme"));
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
  bindMenu("cfgb-theme", function (value) {
    var next = modes.has(value) ? value : "system";
    try { localStorage.setItem(themeKey, next); } catch (error) {}
    applyMode(next);
  });
  document.addEventListener("click", function (event) {
    if (event.target.closest(".menu")) return;
    closeMenus(null);
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") closeMenus(null);
  });

  var links = Array.prototype.slice.call(document.querySelectorAll(".toc a, .toc-mobile a"));
  if (!links.length || !("IntersectionObserver" in window)) return;
  var byId = new Map();
  links.forEach(function (link) {
    var id = decodeURIComponent(link.getAttribute("href").slice(1));
    if (!byId.has(id)) byId.set(id, []);
    byId.get(id).push(link);
  });
  var heads = Array.prototype.slice.call(document.querySelectorAll(".prose h2, .prose h3"));
  var observer = new IntersectionObserver(function (entries) {
    entries.forEach(function (entry) {
      if (!entry.isIntersecting) return;
      links.forEach(function (link) { link.removeAttribute("aria-current"); });
      var current = byId.get(entry.target.id);
      if (!current) return;
      current.forEach(function (link) {
        link.setAttribute("aria-current", "true");
      });
    });
  }, { rootMargin: "0px 0px -70% 0px", threshold: 0 });
  heads.forEach(function (head) { observer.observe(head); });
})();
