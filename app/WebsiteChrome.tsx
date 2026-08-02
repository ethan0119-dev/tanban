"use client";

import Link from "next/link";
import { useState } from "react";
import tanbanIcon from "../apps/merchant-web/src/assets/brand/tanban-icon.png";
import { fallbackSettings, type WebsiteSettings } from "./website-data";

type WebsiteHeaderProps = {
  home?: boolean;
  merchantLoginUrl?: string;
};

type WebsiteFooterProps = {
  settings?: Pick<WebsiteSettings, "brandName" | "brandEnglishName" | "icpNumber" | "footerText">;
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
  const closeMenu = () => setMenuOpen(false);
  const href = (anchor: string) => home ? anchor : `/${anchor}`;

  return (
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
        <Link href={href("#about")} onClick={closeMenu}>关于摊伴</Link>
      </nav>
      <div className="warm-header__actions">
        <a href={merchantLoginUrl} target="_blank" rel="noreferrer">登录商户后台</a>
        <Link className="warm-button warm-button--small" href={href("#contact")}>免费体验</Link>
      </div>
    </header>
  );
}

export function WebsiteFooter({ settings = fallbackSettings }: WebsiteFooterProps) {
  return (
    <footer className="warm-footer warm-footer--shared">
      <div className="warm-footer__brand"><WebsiteBrand /><span>{settings.footerText}</span></div>
      <p>© 2026 {settings.brandName} {settings.brandEnglishName}. 保留所有权利。</p>
      <div className="warm-footer__links"><Link href="/#about">隐私政策</Link><Link href="/#about">服务协议</Link>{settings.icpNumber && <span>{settings.icpNumber}</span>}</div>
    </footer>
  );
}
