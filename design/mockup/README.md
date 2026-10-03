# CFGB visual mockup

A browsable HTML/CSS prototype of the finished example blog. It uses the synthetic articles in `cfgb-example` and does not call Cloudflare, a model, or the renderer. Search results on this prototype are labeled mock data.

## View

From the `cfgb` repository:

```sh
python3 design/mockup/render.py
python3 -m http.server 4173 --directory design/mockup
```

Open `http://127.0.0.1:4173/ja/`.

| Page | URL |
| --- | --- |
| Locale choice for `/` | `/` |
| Japanese home | `/ja/` |
| English home | `/en/` |
| Long article | `/ja/posts/markdown-showcase/` |
| Link card and mixed prose | `/ja/posts/protobuf-schema-guide/` |
| Untranslated language switch | `/ja/posts/post-quantum-notes/` |
| Post list | `/ja/posts/` |
| September archive | `/ja/archive/2026/09/` |
| October archive (timezone boundary) | `/ja/archive/2026/10/` |
| Topic | `/ja/topics/protobuf/` |
| Search | `/ja/search/` |
| About | `/ja/about/` |

The header has two dropdowns. The closed controls read **テーマ** / **Theme** and **配色** / **Palette** only. The current choice is marked inside the open menu, which uses the page colors, so it is dark in the dark theme. Theme chooses system, light, or dark. Palette chooses Classic, Cyber, Dope, Forest, or Dusk. Classic is the warm paper palette. The others are separate color systems, each with a light and a dark pair. Choices are stored in `localStorage` under `cfgb-theme` and `cfgb-palette` and applied before paint. Add `?theme=dark` or `?theme=light`, and `?palette=cyber` (or `dope`, `forest`, `dusk`), to open a page that way. With JavaScript disabled, the page follows the system theme in Classic, the diagram source stays visible, and the table of contents stays an expanded list that can still be collapsed. These palettes are for appearance review. The production theme contract is still system, light, and dark.

Regenerate the HTML after editing `render.py`. The script copies `bars.png` and `schema.svg` from the example corpus when that checkout is available.

## Visual direction

Article prose uses a Japanese mincho: Hiragino Mincho, Yu Mincho, then Noto Serif CJK JP. The header, navigation, and headings use a Japanese gothic: Hiragino Sans, Yu Gothic, Meiryo, then Noto Sans CJK JP. Those names come before any generic family so a Chinese face is not chosen for characters such as 図. Latin faces sit after the Japanese ones. Code uses `ui-monospace`. No web fonts are requested.

The reading column is 40rem. On a wide article the table of contents is a 15rem sticky column on the left. It opens expanded, and the heading collapses or expands it. While it is collapsed, the article uses that column’s width. Below 1220px that same expanded disclosure sits above the body. The right column is the rendered Markdown from `cfgb-example/src/content/aside/<locale>.md`. The mockup does not add a link-list component. A missing file omits the column. Below 960px that column moves under the page. The page padding is 1.25rem. Home and list pages keep the same reading column. Vertical rhythm is about 1.15–1.8 inside prose, with section headings separated by roughly 2rem.

The header is the site name, a Japanese/English switch, and the theme and palette dropdowns, then a second row of section links. The current language stays on this page. The other language opens the paired article when one exists, and that locale’s home when it does not. An untranslated article says so in the translation note.

Classic is warm paper and ink, with a copper link color. Cyber is phosphor and magenta. Dope is peach, crimson, and acid yellow. Forest is sage paper with a persimmon link. Dusk is lilac and violet. Dark mode keeps the same structure and swaps in that palette’s dark pair. Code marks are a background band plus an underline on the marked token. Alerts are a left rule and a label, not filled boxes. Diagrams use `currentColor` so the text stays visible in both themes.
