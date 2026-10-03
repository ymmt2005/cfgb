export function fallbackHtml(corpus, locale) {
  const locales = Object.keys(corpus.site.locales);
  const bilingual = locale === "both";
  const lang = bilingual ? corpus.site.defaultLocale : locale;
  const shown = bilingual ? locales : [locale];
  const copy = shown
    .map((item) => {
      const label = escapeHtml(corpus.site.locales[item].label);
      const search = item === "ja" ? "検索" : "Search";
      return `<p><a href="/${item}/">${label}</a> · <a href="/${item}/search/">${search}</a></p>`;
    })
    .join("");
  const title = bilingual ? "Page not found" : locale === "ja" ? "ページが見つかりません" : "Page not found";
  return `<!DOCTYPE html>
<html lang="${lang}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${title} · ${escapeHtml(corpus.site.title)}</title>
<link rel="stylesheet" href="/assets/site.css">
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
