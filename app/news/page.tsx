"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  API_BASE_URL,
  fallbackArticles,
  formatArticleDate,
} from "../website-data";

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
    <main className="official-site official-inner">
      <header className="official-header">
        <Link className="official-brand" href="/"><span className="official-brand__mark">伴</span><span><strong>摊伴</strong><small>TANBAN</small></span></Link>
        <nav className="official-nav"><Link href="/#product">产品能力</Link><Link href="/#scenes">适用场景</Link><Link href="/news">品牌动态</Link><Link href="/#about">关于摊伴</Link></nav>
        <div className="official-header__actions"><Link className="official-button official-button--small" href="/#contact">预约了解</Link></div>
      </header>
      <section className="official-inner__hero">
        <p>NEWS & STORIES</p>
        <h1>品牌动态</h1>
        <span>记录产品迭代、经营观察与我们走过的每一步。</span>
      </section>
      <section className="official-news-list">
        {articles.map((article, index) => (
          <Link href={`/news/${article.slug}`} key={article.id}>
            <span className="official-news-list__no">{String(index + 1).padStart(2, "0")}</span>
            <div><small>{formatArticleDate(article.publishedAt)}</small><h2>{article.title}</h2><p>{article.summary}</p></div>
            <b>→</b>
          </Link>
        ))}
      </section>
      <footer className="official-footer official-footer--inner"><p>让小生意，也有从容经营的底气。</p><div><span>© 2026 TANBAN</span></div></footer>
    </main>
  );
}
