"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import {
  API_BASE_URL,
  fallbackArticles,
  formatArticleDate,
  type WebsiteArticle,
} from "../../website-data";

export default function ArticlePage() {
  const params = useParams<{ slug: string }>();
  const slug = params?.slug || "";
  const fallback = fallbackArticles.find((item) => item.slug === slug) || fallbackArticles[0];
  const [article, setArticle] = useState<WebsiteArticle>(fallback);

  useEffect(() => {
    if (!slug) return;
    fetch(`${API_BASE_URL}/public/website/articles/${encodeURIComponent(slug)}`)
      .then((response) => response.ok ? response.json() : Promise.reject())
      .then((payload) => payload?.data && setArticle(payload.data))
      .catch(() => undefined);
  }, [slug]);

  return (
    <main className="official-site official-inner">
      <header className="official-header">
        <Link className="official-brand" href="/"><span className="official-brand__mark">伴</span><span><strong>摊伴</strong><small>TANBAN</small></span></Link>
        <nav className="official-nav"><Link href="/#product">产品能力</Link><Link href="/#scenes">适用场景</Link><Link href="/news">品牌动态</Link><Link href="/#about">关于摊伴</Link></nav>
        <div className="official-header__actions"><Link className="official-button official-button--small" href="/#contact">预约了解</Link></div>
      </header>
      <article className="official-article">
        <Link href="/news" className="official-article__back">← 返回品牌动态</Link>
        <small>{formatArticleDate(article.publishedAt)} · 品牌动态</small>
        <h1>{article.title}</h1>
        <p className="official-article__summary">{article.summary}</p>
        {article.coverUrl && <img src={article.coverUrl} alt="" />}
        <div className="official-article__body">
          {article.content.split(/\n{2,}/).map((paragraph) => <p key={paragraph}>{paragraph}</p>)}
        </div>
      </article>
      <footer className="official-footer official-footer--inner"><p>让小生意，也有从容经营的底气。</p><div><span>© 2026 TANBAN</span></div></footer>
    </main>
  );
}
