import assert from "node:assert/strict";
import test from "node:test";
import { watchSystemTheme } from "../src/lib/system-theme.mjs";

test("system theme changes redraw only while the page is in system mode", () => {
  const listeners = [];
  const media = {
    addEventListener(_type, listener) {
      listeners.push(listener);
    },
    removeEventListener(_type, listener) {
      const index = listeners.indexOf(listener);
      if (index >= 0) listeners.splice(index, 1);
    },
  };
  let system = true;
  const draws = [];
  const stop = watchSystemTheme(media, () => system, () => {
    draws.push("draw");
  });
  listeners[0]();
  system = false;
  listeners[0]();
  system = true;
  listeners[0]();
  stop();
  listeners[0]?.();
  assert.deepEqual(draws, ["draw", "draw"]);
});
