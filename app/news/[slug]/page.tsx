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
import { WebsiteFooter, WebsiteHeader } from "../../WebsiteChrome";

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
    <main className="warm-site warm-inner">
      <WebsiteHeader />
      <article className="warm-article">
        <Link href="/news" className="warm-article__back">← 返回摊伴动态</Link>
        <small>{formatArticleDate(article.publishedAt)} · 品牌动态</small>
        <h1>{article.title}</h1>
        <p className="warm-article__summary">{article.summary}</p>
        {article.coverUrl && <img src={article.coverUrl} alt="" />}
        <div className="warm-article__body">
          {article.content.split(/\n{2,}/).map((paragraph) => <p key={paragraph}>{paragraph}</p>)}
        </div>
      </article>
      <WebsiteFooter />
    </main>
  );
}
