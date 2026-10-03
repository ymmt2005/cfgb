#!/usr/bin/env python3
"""Generate the browsable CFGB visual mockup. Content is transcribed from cfgb-example."""

from pathlib import Path

ROOT = Path(__file__).resolve().parent
EXAMPLE = ROOT.parents[2] / "cfgb-example"
MEDIA = {
    "bars.png": EXAMPLE / "src/content/posts/2026/2026-09-20-markdown-showcase/assets/bars.png",
    "schema.svg": EXAMPLE / "src/content/posts/2026/2026-09-19-protobuf-guide/assets/schema.svg",
}

TOPICS = {
    "protobuf": {"ja": "Protocol Buffers", "en": "Protocol Buffers"},
    "oss": {"ja": "オープンソース", "en": "Open Source"},
    "software-engineering": {"ja": "ソフトウェアエンジニアリング", "en": "Software Engineering"},
    "security": {"ja": "セキュリティ", "en": "Security"},
    "ai": {"ja": "AI", "en": "AI"},
    "writing": {"ja": "文章を書く", "en": "Writing"},
}

NAV = {
    "ja": [("ホーム", "/ja/"), ("記事", "/ja/posts/"), ("アーカイブ", "/ja/archive/2026/09/"), ("トピック", "/ja/topics/protobuf/"), ("検索", "/ja/search/"), ("About", "/ja/about/")],
    "en": [("Home", "/en/"), ("Posts", "/en/posts/"), ("Archive", "/en/archive/2026/09/"), ("Topics", "/en/topics/protobuf/"), ("Search", "/en/search/"), ("About", "/en/about/")],
}

