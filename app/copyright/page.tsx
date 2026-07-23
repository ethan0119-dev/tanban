import type { Metadata } from "next";
import { CopyrightAd } from "./CopyrightAd";

export const metadata: Metadata = {
  title: "版权说明｜摊伴餐饮系统",
  description: "摊伴餐饮系统产品介绍与联系信息。",
};

export default function CopyrightPage() {
  return <CopyrightAd />;
}
