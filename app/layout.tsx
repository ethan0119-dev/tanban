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
  metadataBase: new URL("https://tanban.com.cn"),
  title: {
    default: "一百六十度科技｜摊伴系统",
    template: "%s｜一百六十度科技｜摊伴系统",
  },
  description: "扫码点单、平板收银、自动打印、会员储值，面向早餐摊、咖啡车、夜市餐饮与小型门店的一体化经营系统。",
  openGraph: {
    title: "一百六十度科技｜摊伴系统",
    description: "扫码点单、平板收银、自动打印、会员储值，一套摊伴就够了。",
    images: [{ url: "/website/hero-devices.png", width: 1536, height: 1024, alt: "摊伴平板收银与顾客点单系统" }],
  },
  twitter: {
    card: "summary_large_image",
    title: "一百六十度科技｜摊伴系统",
    description: "扫码点单、平板收银、自动打印、会员储值，一套摊伴就够了。",
    images: ["/website/hero-devices.png"],
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