POSTS = [
    {
        "locale": "ja",
        "title": "月別アーカイブとタイムゾーン",
        "date": "2026年10月1日",
        "sort": "2026-10-01",
        "year": "2026",
        "month": "10",
        "url": "/ja/posts/archive-timezone-boundary/",
        "summary": "月別アーカイブをサイトのタイムゾーンで分類するためのサンプルです。UTC では九月末、日本時間では十月初めになる公開日時を使い、記事ディレクトリの年や名前と、公開月が別になり得ることを確認します。",
        "topics": ["software-engineering"],
        "alt": "/en/",
        "alt_note": "英語版はありません。言語を切り替えると英語のホームへ移動します。",
    },
    {
        "locale": "ja",
        "title": "認証認可の境界を整理する",
        "date": "2026年9月22日",
        "sort": "2026-09-22",
        "year": "2026",
        "month": "09",
        "url": "/ja/posts/authentication-and-authorization/",
        "summary": "認証認可の説明で混ざりやすい、利用者の確認と操作権限の判断を整理します。OAuth、OIDC、RBAC、ABAC を会話の見出しとして置き、境界ごとに何を確認するかを分ける架空のメモです。",
        "topics": ["security", "software-engineering"],
        "alt": "/en/",
        "alt_note": "英語版はありません。言語を切り替えると英語のホームへ移動します。",
    },
    {
        "locale": "ja",
        "title": "耐量子暗号を読むためのメモ",
        "date": "2026年9月21日",
        "sort": "2026-09-21",
        "year": "2026",
        "month": "09",
        "url": "/ja/posts/post-quantum-notes/",
        "summary": "耐量子暗号を学ぶときに、鍵共有とデータ暗号化を区別するための短いメモです。量子コンピュータ、ML-KEM、ハイブリッド方式という検索語を含め、技術用語を日本語で探せるかを確認するサンプルです。",
        "topics": ["security"],
        "alt": "/en/",
        "alt_note": "英語版はありません。言語を切り替えると英語のホームへ移動します。",
    },
    {
        "locale": "ja",
        "title": "Markdown 表現の確認",
        "date": "2026年9月20日",
        "sort": "2026-09-20",
        "year": "2026",
        "month": "09",
        "url": "/ja/posts/markdown-showcase/",
        "summary": "表、注意書き、脚注、コードのファイル名と行強調、Mermaid の図を一つの記事で確認するサンプルです。画像や HTML、内部リンクも含め、明暗テーマと JavaScript 無効時の表示を見比べます。",
        "topics": ["software-engineering"],
        "alt": "/en/posts/markdown-rendering-showcase/",
        "alt_note": "英語版があります。",
    },
    {
        "locale": "ja",
        "title": "Protocol Buffers のスキーマを読む",
        "date": "2026年9月19日",
        "sort": "2026-09-19",
        "year": "2026",
        "month": "09",
        "url": "/ja/posts/protobuf-schema-guide/",
        "summary": "Protocol Buffers のメッセージ、フィールド番号、型の参照を小さな例で確認します。スキーマの関係図と Go のコードを使い、変更時に確認したい互換性の観点を整理する、日本語と英語の対訳サンプルです。",
        "topics": ["protobuf", "oss"],
        "alt": "/en/posts/reading-protobuf-schemas/",
        "alt_note": "英語版があります。スラッグはロケールごとに独立しています。",
    },
    {
        "locale": "ja",
        "title": "短く書くために残すもの",
        "date": "2025年12月15日",
        "sort": "2025-12-15",
        "year": "2025",
        "month": "12",
        "url": "/ja/posts/what-to-keep-in-a-short-post/",
        "summary": "短い文章にするために何を削るかではなく、読者に何を持ち帰ってほしいかを考える架空の随筆です。結論を一つに絞り、細部を参照先へ置く方針を説明します。",
        "topics": ["writing"],
        "alt": "/en/",
        "alt_note": "英語版はありません。言語を切り替えると英語のホームへ移動します。",
    },
    {
        "locale": "en",
        "title": "Introducing the fictional example corpus",
        "date": "24 Sep 2026",
        "sort": "2026-09-24",
        "year": "2026",
        "month": "09",
        "url": "/en/posts/introducing-the-example-corpus/",
        "summary": "This fictional announcement introduces a bilingual content corpus for testing static publishing, feeds, and search. It is not a product release.",
        "topics": ["oss", "ai"],
        "alt": "/ja/",
        "alt_note": "No Japanese version. Switching language opens the Japanese home.",
    },
    {
        "locale": "en",
        "title": "Retries and event processing",
        "date": "23 Sep 2026",
        "sort": "2026-09-23",
        "year": "2026",
        "month": "09",
        "url": "/en/posts/retries-and-events/",
        "summary": "A fictional event-processing design separates delivery attempts from application success, so a retry does not pretend the work happened twice.",
        "topics": ["software-engineering"],
        "alt": "/ja/",
        "alt_note": "No Japanese version. Switching language opens the Japanese home.",
    },
    {
        "locale": "en",
        "title": "A Markdown rendering showcase",
        "date": "21 Sep 2026",
        "sort": "2026-09-21",
        "year": "2026",
        "month": "09",
        "url": "/en/posts/markdown-rendering-showcase/",
        "summary": "Exercise tables, alerts, footnotes, highlighted code, Mermaid diagrams, local images, and theme behavior in one English article.",
        "topics": ["software-engineering"],
        "alt": "/ja/posts/markdown-showcase/",
        "alt_note": "A Japanese version exists, with its own slug.",
    },
    {
        "locale": "en",
        "title": "Reading a Protocol Buffers schema",
        "date": "20 Sep 2026",
        "sort": "2026-09-20",
        "year": "2026",
        "month": "09",
        "url": "/en/posts/reading-protobuf-schemas/",
        "summary": "Read a small Protocol Buffers message, identify field numbers, and follow type references. The English slug differs from the Japanese article in the same group.",
        "topics": ["protobuf", "oss"],
        "alt": "/ja/posts/protobuf-schema-guide/",
        "alt_note": "A Japanese version exists, with its own slug and publication date.",
    },
]


def topic_label(locale, topic_id):
    return TOPICS[topic_id][locale]


def chips(locale, topics):
    return "".join(
        f'<a href="/{locale}/topics/{topic}/">{topic_label(locale, topic)}</a>'
        for topic in topics
    )


