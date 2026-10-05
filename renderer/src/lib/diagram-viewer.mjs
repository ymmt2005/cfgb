// Move the existing render target into a native modal, keeping one SVG and its
// IDs/source. Theme redraws still write to the same target while it is expanded.
export function createDiagramViewer(document) {
  const template = document.getElementById("cfgb-diagram-viewer");
  const window = document.defaultView;
  const buttons = new Map();
  let dialog, shell, viewport, canvas, output, fullscreen, message, active;

  function scaleTo(scale, fit = false) {
    if (!active) return;
    const x =
      (viewport.scrollLeft + viewport.clientWidth / 2 - canvas.offsetLeft) /
      (canvas.offsetWidth || 1);
    const y =
      (viewport.scrollTop + viewport.clientHeight / 2 - canvas.offsetTop) /
      (canvas.offsetHeight || 1);
    active.scale = scale;
    active.fit = fit;
    canvas.style.width = `${active.width * scale}px`;
    canvas.style.height = `${active.height * scale}px`;
    output.value = new Intl.NumberFormat(document.documentElement.lang, {
      style: "percent",
      maximumFractionDigits: 0,
    }).format(scale);
    viewport.scrollLeft =
      x * canvas.offsetWidth + canvas.offsetLeft - viewport.clientWidth / 2;
    viewport.scrollTop =
      y * canvas.offsetHeight + canvas.offsetTop - viewport.clientHeight / 2;
  }

  function fit(minimum = 0) {
    if (!active) return;
    const style = window.getComputedStyle(viewport);
    const width = Math.max(
      1,
      viewport.clientWidth -
        parseFloat(style.paddingLeft) -
        parseFloat(style.paddingRight),
    );
    const height = Math.max(
      1,
      viewport.clientHeight -
        parseFloat(style.paddingTop) -
        parseFloat(style.paddingBottom),
    );
    active.minimum = minimum;
    scaleTo(
      Math.max(minimum, Math.min(width / active.width, height / active.height)),
      true,
    );
  }

  function measure() {
    const svg = active.block.querySelector("svg");
    const box = svg.viewBox.baseVal;
    const bounds = svg.getBoundingClientRect();
    active.width = box.width || bounds.width || 1;
    active.height = box.height || bounds.height || 1;
    if (active.fit) fit(active.minimum);
    else scaleTo(active.scale);
  }

  async function close() {
    if (document.fullscreenElement === shell) {
      try {
        await document.exitFullscreen();
      } catch {
        message.hidden = false;
        return;
      }
    }
    dialog.close();
    restore();
  }

  function restore() {
    if (!active) return;
    const { block, placeholder, button } = active;
    const error = block.nextElementSibling?.classList.contains("mermaid-error")
      ? block.nextElementSibling
      : null;
    placeholder.replaceWith(block);
    if (error) block.after(error);
    active = null;
    if (button.isConnected) button.focus({ preventScroll: true });
    else {
      block.tabIndex = -1;
      block.focus({ preventScroll: true });
    }
  }

  function create() {
    dialog = template.content.firstElementChild.cloneNode(true);
    shell = dialog.querySelector(".diagram-viewer-shell");
    viewport = dialog.querySelector(".diagram-viewport");
    canvas = dialog.querySelector(".diagram-canvas");
    output = dialog.querySelector("output");
    message = dialog.querySelector(".diagram-message");
    fullscreen = dialog.querySelector("[data-diagram-fullscreen]");
    fullscreen.hidden = !document.fullscreenEnabled || !shell.requestFullscreen;
    dialog
      .querySelector("[data-diagram-close]")
      .addEventListener("click", close);
    dialog
      .querySelector("[data-diagram-fit]")
      .addEventListener("click", () => fit());
    dialog
      .querySelector("[data-diagram-actual]")
      .addEventListener("click", () => scaleTo(1));
    dialog
      .querySelector("[data-diagram-zoom-in]")
      .addEventListener("click", () => scaleTo(active.scale * 1.25));
    dialog
      .querySelector("[data-diagram-zoom-out]")
      .addEventListener("click", () => scaleTo(active.scale / 1.25));
    fullscreen.addEventListener("click", async () => {
      message.hidden = true;
      try {
        if (document.fullscreenElement === shell)
          await document.exitFullscreen();
        else await shell.requestFullscreen();
      } catch {
        message.hidden = false;
      }
    });
    document.addEventListener("fullscreenchange", () => {
      const entered = document.fullscreenElement === shell;
      fullscreen.textContent = entered
        ? fullscreen.dataset.exitLabel
        : fullscreen.dataset.enterLabel;
      fullscreen.setAttribute("aria-pressed", String(entered));
    });
    dialog.addEventListener("cancel", (event) => {
      event.preventDefault();
      close();
    });
    dialog.addEventListener("close", () => {
      if (!dialog.open) restore();
    });
    const resize = () => {
      if (active?.fit) fit(active.minimum);
    };
    if (window.ResizeObserver)
      new window.ResizeObserver(resize).observe(viewport);
    else window.addEventListener("resize", resize);
    document.body.append(dialog);
  }

  function open(block, button) {
    if (active) return;
    if (!dialog) create();
    const placeholder = document.createElement("div");
    placeholder.style.height = `${block.getBoundingClientRect().height}px`;
    block.before(placeholder);
    active = {
      block,
      button,
      placeholder,
      width: 1,
      height: 1,
      scale: 1,
      fit: true,
      // Expand small diagrams to use the window, but keep large labels at
      // least their authored size. Fit-to-window is an explicit overview.
      minimum: 1,
    };
    canvas.replaceChildren(block);
    message.hidden = true;
    dialog.showModal();
    measure();
    viewport.scrollLeft = 0;
    viewport.scrollTop = 0;
  }

  return {
    update(block) {
      let button = buttons.get(block);
      if (!block.querySelector("svg")) {
        button?.remove();
        buttons.delete(block);
        if (active?.block === block) close();
        return;
      }
      if (!button) {
        button = document.createElement("button");
        button.type = "button";
        button.className = "diagram-expand";
        button.textContent = template.dataset.expandLabel;
        button.setAttribute("aria-haspopup", "dialog");
        button.setAttribute("data-pagefind-ignore", "");
        button.addEventListener("click", () => open(block, button));
        block.before(button);
        buttons.set(block, button);
      }
      if (active?.block === block) measure();
    },
  };
}
