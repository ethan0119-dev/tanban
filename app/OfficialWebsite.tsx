"use client";

import { Bread } from "@phosphor-icons/react/dist/ssr/Bread";
import { CaretRight } from "@phosphor-icons/react/dist/ssr/CaretRight";
import { ChatCircleDots } from "@phosphor-icons/react/dist/ssr/ChatCircleDots";
import { Coffee } from "@phosphor-icons/react/dist/ssr/Coffee";
import { CreditCard } from "@phosphor-icons/react/dist/ssr/CreditCard";
import { ForkKnife } from "@phosphor-icons/react/dist/ssr/ForkKnife";
import { Headset } from "@phosphor-icons/react/dist/ssr/Headset";
import { Monitor } from "@phosphor-icons/react/dist/ssr/Monitor";
import { Phone } from "@phosphor-icons/react/dist/ssr/Phone";
import { Printer } from "@phosphor-icons/react/dist/ssr/Printer";
import { QrCode } from "@phosphor-icons/react/dist/ssr/QrCode";
import { Receipt } from "@phosphor-icons/react/dist/ssr/Receipt";
import { Storefront } from "@phosphor-icons/react/dist/ssr/Storefront";
import { UsersThree } from "@phosphor-icons/react/dist/ssr/UsersThree";
import Link from "next/link";
import { useEffect, useState, type MouseEvent } from "react";
import {
  API_BASE_URL,
  fallbackArticles,
  fallbackSettings,
  formatArticleDate,
  type WebsiteArticle,
  type WebsiteSettings,
} from "./website-data";
import { OPEN_EXPERIENCE_EVENT, WebsiteFooter, WebsiteHeader } from "./WebsiteChrome";

type WebsitePayload = {
  settings?: Partial<WebsiteSettings>;
  articles?: WebsiteArticle[];
};

const flowSteps = [
  {
    number: "01",
    title: "顾客扫码点单",
    text: "微信扫码进入小程序，自助点单，支持堂食、自取、外卖多种场景。",
    icon: QrCode,
    imageKey: "scanOrderImageUrl" as const,
    imagePosition: "right",
  },
  {
    number: "02",
    title: "店员平板收银",
    text: "平板收银高效开单，订单管理、桌台管理、会员与储值一目了然，经营更省心。",
    icon: CreditCard,
    imageKey: "cashierImageUrl" as const,
    imagePosition: "left",
  },
  {
    number: "03",
    title: "后厨打印与取餐",
    text: "自动打印小票，后厨高效出餐；取餐自动提醒，顾客不久等。",
    icon: Printer,
    imageKey: "kitchenImageUrl" as const,
    imagePosition: "right",
  },
];

const sceneData = [
  { label: "早餐摊", icon: Storefront, key: "sceneBreakfastImageUrl" as const },
  { label: "咖啡车", icon: Coffee, key: "sceneCoffeeTruckImageUrl" as const },
  { label: "小蛋糕店", icon: Bread, key: "sceneBakeryImageUrl" as const },
  { label: "夜市小吃", icon: ForkKnife, key: "sceneNightMarketImageUrl" as const },
  { label: "小餐馆", icon: Storefront, key: "sceneCafeImageUrl" as const },
];

