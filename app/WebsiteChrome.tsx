"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import tanbanIcon from "../apps/merchant-web/src/assets/brand/tanban-icon.png";
import { fallbackSettings, type WebsiteSettings } from "./website-data";

type WebsiteHeaderProps = {
  home?: boolean;
  merchantLoginUrl?: string;
};

type WebsiteFooterProps = {
  settings?: Pick<WebsiteSettings, "companyName" | "icpNumber" | "footerText">;
};

const tanbanIconSrc = typeof tanbanIcon === "string" ? tanbanIcon : tanbanIcon.src;

export function WebsiteBrand() {
  return (
    <span className="warm-brand">
      <span className="warm-brand__mark"><img src={tanbanIconSrc} alt="" /></span>
      <span className="warm-brand__word"><strong>摊伴</strong><small>TANBAN</small></span>
    </span>
  );
}

export function WebsiteHeader({ home = false, merchantLoginUrl = fallbackSettings.merchantLoginUrl }: WebsiteHeaderProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [aboutOpen, setAboutOpen] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);
  const closeMenu = () => setMenuOpen(false);
  const href = (anchor: string) => home ? anchor : `/${anchor}`;

  useEffect(() => {
    if (!aboutOpen) return;

    const previousActiveElement = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setAboutOpen(false);
    };

    document.body.style.overflow = "hidden";
    document.addEventListener("keydown", handleKeyDown);
    dialogRef.current?.focus();

    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", handleKeyDown);
      previousActiveElement?.focus();
    };
  }, [aboutOpen]);

  const openAbout = () => {
    closeMenu();
    setAboutOpen(true);
  };

  return (
    <>
      <header className="warm-header">
        <Link href={home ? "#top" : "/"} aria-label="摊伴首页" onClick={closeMenu}><WebsiteBrand /></Link>
        <button
          className="warm-menu-toggle"
          type="button"
          aria-label="切换导航"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((value) => !value)}
        >
          <span /><span />
        </button>
        <nav className={menuOpen ? "warm-nav is-open" : "warm-nav"}>
          <Link href={href("#product")} onClick={closeMenu}>产品能力</Link>
          <Link href={href("#scenes")} onClick={closeMenu}>适用门店</Link>
          <Link href="/news" onClick={closeMenu}>产品动态</Link>
          <button className="warm-nav__about" type="button" aria-haspopup="dialog" onClick={openAbout}>关于摊伴</button>
        </nav>
        <div className="warm-header__actions">
          <a href={merchantLoginUrl} target="_blank" rel="noreferrer">登录商户后台</a>
          <Link className="warm-button warm-button--small" href={href("#contact")}>免费体验</Link>
        </div>
      </header>

      {aboutOpen && (
        <div className="warm-about-modal" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setAboutOpen(false);
        }}>
          <div
            className="warm-about-modal__dialog"
            ref={dialogRef}
            role="dialog"
            aria-modal="true"
            aria-labelledby="warm-about-title"
            tabIndex={-1}
          >
            <button className="warm-about-modal__close" type="button" aria-label="关闭关于摊伴弹窗" onClick={() => setAboutOpen(false)}>×</button>
            <span className="warm-about-modal__eyebrow">ABOUT TANBAN</span>
            <h2 id="warm-about-title">关于摊伴</h2>
            <div className="warm-about-modal__company">
              <strong>北京一百六十度科技有限公司</strong>
              <span>年轻的软件公司 · 多年一线行业经验</span>
            </div>
            <p>北京一百六十度科技有限公司是一家年轻的软件公司。我们在小微经营数字化领域持续摸索多年，将一线实践中积累的经验，沉淀成真正贴近经营现场、简单好用的产品。</p>
            <p>摊伴正是这段经验的成果。它综合了小微商户、流动摊位和夜市经营者在点单、收银、出餐、会员与日常管理中的真实需求与痛点，打造一套轻量、灵活、易上手的经营系统，让认真经营的每一门小生意都能更从容。</p>
            <div className="warm-about-modal__signature"><i /><span>让小生意，也有从容经营的底气。</span></div>
          </div>
        </div>
      )}
    </>
  );
}

export function WebsiteFooter({ settings = fallbackSettings }: WebsiteFooterProps) {
  return (
    <footer className="warm-footer warm-footer--shared">
      <div className="warm-footer__brand"><WebsiteBrand /><span>{settings.footerText}</span></div>
      <p>© 2026 {settings.companyName || "北京一百六十度科技有限公司"}. 保留所有权利。</p>
      <div className="warm-footer__links">
        <Link href="/#about">隐私政策</Link>
        <Link href="/#about">服务协议</Link>
        {settings.icpNumber && <a href="https://beian.miit.gov.cn/" target="_blank" rel="noreferrer">{settings.icpNumber}</a>}
      </div>
    </footer>
  );
}
