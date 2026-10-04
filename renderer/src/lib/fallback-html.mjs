import { sitePath } from "./site-path.mjs";
import { copyFor, rootNotFoundTitle } from "./load-site.mjs";

export function fallbackHtml(corpus, locale) {
  const locales = Object.keys(corpus.site.locales);
  const bilingual = locale === "both";
  const lang = bilingual ? corpus.site.defaultLocale : locale;
  const shown = bilingual ? locales : [locale];
  const copy = shown
    .map((item) => {
      const label = escapeHtml(corpus.site.locales[item].label);
      const search = escapeHtml(copyFor(item).search);
      return `<p><a href="${escapeHtml(sitePath(corpus.site, `/${item}/`))}">${label}</a> · <a href="${escapeHtml(sitePath(corpus.site, `/${item}/search/`))}">${search}</a></p>`;
    })
    .join("");
  const title = bilingual ? rootNotFoundTitle() : copyFor(locale).notFound;
  return `<!DOCTYPE html>
<html lang="${lang}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title} · ${escapeHtml(corpus.site.title)}</title>
<link rel="stylesheet" href="${escapeHtml(sitePath(corpus.site, "/assets/site.css"))}">
</head>
<body data-pagefind-ignore="all">
<main id="content" class="wrap page-narrow">
<h1>${title}</h1>
${copy}
</main>
</body>
</html>
`;
}

function escapeHtml(value) {
  return String(value).replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;");
}