export function OfficialWebsite() {
  const [settings, setSettings] = useState(fallbackSettings);
  const [articles, setArticles] = useState(fallbackArticles);
  const [contactNotice, setContactNotice] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    fetch(`${API_BASE_URL}/public/website`, { signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error("website content unavailable");
        return response.json();
      })
      .then((payload: { data?: WebsitePayload } | WebsitePayload) => {
        const data = "data" in payload && payload.data ? payload.data : payload;
        if (data.settings) setSettings({ ...fallbackSettings, ...data.settings });
        if (Array.isArray(data.articles) && data.articles.length) setArticles(data.articles);
      })
      .catch(() => undefined);
    return () => controller.abort();
  }, []);

  const heroTitle = settings.heroTitle.includes("，")
    ? settings.heroTitle.replace("，", "，\n")
    : settings.heroTitle;
  const supportPhone = settings.supportPhone.trim();
  const hasContactDetails = Boolean(supportPhone || settings.contactQrUrl.trim() || settings.contactWechat.trim());

  const showContactNotice = () => {
    setContactNotice(true);
    window.setTimeout(() => setContactNotice(false), 2400);
  };

  const handleContactNavigation = (event: MouseEvent<HTMLAnchorElement>) => {
    if (hasContactDetails) return;
    event.preventDefault();
    showContactNotice();
  };

  const handleExperience = (event: MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault();
    window.dispatchEvent(new Event(OPEN_EXPERIENCE_EVENT));
  };

  return (
    <main className="warm-site">
      <WebsiteHeader home />

      <section className="warm-hero" id="top">
        <div className="warm-hero__copy">
          <p className="warm-kicker">{settings.heroEyebrow}</p>
          <h1><span>{heroTitle}</span><em>{settings.heroHighlight}</em></h1>
          <p className="warm-hero__lead">扫码点单、平板收银、自动打印、会员储值，一套摊伴就够了。</p>
          <div className="warm-hero__actions">
            <a className="warm-button" href="#experience" role="button" aria-haspopup="dialog" onClick={handleExperience}>免费体验</a>
            <a className="warm-button warm-button--outline" href="#contact" onClick={handleContactNavigation}><ChatCircleDots size={20} weight="fill" />联系客服</a>
          </div>
          <div className="warm-hero__features">
            <span><QrCode size={17} />微信扫码点单</span>
            <span><Monitor size={17} />平板收银管理</span>
            <span><Printer size={17} />自动打印出餐</span>
            <span><UsersThree size={17} />会员储值营销</span>
          </div>
        </div>
        <div className="warm-hero__asset">
          <img src={settings.heroImageUrl || fallbackSettings.heroImageUrl} alt="摊伴平板收银与顾客点单界面" />
        </div>
      </section>

      <section className="warm-flow" id="product">
        <div className="warm-section-title">
          <h2>一套系统，跑通点单到收款</h2>
          <p>从顾客下单到后厨出餐，每一步都更顺畅</p>
        </div>
        <div className="warm-flow__list">
          {flowSteps.map((step) => {
            const StepIcon = step.icon;
            return (
              <article className={`warm-flow-card warm-flow-card--${step.imagePosition}`} key={step.number}>
                <div className="warm-flow-card__copy">
                  <div className="warm-flow-card__heading">
                    <span>{step.number}</span>
                    <StepIcon size={28} weight="duotone" />
                    <h3>{step.title}</h3>
                  </div>
                  <p>{step.text}</p>
                  <a href="#contact" aria-label={`了解${step.title}`}><CaretRight size={20} weight="bold" /></a>
                </div>
                <img src={settings[step.imageKey] || fallbackSettings[step.imageKey]} alt={step.title} />
              </article>
            );
          })}
        </div>
      </section>

      <section className="warm-scenes" id="scenes">
        <div className="warm-section-title">
          <h2>适用于各类小店经营场景</h2>
          <p>小店生意，用摊伴更简单</p>
        </div>
        <div className="warm-scene-grid">
          {sceneData.map((scene) => {
            const SceneIcon = scene.icon;
            return (
              <article key={scene.label}>
                <img src={settings[scene.key] || fallbackSettings[scene.key]} alt={scene.label} />
                <span><SceneIcon size={18} weight="fill" />{scene.label}</span>
              </article>
            );
          })}
        </div>
      </section>

      <section className="warm-news" id="news">
        <div className="warm-news__heading">
          <h2>摊伴动态</h2>
          <Link href="/news">查看更多 <CaretRight size={14} /></Link>
        </div>
        <div className="warm-news__grid">
          {articles.slice(0, 3).map((article) => (
            <Link href={`/news/${article.slug}`} key={article.id}>
              <small>{formatArticleDate(article.publishedAt)}</small>
              <h3>{article.title}</h3>
              <p>{article.summary}</p>
              <span>阅读全文 <CaretRight size={12} /></span>
            </Link>
          ))}
        </div>
      </section>

      <section className="warm-about" id="about">
        <span><Receipt size={27} weight="duotone" /></span>
        <div><small>ABOUT TANBAN</small><h2>小店不小，认真经营都值得被看见</h2></div>
        <p>摊伴专为咖啡摊、早餐摊、夜市餐饮和小型门店打造。我们把复杂的经营工具做得简单、顺手，让主理人更专注产品与顾客。</p>
      </section>

      <section className="warm-contact" id="contact">
        <div className="warm-contact__qr">
          {settings.contactQrUrl ? (
            <img src={settings.contactQrUrl} alt="摊伴客服微信二维码" />
          ) : (
            <span><QrCode size={52} weight="duotone" /></span>
          )}
          <div><strong>{settings.contactQrUrl ? "扫码添加客服微信" : "客服微信敬请期待"}</strong><small>获取产品介绍与开通指导</small></div>
        </div>
        <div className="warm-contact__phone">
          <Phone size={34} weight="fill" />
          <div>{supportPhone ? <a href={`tel:${supportPhone}`}>{supportPhone}</a> : <strong>敬请期待</strong>}<small>工作日 09:00 - 21:00</small></div>
        </div>
        {supportPhone ? (
          <a className="warm-button warm-contact__online" href={`tel:${supportPhone}`}><ChatCircleDots size={22} weight="fill" />在线咨询</a>
        ) : (
          <button className="warm-button warm-contact__online" type="button" onClick={showContactNotice}><ChatCircleDots size={22} weight="fill" />在线咨询</button>
        )}
      </section>

      <WebsiteFooter settings={settings} />

      <a className="warm-floating-service" href="#contact" aria-label="客服咨询" onClick={handleContactNavigation}>
        <Headset size={25} weight="duotone" /><span>客服<br />咨询</span>
      </a>
      {contactNotice && <div className="warm-contact-notice" role="status" aria-live="polite">客服功能敬请期待</div>}
    </main>
  );
}
