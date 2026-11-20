// First-visit locale redirect for the docs site.
//
// The site's baseline is manual switching (locale menu), which stays the only
// thing that can change language after the first visit: this snippet runs
// once per full page load, and only redirects when no stored decision exists.
// A zh* browser landing on an English page is moved to its /zh/ twin; the
// one-shot localStorage flag keeps the jump from repeating (and from fighting
// a user who later switches back manually).
//
// The logic ships as this exact string, so the tests execute the same source
// the browser gets (node:vm sandbox), not a paraphrase of it.

const STORAGE_KEY = "ps-locale";

export function redirectSnippet(base) {
  return [
    "(function () {",
    "  try {",
    '    var KEY = "' + STORAGE_KEY + '";',
    "    if (localStorage.getItem(KEY)) return;",
    '    localStorage.setItem(KEY, "auto");',
    '    var lang = (navigator.language || "").toLowerCase();',
    '    if (lang.indexOf("zh") !== 0) return;',
    "    var b = " + JSON.stringify(base) + ";",
    "    var path = location.pathname;",
    '    if (/\\.[a-z0-9]+$/i.test(path)) return;',
    '    if (path.indexOf(b + "zh/") === 0 || path === b + "zh") return;',
    '    if (path.indexOf(b) !== 0) return;',
    '    location.replace(b + "zh/" + path.slice(b.length) + location.search + location.hash);',
    "  } catch (e) {}",
    "})();",
  ].join("\n");
}
