// Tests for the first-visit locale redirect. They execute the exact string
// the site ships (redirectSnippet output) inside a node:vm sandbox, so a
// change to the snippet cannot drift away from what was verified.
import { test } from "node:test";
import assert from "node:assert/strict";
import { runInNewContext } from "node:vm";
import { redirectSnippet } from "./redirect.mjs";

const BASE = "/Playsmith/";
const SNIPPET = redirectSnippet(BASE);

function sandbox({ language = "en-US", path = BASE, search = "", hash = "", stored = null } = {}) {
  const storage = new Map();
  if (stored !== null) storage.set("ps-locale", stored);
  const target = { replaced: null };
  const sb = {
    navigator: { language },
    localStorage: {
      getItem: (k) => (storage.has(k) ? storage.get(k) : null),
      setItem: (k, v) => storage.set(k, String(v)),
    },
    location: {
      pathname: path,
      search,
      hash,
      replace: (url) => {
        target.replaced = url;
      },
    },
  };
  return { sb, storage, target };
}

function run(sb) {
  runInNewContext(SNIPPET, sb);
}

test("zh browser first visit on an English page is redirected to the zh twin", () => {
  const { sb, target, storage } = sandbox({ language: "zh-CN", path: BASE + "features/" });
  run(sb);
  assert.equal(target.replaced, BASE + "zh/features/");
  assert.equal(storage.get("ps-locale"), "auto");
});

test("zh-TW browsers count as zh too", () => {
  const { sb, target } = sandbox({ language: "zh-TW", path: BASE + "build/" });
  run(sb);
  assert.equal(target.replaced, BASE + "zh/build/");
});

test("a zh browser already on a zh page stays put", () => {
  const { sb, target, storage } = sandbox({ language: "zh-CN", path: BASE + "zh/features/" });
  run(sb);
  assert.equal(target.replaced, null);
  assert.equal(storage.get("ps-locale"), "auto");
});

test("non-zh browsers are never redirected", () => {
  const { sb, target, storage } = sandbox({ language: "en-US", path: BASE + "features/" });
  run(sb);
  assert.equal(target.replaced, null);
  assert.equal(storage.get("ps-locale"), "auto");
});

test("second visit never redirects, even for a zh browser on an English page", () => {
  const { sb, target, storage } = sandbox({
    language: "zh-CN",
    path: BASE + "features/",
    stored: "auto",
  });
  run(sb);
  assert.equal(target.replaced, null);
  assert.equal(storage.get("ps-locale"), "auto");
});

test("asset-looking paths are not redirected", () => {
  for (const p of [BASE + "assets/logo.png", BASE + "sitemap.xml", BASE + "robots.txt"]) {
    const { sb, target } = sandbox({ language: "zh-CN", path: p });
    run(sb);
    assert.equal(target.replaced, null, p);
  }
});

test("the site root is redirected to the zh root", () => {
  const { sb, target } = sandbox({ language: "zh-CN", path: BASE });
  run(sb);
  assert.equal(target.replaced, BASE + "zh/");
});

test("query and hash survive the redirect", () => {
  const { sb, target } = sandbox({
    language: "zh-CN",
    path: BASE + "features/",
    search: "?x=1",
    hash: "#anchors",
  });
  run(sb);
  assert.equal(target.replaced, BASE + "zh/features/?x=1#anchors");
});

test("paths outside the configured base are left alone", () => {
  const { sb, target } = sandbox({ language: "zh-CN", path: "/elsewhere/features/" });
  run(sb);
  assert.equal(target.replaced, null);
});

test("a blocked localStorage (privacy mode) must not throw or redirect", () => {
  const calls = [];
  const sb = {
    navigator: { language: "zh-CN" },
    location: {
      pathname: BASE + "features/",
      search: "",
      hash: "",
      replace: (url) => calls.push(url),
    },
    // no localStorage at all: the snippet's try/catch must swallow the
    // ReferenceError instead of breaking the page
  };
  assert.doesNotThrow(() => run(sb));
  assert.equal(calls.length, 0);
});
