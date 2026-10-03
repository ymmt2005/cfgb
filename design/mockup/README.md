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

The header theme control cycles system, light, and dark. It stores the choice in `localStorage` under `cfgb-theme` and applies it before paint. Add `?theme=dark` or `?theme=light` to open a page in that theme. With JavaScript disabled, the page follows the system theme, the diagram source stays visible, and the table of contents remains a list of links.

Regenerate the HTML after editing `render.py`. The script copies `bars.png` and `schema.svg` from the example corpus when that checkout is available.

## Visual direction

Article prose uses a Japanese mincho: Hiragino Mincho, Yu Mincho, then Noto Serif CJK JP. The header, navigation, and headings use a Japanese gothic: Hiragino Sans, Yu Gothic, Meiryo, then Noto Sans CJK JP. Those names come before any generic family so a Chinese face is not chosen for characters such as 図. Latin faces sit after the Japanese ones. Code uses `ui-monospace`. No web fonts are requested.

The reading column is 40rem. On a desktop article the table of contents is a 15rem sticky column; below 860px it becomes a disclosure above the body. The page padding is 1.25rem. Home and list pages keep the same column rather than stretching summaries across the viewport. Vertical rhythm is about 1.15–1.8 inside prose, with section headings separated by roughly 2rem.

The header is the site name, a Japanese/English switch, and the theme control, then a second row of section links. The current language stays on this page. The other language opens the paired article when one exists, and that locale’s home when it does not. An untranslated article says so in the translation note.

The palette is warm paper and ink, with a copper link color. Dark theme uses the same structure and a lighter copper, not a separate layout. Code marks are a background band plus an underline on the marked token. Alerts are a left rule and a label, not filled boxes. Diagrams use `currentColor` so the text stays visible in both themes.
