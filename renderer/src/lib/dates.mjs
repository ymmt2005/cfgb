import catalog from "./locales.json" with { type: "json" };

export function localeEntry(locale) {
  if (!Object.hasOwn(catalog.locales, locale))
    throw new Error(`locale ${locale} is not supported`);
  return catalog.locales[locale];
}

// Omit timeZone for the reader's browser zone; static rendering passes UTC.
export function formatDate(iso, locale, timeZone) {
  const entry = localeEntry(locale);
  const date = new Date(iso);
  if (entry.date.form === "ymd-kanji") {
    const parts = new Intl.DateTimeFormat(entry.date.intl, {
      timeZone,
      year: "numeric",
      month: "numeric",
      day: "numeric",
    }).formatToParts(date);
    const value = (type) => parts.find((part) => part.type === type).value;
    return `${value("year")}年${value("month")}月${value("day")}日`;
  }
  if (entry.date.form === "intl-medium") {
    return new Intl.DateTimeFormat(entry.date.intl, {
      timeZone,
      day: "numeric",
      month: "short",
      year: "numeric",
    }).format(date);
  }
  throw new Error(`locale ${locale} has no date form`);
}

export function localizeDates(root) {
  for (const element of root.querySelectorAll("time[data-cfgb-date]")) {
    element.textContent = formatDate(
      element.dateTime,
      element.dataset.cfgbDate,
    );
  }
}