def layout(locale, title, body, current, alt_href, description, self_href=None):
    nav = []
    for label, href in NAV[locale]:
        current_attr = ' aria-current="page"' if href == current else ""
        nav.append(f'<a href="{href}"{current_attr}>{label}</a>')
    if self_href is None:
        self_href = current
    lang = locale
    skip = "本文へ" if locale == "ja" else "Skip to content"
    ja_href = self_href if locale == "ja" else alt_href
    en_href = alt_href if locale == "ja" else self_href
    theme_label = "システム" if locale == "ja" else "System"
    return f"""<!DOCTYPE html>
<html lang="{lang}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{title}</title>
<meta name="description" content="{description}">
<link rel="stylesheet" href="/assets/site.css">
<script>
(function(){{try{{var q=new URLSearchParams(location.search).get("theme");var t=q;if(t!=="light"&&t!=="dark")t=localStorage.getItem("cfgb-theme");if(t==="light"||t==="dark")document.documentElement.setAttribute("data-theme",t);}}catch(e){{}}}})();
</script>
</head>
<body>
<a class="skip" href="#content">{skip}</a>
<header class="mast">
  <div class="wrap">
    <div class="mast-row">
      <a class="brand" href="/{locale}/">CFGB <span>Example</span></a>
      <div class="tools">
        <nav class="locales" aria-label="{'言語' if locale == 'ja' else 'Language'}">
          <a href="{ja_href}" {'aria-current="true"' if locale == 'ja' else ''} hreflang="ja">日本語</a>
          <a href="{en_href}" {'aria-current="true"' if locale == 'en' else ''} hreflang="en">English</a>
        </nav>
        <button class="theme" id="theme" type="button">{theme_label}</button>
      </div>
    </div>
    <nav class="sections" aria-label="{'サイト' if locale == 'ja' else 'Site'}">{''.join(nav)}</nav>
  </div>
</header>
<main id="content">
  <div class="wrap">
{body}
  </div>
</main>
<footer class="site"><div class="wrap"><span>CFGB Example · example.invalid</span><span><a href="/{locale}/feed.xml">RSS</a> · <a href="https://github.com/ymmt2005/cfgb">GitHub</a></span></div></footer>
<script src="/assets/site.js"></script>
</body>
</html>
"""


def post_list(locale, posts):
    items = []
    for post in posts:
        items.append(
            "<li>"
            f'<p class="post-meta"><a href="/{locale}/archive/{post["year"]}/{post["month"]}/">{post["date"]}</a></p>'
            f'<h2><a href="{post["url"]}">{post["title"]}</a></h2>'
            f'<p class="summary">{post["summary"]}</p>'
            f'<p class="topics">{chips(locale, post["topics"])}</p>'
            "</li>"
        )
    return '<ol class="post-list">' + "".join(items) + "</ol>"


def article_page(post, prose, toc):
    locale = post["locale"]
    paired = post["alt"].rstrip("/").count("/") > 1
    if locale == "ja":
        link_label = "対訳を開く" if paired else "英語のホーム"
    else:
        link_label = "Open translation" if paired else "Japanese home"
    note = f'<p class="translation">{post["alt_note"]} <a href="{post["alt"]}">{link_label}</a></p>'
    mobile = f'<details class="toc-mobile"><summary>{"目次" if locale == "ja" else "Contents"}</summary>{toc}</details>'
    desktop = f'<nav class="toc toc-desktop" aria-label="{"目次" if locale == "ja" else "Contents"}"><p>{"目次" if locale == "ja" else "Contents"}</p>{toc}</nav>'
    body = f"""
<div class="article-layout">
  <article>
    <p class="kicker">{post["date"]}</p>
    <h1 class="article-title">{post["title"]}</h1>
    <div class="meta-row"><p class="topics">{chips(locale, post["topics"])}</p></div>
    {note}
    {mobile}
    <div class="prose">
{prose}
    </div>
  </article>
  {desktop}
</div>
"""
    return layout(locale, post["title"] + " · CFGB Example", body, "/ja/posts/" if locale == "ja" else "/en/posts/", post["alt"], post["summary"], post["url"])


def code_block(filename, rows):
    body = []
    for number, marked, html in rows:
        klass = "row mark" if marked else "row"
        body.append(f'<span class="{klass}"><span class="ln">{number}</span>{html}</span>')
    return f"""<figure class="code"><figcaption><span>{filename}</span><button class="copy" type="button">コピー</button></figcaption><pre><code>{''.join(body)}</code></pre></figure>"""


def write(rel, html):
    path = ROOT / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(html, encoding="utf-8")


