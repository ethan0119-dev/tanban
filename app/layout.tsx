import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL("https://tanban-saas.liuxiaoyicn.chatgpt.site"),
  title: "摊伴 TANBAN｜移动餐饮 SaaS",
  description: "面向咖啡摊、夜市摊与小型门店的一体化点单经营系统。",
  openGraph: {
    title: "摊伴 TANBAN｜移动餐饮 SaaS",
    description: "让每一个小摊，经营成一个好品牌。平台、商户、顾客三端一体。",
    images: [{ url: "/og.png", width: 1731, height: 909, alt: "摊伴移动餐饮 SaaS 三端一体" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "摊伴 TANBAN｜移动餐饮 SaaS",
    description: "让每一个小摊，经营成一个好品牌。",
    images: ["/og.png"],
  },
  icons: {
    icon: "/favicon.svg",
    shortcut: "/favicon.svg",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>
        {children}
      </body>
    </html>
  );
}
