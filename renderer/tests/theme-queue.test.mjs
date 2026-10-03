import assert from "node:assert/strict";
import test from "node:test";
import { createThemeQueue } from "../src/lib/theme-queue.mjs";

test("a delayed redraw yields to the latest theme and keeps the error path available", async () => {
  let release = () => {};
  const gate = new Promise((resolve) => {
    release = resolve;
  });
  const applied = [];
  const request = createThemeQueue(async (current) => {
    await gate;
    if (!current()) return;
    applied.push("latest");
  });
  const first = request();
  const second = request();
  const third = request();
  release();
  await Promise.all([first, second, third]);
  assert.deepEqual(applied, ["latest"]);

  const failures = [];
  const failing = createThemeQueue(async () => {
    throw new Error("diagram failed");
  });
  await failing();
  failures.push("returned");
  assert.deepEqual(failures, ["returned"]);
});
