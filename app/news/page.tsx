"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  API_BASE_URL,
  fallbackArticles,
  formatArticleDate,
} from "../website-data";
import { WebsiteFooter, WebsiteHeader } from "../WebsiteChrome";

export default function NewsPage() {
  const [articles, setArticles] = useState(fallbackArticles);

  useEffect(() => {
    fetch(`${API_BASE_URL}/public/website/articles?page=1&page_size=50`)
      .then((response) => response.ok ? response.json() : Promise.reject())
      .then((payload) => {
        const data = payload?.data;
        if (Array.isArray(data) && data.length) setArticles(data);
      })
      .catch(() => undefined);
  }, []);

  return (
    <main className="warm-site warm-inner">
      <WebsiteHeader />
      <section className="warm-inner__hero">
        <p>NEWS & STORIES</p>
        <h1>摊伴动态</h1>
        <span>记录产品迭代、经营观察与我们走过的每一步。</span>
      </section>
      <section className="warm-news-list">
        {articles.map((article, index) => (
          <Link href={`/news/${article.slug}`} key={article.id}>
            <span className="warm-news-list__no">{String(index + 1).padStart(2, "0")}</span>
            <div><small>{formatArticleDate(article.publishedAt)}</small><h2>{article.title}</h2><p>{article.summary}</p></div>
            <b>→</b>
          </Link>
        ))}
      </section>
      <WebsiteFooter />
    </main>
  );
}
