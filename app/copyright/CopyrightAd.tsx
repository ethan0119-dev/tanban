"use client";

import { useEffect, useState } from "react";

interface Copy {
  brand: string;
  title: string;
  subtitle: string;
  wechat: string;
  qr: string;
}

const fallback: Copy = {
  brand: "摊伴餐饮系统",
  title: "让每一家小店，都能轻松拥有自己的数字化点餐系统",
  subtitle: "点餐、营销、会员与门店经营，一套系统顺畅连接。",
  wechat: "",
  qr: "",
};

export function CopyrightAd() {
  const [copy, setCopy] = useState(fallback);

  useEffect(() => {
    const query = new URLSearchParams(window.location.search);
    setCopy({
      brand: query.get("brand")?.slice(0, 80) || fallback.brand,
      title: query.get("title")?.slice(0, 120) || fallback.title,
      subtitle: query.get("subtitle")?.slice(0, 300) || fallback.subtitle,
      wechat: query.get("wechat")?.slice(0, 80) || "",
      qr: query.get("qr") || "",
    });
  }, []);

  return (
    <main className="copyright-ad">
      <section className="copyright-ad__hero">
        <p>TANBAN · SMART ORDERING</p>
        <div className="copyright-ad__brand"><span>伴</span>{copy.brand}</div>
        <h1>{copy.title}</h1>
        <p className="copyright-ad__lead">{copy.subtitle}</p>
        <div className="copyright-ad__pills"><span>微信点餐</span><span>营销增长</span><span>门店经营</span></div>
      </section>

      <section className="copyright-ad__section">
        <p className="copyright-ad__kicker">ONE SYSTEM · FULL JOURNEY</p>
        <h2>一套系统，连接经营全流程</h2>
        <div className="copyright-ad__grid">
          <article><i>01</i><h3>微信点餐</h3><p>堂食、自取、支付和订单进度顺畅连接，让顾客少等待。</p></article>
          <article><i>02</i><h3>营销增长</h3><p>优惠券、弹窗活动、抽奖和推荐商品，帮助好产品被看见。</p></article>
          <article><i>03</i><h3>门店管理</h3><p>商品、库存、打印和经营数据统一管理，日常操作更简单。</p></article>
          <article><i>04</i><h3>品牌装修</h3><p>配色、首页组件和内容自由组合，保留每家小店自己的气质。</p></article>
        </div>
      </section>

      <section className="copyright-ad__quote">
        <p>“为认真经营的小店，做一套好用、耐用、看得懂的系统。”</p>
        <span>摊伴把复杂的技术留在后台，把简单顺畅的体验交给商家和顾客。</span>
      </section>

      <section className="copyright-ad__contact">
        <p className="copyright-ad__kicker">LET&apos;S TALK</p>
        <h2>想了解更多？扫码联系摊伴</h2>
        {/* QR images are administrator-provided URLs and intentionally bypass build-time image optimization. */}
        {/* eslint-disable-next-line @next/next/no-img-element */}
        {copy.qr ? <img src={copy.qr} alt="摊伴微信联系二维码" /> : <div className="copyright-ad__qr-empty">联系二维码<br />由平台后台配置</div>}
        {copy.wechat && <p>微信号：{copy.wechat}</p>}
        <small>长按二维码识别，或截图后在微信中扫码</small>
      </section>

      <footer>© {new Date().getFullYear()} {copy.brand} · 为小店经营提供数字化支持</footer>
    </main>
  );
}