def main():
    media = ROOT / "media"
    media.mkdir(exist_ok=True)
    for name, src in MEDIA.items():
        (media / name).write_bytes(src.read_bytes())

    ja = [p for p in POSTS if p["locale"] == "ja"]
    en = [p for p in POSTS if p["locale"] == "en"]
    ja_latest = ja[:5]
    en_latest = en[:5]

    write("index.html", """<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CFGB Example</title>
<link rel="stylesheet" href="/assets/site.css">
</head>
<body>
<main><div class="wrap negotiate">
<p class="kicker">/</p>
<h1 class="home-title">言語を選びます</h1>
<p class="home-intro">公開時、この URL は Worker がクッキーと Accept-Language を見て各言語のホームへ一時的に移動します。モックでは移動先を自分で選びます。</p>
<div class="choices"><a href="/ja/">日本語</a><a href="/en/">English</a></div>
</div></main>
</body>
</html>
""")

    ja_home = f"""
<p class="kicker">Latest</p>
<h1 class="home-title">CFGB Example</h1>
<div class="home-intro">
<p>ようこそ、CFGB のサンプルブログへ。</p>
<p>このサイトの文章は、日英の記事と静的な公開フローを検証するための架空のサンプルです。</p>
<p><a href="/ja/about/">About</a> · <a href="https://github.com/ymmt2005/cfgb">GitHub</a> · <a href="/ja/feed.xml">RSS</a></p>
</div>
{post_list("ja", ja_latest)}
"""
    write("ja/index.html", layout("ja", "CFGB Example", ja_home, "/ja/", "/en/", "CFGB のサンプルブログ"))

    en_home = f"""
<p class="kicker">Latest</p>
<h1 class="home-title">CFGB Example</h1>
<div class="home-intro">
<p>Welcome to the CFGB example blog.</p>
<p>These fictional articles exercise multilingual publishing and a static delivery workflow.</p>
<p><a href="/en/about/">About</a> · <a href="https://github.com/ymmt2005/cfgb">GitHub</a> · <a href="/en/feed.xml">RSS</a></p>
</div>
{post_list("en", en_latest)}
"""
    write("en/index.html", layout("en", "CFGB Example", en_home, "/en/", "/ja/", "CFGB example blog"))

    write("ja/posts/index.html", layout("ja", "記事 · CFGB Example", f'<h1 class="home-title">記事</h1>{post_list("ja", ja)}', "/ja/posts/", "/en/posts/", "日本語の記事"))
    write("en/posts/index.html", layout("en", "Posts · CFGB Example", f'<h1 class="home-title">Posts</h1>{post_list("en", en)}', "/en/posts/", "/ja/posts/", "English posts"))

    sep = [p for p in ja if p["year"] == "2026" and p["month"] == "09"]
    oct_ = [p for p in ja if p["year"] == "2026" and p["month"] == "10"]
    write("ja/archive/2026/09/index.html", layout("ja", "2026年9月 · CFGB Example", f'<p class="kicker">Archive</p><h1 class="home-title">2026年9月</h1><p class="summary">サイトのタイムゾーン Asia/Tokyo で九月に公開された日本語記事です。</p>{post_list("ja", sep)}', "/ja/archive/2026/09/", "/en/archive/2026/09/", "2026年9月"))
    write("ja/archive/2026/10/index.html", layout("ja", "2026年10月 · CFGB Example", f'<p class="kicker">Archive</p><h1 class="home-title">2026年10月</h1><p class="summary">UTC では九月末の記事が、日本時間では十月のアーカイブに入ります。</p>{post_list("ja", oct_)}', "/ja/archive/2026/10/", "/en/", "2026年10月"))
    write("en/archive/2026/09/index.html", layout("en", "September 2026 · CFGB Example", f'<p class="kicker">Archive</p><h1 class="home-title">September 2026</h1>{post_list("en", [p for p in en if p["month"]=="09"])}', "/en/archive/2026/09/", "/ja/archive/2026/09/", "September 2026"))

    proto = [p for p in ja if "protobuf" in p["topics"]]
    write("ja/topics/protobuf/index.html", layout("ja", "Protocol Buffers · CFGB Example", f'<p class="kicker">Topic</p><h1 class="home-title">Protocol Buffers</h1><p class="summary">トピック ID は言語間で共有し、表示名だけを翻訳します。</p>{post_list("ja", proto)}', "/ja/topics/protobuf/", "/en/topics/protobuf/", "Protocol Buffers"))
    write("en/topics/protobuf/index.html", layout("en", "Protocol Buffers · CFGB Example", f'<p class="kicker">Topic</p><h1 class="home-title">Protocol Buffers</h1>{post_list("en", [p for p in en if "protobuf" in p["topics"]])}', "/en/topics/protobuf/", "/ja/topics/protobuf/", "Protocol Buffers"))

    write("ja/about/index.html", layout("ja", "About · CFGB Example", '<article class="page-narrow prose"><h1>このサンプルについて</h1><p>CFGB の開発に使う架空の技術ブログです。実在する著者の経歴や実績を表すものではありません。</p></article>', "/ja/about/", "/en/about/", "About"))
    write("en/about/index.html", layout("en", "About · CFGB Example", '<article class="page-narrow prose"><h1>About this example</h1><p>This is a fictional technical blog used to develop CFGB. It does not describe the repository owner’s biography or achievements.</p></article>', "/en/about/", "/ja/about/", "About"))

    search_ja = """
<div class="page-narrow">
<p class="kicker">Search</p>
<h1 class="home-title">検索</h1>
<p class="mock-banner">表示中の結果はラベル付きのモックデータです。Pagefind による実検索は後のマイルストーンで接続します。</p>
<form data-search-mock>
  <div class="search-box"><input name="q" value="スキーマ" aria-label="検索語"></div>
  <div class="filters">
    <select name="topic" aria-label="トピック"><option value="">すべてのトピック</option><option value="protobuf">Protocol Buffers</option><option value="security">セキュリティ</option><option value="writing">文章を書く</option></select>
    <select name="year" aria-label="年"><option value="">すべての年</option><option value="2026">2026</option><option value="2025">2025</option></select>
  </div>
</form>
<div class="result" data-result data-topic="protobuf oss" data-year="2026" data-text="スキーマ Protocol Buffers フィールド番号">
  <p class="post-meta">2026年9月19日</p>
  <h2><a href="/ja/posts/protobuf-schema-guide/">Protocol Buffers のスキーマを読む</a></h2>
  <p class="summary">メッセージ、フィールド番号、型の参照を小さな例で確認します。</p>
</div>
<div class="result" data-result data-topic="software-engineering" data-year="2026" data-text="脚注 注意書き Mermaid">
  <p class="post-meta">2026年9月20日</p>
  <h2><a href="/ja/posts/markdown-showcase/">Markdown 表現の確認</a></h2>
  <p class="summary">脚注、注意書き、コードの行強調を一つの記事で確認します。</p>
</div>
<div class="result" data-result data-topic="writing" data-year="2025" data-text="短い文章 要約">
  <p class="post-meta">2025年12月15日</p>
  <h2><a href="/ja/posts/what-to-keep-in-a-short-post/">短く書くために残すもの</a></h2>
  <p class="summary">読者に持ち帰ってほしいことを一つに絞る架空の随筆です。</p>
</div>
</div>
"""
    write("ja/search/index.html", layout("ja", "検索 · CFGB Example", search_ja, "/ja/search/", "/en/search/", "検索"))

    search_en = """
<div class="page-narrow">
<p class="kicker">Search</p>
<h1 class="home-title">Search</h1>
<p class="mock-banner">These results are labeled mock data. Pagefind is connected in a later milestone.</p>
<form data-search-mock>
  <div class="search-box"><input name="q" value="schema" aria-label="Query"></div>
  <div class="filters">
    <select name="topic" aria-label="Topic"><option value="">All topics</option><option value="protobuf">Protocol Buffers</option><option value="software-engineering">Software Engineering</option></select>
    <select name="year" aria-label="Year"><option value="">All years</option><option value="2026">2026</option></select>
  </div>
</form>
<div class="result" data-result data-topic="protobuf oss" data-year="2026" data-text="schema Protocol Buffers field">
  <p class="post-meta">20 Sep 2026</p>
  <h2><a href="/en/posts/reading-protobuf-schemas/">Reading a Protocol Buffers schema</a></h2>
  <p class="summary">Field numbers and type references in a small message.</p>
</div>
<div class="result" data-result data-topic="software-engineering" data-year="2026" data-text="retries event">
  <p class="post-meta">23 Sep 2026</p>
  <h2><a href="/en/posts/retries-and-events/">Retries and event processing</a></h2>
  <p class="summary">Delivery attempts are not the same thing as application success.</p>
</div>
</div>
"""
    write("en/search/index.html", layout("en", "Search · CFGB Example", search_en, "/en/search/", "/ja/search/", "Search"))

    go = code_block("main.go", [
        (1, False, '<span class="kw">package</span> main'),
        (2, False, '<span class="kw">import</span> <span class="str">"fmt"</span>'),
        (3, False, ""),
        (4, True, '<span class="kw">func</span> <span class="fn">main</span>() {'),
        (5, True, '    fmt.<mark class="token">Println</mark>(<span class="str">"hello"</span>)'),
        (6, True, "}"),
    ])
    proto_code = code_block("user.proto", [
        (1, False, '<span class="kw">syntax</span> = <span class="str">"proto3"</span>;'),
        (2, False, '<span class="kw">message</span> User {'),
        (3, True, '  <span class="kw">string</span> display_name = <span class="fn">1</span>;'),
        (4, False, "}"),
    ])
    diagram = """<svg class="diagram" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 200" role="img" aria-label="Draft から Review を経て Publish へ進み、Review から Draft へ戻れる図">
<rect x="24" y="64" width="150" height="64" rx="8" fill="currentColor" opacity="0.08" stroke="currentColor"/>
<text x="99" y="102" text-anchor="middle" fill="currentColor" font-size="18">Draft</text>
<rect x="244" y="64" width="150" height="64" rx="8" fill="currentColor" opacity="0.08" stroke="currentColor"/>
<text x="319" y="102" text-anchor="middle" fill="currentColor" font-size="18">Review</text>
<rect x="464" y="64" width="150" height="64" rx="8" fill="currentColor" opacity="0.08" stroke="currentColor"/>
<text x="539" y="102" text-anchor="middle" fill="currentColor" font-size="18">Publish</text>
<path d="M174 96H244M394 96H464" stroke="currentColor" fill="none"/>
<path d="M319 128 V168 H99 V128" stroke="currentColor" fill="none"/>
</svg>"""
    showcase = next(p for p in ja if p["url"].endswith("markdown-showcase/"))
    toc = """<ol>
<li><a href="#コード">コード</a></li>
<li class="sub"><a href="#長い行">長い行</a></li>
<li><a href="#図と注意書き">図と注意書き</a></li>
<li><a href="#補足">補足</a></li>
<li><a href="#補足-2">補足</a></li>
<li><a href="#架空の公開パイプラインをレビューする">架空の公開パイプラインをレビューする</a></li>
<li class="sub"><a href="#正しいデータを一つにする">正しいデータを一つにする</a></li>
<li class="sub"><a href="#人間の編集を守る">人間の編集を守る</a></li>
<li class="sub"><a href="#プレビューの役割">プレビューの役割</a></li>
<li><a href="#このレビューで残ったこと">このレビューで残ったこと</a></li>
</ol>"""
    prose = f"""
<h2 id="コード">コード</h2>
<p>構文強調と行強調、コピー操作を確認します。4–6 行目が強調され、<mark class="token">Println</mark> が文中の印です。</p>
{go}
<h3 id="長い行">長い行</h3>
<p>横スクロール時もページ全体の幅を壊さないことを確認します。</p>
{code_block("snippet.txt", [(1, False, "0123456789 " * 12)])}
<h2 id="図と注意書き">図と注意書き</h2>
<p>Mermaid の図、注意書き、表、脚注を確認します。図の文字は本文と同じ色を使い、暗いテーマでも背景に埋もれません。</p>
<div class="alert alert-note"><p class="alert-label">Note</p><p>This is a note. 日本語の本文のなかに English の注意書きを置いても、行間と文字色は同じ組版のままです。</p></div>
<div class="alert alert-tip"><p class="alert-label">Tip</p><p>Copy the code with the keyboard too.</p></div>
<div class="alert alert-important"><p class="alert-label">Important</p><p>Keep the original source in Git.</p></div>
<div class="alert alert-warning"><p class="alert-label">Warning</p><p>A preview is not automatically private.</p></div>
<div class="alert alert-caution"><p class="alert-label">Caution</p><p>Review external embeds before publishing.</p></div>
<div class="table-wrap"><table><thead><tr><th>Feature</th><th>State</th></tr></thead><tbody><tr><td>Table</td><td>Ready</td></tr><tr><td><s>Old label</s></td><td>Replaced</td></tr></tbody></table></div>
<ul><li><label><input type="checkbox" checked disabled> Write Markdown</label></li><li><label><input type="checkbox" disabled> Review the preview</label></li></ul>
<details><summary>Raw HTML disclosure</summary><p id="html-anchor">A stable explicit HTML anchor.</p></details>
<figure><img src="/media/bars.png" alt="Lossless raster fixture with three colored bars" width="640" height="180"><figcaption>元画像はそのまま掲載します。</figcaption></figure>
{diagram}
<pre class="mermaid-source"><code>flowchart TD
  Draft --&gt; Review
  Review --&gt; Publish
  Review --&gt; Draft</code></pre>
<p>An inline <a href="https://www.iana.org/domains/reserved">ordinary link</a> stays a link. This standalone URL has no cache and must fall back to a hyperlink:</p>
<p><a href="https://www.iana.org/domains/reserved">https://www.iana.org/domains/reserved</a></p>
<p>Footnotes are supported.<a href="#fn-one" id="fnref-one">[1]</a></p>
<h2 id="補足">補足</h2>
<p><a href="/ja/posts/protobuf-schema-guide/#フィールド番号">Protocol Buffers の記事</a>へ戻ります。</p>
<h2 id="補足-2">補足</h2>
<p>同じ見出しでも固有のアンカーが必要です。</p>
<h2 id="架空の公開パイプラインをレビューする">架空の公開パイプラインをレビューする</h2>
<p>ここからは長い技術記事の要約と目次を確認するための架空の設計レビューです。対象は、Markdown を Git に保存し、レビューされた内容だけを静的なサイトへ公開する小さなシステムです。実在するサービスの導入事例ではありません。</p>
<h3 id="正しいデータを一つにする">正しいデータを一つにする</h3>
<p>記事の本文と要約は同じ Git の変更として扱います。本文はリポジトリにあるのに要約だけが別のサービスに保存されていると、過去の状態を再現するときに二つの履歴を照合しなければなりません。公開に使ったコミットを指定すれば、本文、要約、画像をまとめて確認できる構成を考えます。</p>
<h3 id="人間の編集を守る">人間の編集を守る</h3>
<p>自動生成された要約を人間が直したら、その変更には意味があると考えます。本文が変わったという理由だけで再生成すると、表現を整えた作業を失います。生成時の出力と現在の要約が同じときだけ、自動更新の候補にします。</p>
<h3 id="プレビューの役割">プレビューの役割</h3>
<p>差分の確認と表示の確認は役割が異なります。パソコンで読めても、幅の狭い画面ではコードブロックがページ全体を押し広げることがあります。暗いテーマで図の文字が背景に埋もれることもあります。本文が同じでも表示条件が違えば確認すべき点は変わります。</p>
<h2 id="このレビューで残ったこと">このレビューで残ったこと</h2>
<p>この架空のレビューでは、機能の数よりも境界を明確にすることを優先しました。生成する処理と配信する処理、人間の編集と機械が管理する出力、原稿の公開範囲とプレビューの公開範囲をそれぞれ分けています。</p>
<aside class="footnotes"><ol><li id="fn-one">A local note, not an external request. <a href="#fnref-one">↩</a></li></ol></aside>
"""
    write("ja/posts/markdown-showcase/index.html", article_page(showcase, prose, toc))

    en_show = next(p for p in en if "markdown-rendering" in p["url"])
    en_toc = "<ol><li><a href=\"#code\">Code</a></li><li><a href=\"#diagram\">Diagram</a></li></ol>"
    en_prose = f"""
<h2 id="code">Code</h2>
<p>The same filename, line marks, and copy control as the Japanese article. Mixed prose: 行強調は <mark class="token">Println</mark> の下線で示します。</p>
{go}
<h2 id="diagram">Diagram</h2>
{diagram}
<pre class="mermaid-source"><code>flowchart TD
  Draft --&gt; Review --&gt; Publish</code></pre>
<p>Switching language returns to <a href="/ja/posts/markdown-showcase/">the Japanese article</a>, not to a translated slug of this URL.</p>
"""
    write("en/posts/markdown-rendering-showcase/index.html", article_page(en_show, en_prose, en_toc))

    pb = next(p for p in ja if "protobuf-schema" in p["url"])
    pb_toc = "<ol><li><a href=\"#スキーマの入口\">スキーマの入口</a></li><li class=\"sub\"><a href=\"#フィールド番号\">フィールド番号</a></li><li><a href=\"#関連する資料\">関連する資料</a></li></ol>"
    pb_prose = f"""
<h2 id="スキーマの入口">スキーマの入口</h2>
<p>Protocol Buffers のスキーマは、メッセージとフィールドの関係を定義します。型の参照をたどると、API が返すデータの形を確認できます。日本語の地の文に Protocol Buffers や API のような英語の固有名詞が混ざっても、明朝とシステムフォントの役割は分けたままです。</p>
<figure><img src="/media/schema.svg" alt="メッセージとフィールドの関係" width="640" height="240"></figure>
<h3 id="フィールド番号">フィールド番号</h3>
<p>フィールド番号は互換性を考える手がかりです。このサンプルでは名前と番号を分けて表示します。</p>
{proto_code}
<h2 id="関連する資料">関連する資料</h2>
<p>次の単独 URL はキャッシュ済みのリンクカードになります。</p>
<a class="card" href="https://github.com/ymmt2005/pbschema-lens"><span class="card-site">GitHub</span><span class="card-title">pbschema-lens (fixture metadata)</span><span class="card-desc">Synthetic cached metadata used to test link-card rendering. This is not a fetched description.</span></a>
<p><a href="/ja/posts/markdown-showcase/#コード">Markdown 表現のサンプル</a>も参照してください。</p>
"""
    write("ja/posts/protobuf-schema-guide/index.html", article_page(pb, pb_prose, pb_toc))

    en_pb = next(p for p in en if "reading-protobuf" in p["url"])
    write("en/posts/reading-protobuf-schemas/index.html", article_page(en_pb, f"""
<h2 id="entry">A small message</h2>
<p>Field numbers are the compatibility clue. The Japanese article in this group uses a different slug and an earlier publication instant.</p>
{proto_code}
<a class="card" href="https://github.com/ymmt2005/pbschema-lens"><span class="card-site">GitHub</span><span class="card-title">pbschema-lens (fixture metadata)</span><span class="card-desc">Synthetic cached metadata used to test link-card rendering. This is not a fetched description.</span></a>
""", "<ol><li><a href=\"#entry\">A small message</a></li></ol>"))

    quantum = next(p for p in ja if "post-quantum" in p["url"])
    write("ja/posts/post-quantum-notes/index.html", article_page(quantum, """
<h2 id="区別">区別して読む</h2>
<p>鍵共有とデータ暗号化は、どちらも「量子コンピュータに備える」という話のなかで並びますが、確認する性質は別です。ML-KEM は鍵を共有する方式の名前で、ハイブリッド方式は古い方式と新しい方式を同時に使う構成です。</p>
<p>この記事には英語版がありません。ヘッダーの English は、存在しない対訳を作らず、英語のホームを開きます。</p>
""", "<ol><li><a href=\"#区別\">区別して読む</a></li></ol>"))

    # Short pages so list links resolve.
    for post in POSTS:
        rel = post["url"].strip("/") + "/index.html"
        if (ROOT / rel).exists():
            continue
        write(rel, article_page(post, f'<p>{post["summary"]}</p>', "<ol></ol>"))

    # Placeholder feeds so footer links are not dead.
    for locale in ("ja", "en"):
        write(f"{locale}/feed.xml", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<rss version=\"2.0\"><channel><title>CFGB Example</title><description>Mock feed. The renderer emits the real feed later.</description></channel></rss>\n")


if __name__ == "__main__":
    main()
