// A query also appears in Pagefind's loading message. Wait for the completed
// result list (including an empty list), with result data materialized.
export async function waitForSearch(page, query) {
  await page.waitForFunction(query => {
    const results = document.querySelector(".pagefind-ui__results");
    const message = document.querySelector(".pagefind-ui__message")?.textContent || "";
    return results && !results.querySelector(".pagefind-ui__loading") && message.includes(query);
  }, query);
}
