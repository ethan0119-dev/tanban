export type WebsiteSettings = {
  brandName: string;
  brandEnglishName: string;
  heroEyebrow: string;
  heroTitle: string;
  heroHighlight: string;
  heroSubtitle: string;
  heroImageUrl: string;
  scanOrderImageUrl: string;
  cashierImageUrl: string;
  kitchenImageUrl: string;
  sceneBreakfastImageUrl: string;
  sceneCoffeeTruckImageUrl: string;
  sceneBakeryImageUrl: string;
  sceneNightMarketImageUrl: string;
  sceneCafeImageUrl: string;
  supportPhone: string;
  supportEmail: string;
  contactWechat: string;
  contactQrUrl: string;
  companyName: string;
  companyAddress: string;
  icpNumber: string;
  footerText: string;
  merchantLoginUrl: string;
};

export type WebsiteArticle = {
  id: string;
  slug: string;
  title: string;
  summary: string;
  coverUrl: string;
  content: string;
  publishedAt: string;
  isFeatured: boolean;
};

export const publicSecurityRecord = {
  number: "京公网安备11011202102155号",
  code: "11011202102155",
  queryUrl: "https://beian.mps.gov.cn/#/query/webSearch?code=11011202102155",
  iconUrl: "/website/public-security-record.png",
} as const;

export const fallbackSettings: WebsiteSettings = {
  brandName: "摊伴",
  brandEnglishName: "TANBAN",
  heroEyebrow: "为小店而生的数字化经营工具",
  heroTitle: "小店，也值得拥有一套",
  heroHighlight: "好用的经营系统。",
  heroSubtitle:
    "从顾客扫码点单、会员营销，到门店接单与平台管理，摊伴把日常经营需要的能力放进一套简单、顺手的系统。",
  heroImageUrl: "/website/hero-devices.png",
  scanOrderImageUrl: "/website/scan-ordering.png",
  cashierImageUrl: "/website/cashier-counter.png",
  kitchenImageUrl: "/website/kitchen-printer.png",
  sceneBreakfastImageUrl: "/website/scene-breakfast.png",
  sceneCoffeeTruckImageUrl: "/website/scene-coffee-truck.png",
  sceneBakeryImageUrl: "/website/scene-bakery.png",
  sceneNightMarketImageUrl: "/website/scene-night-market.png",
  sceneCafeImageUrl: "/website/scene-cafe.png",
  supportPhone: "400-865-0906",
  supportEmail: "hello@tanban.cn",
  contactWechat: "TanbanService",
  contactQrUrl: "",
  companyName: "北京一百六十度科技有限公司",
  companyAddress: "中国 · 北京",
  icpNumber: "京ICP备2023013917号-2",
  footerText: "让小生意，也有从容经营的底气。",
  merchantLoginUrl: "https://b.tanban.com.cn/",
};

export const fallbackArticles: WebsiteArticle[] = [
  {
    id: "1",
    slug: "tanban-official-website",
    title: "摊伴官网与内容管理能力进入开发阶段",
    summary: "品牌官网、产品介绍、动态发布和客服信息将由平台后台统一维护。",
    coverUrl: "/website/hero-devices.png",
    content:
      "摊伴正在建设新的品牌官网。\n\n本次建设会把产品能力、适用场景、品牌动态和联系方式组织成更清晰的内容，同时在平台管理端增加官网管理入口。后续运营人员可以直接维护首页图片、动态文章、客服二维码和客服电话，不需要改动代码。",
    publishedAt: "2026-07-30 10:00:00",
    isFeatured: true,
  },
  {
    id: "2",
    slug: "three-surface-product",
    title: "平台、商户、顾客三端如何协同",
    summary: "一套数据链路连接点单、履约、会员与经营分析。",
    coverUrl: "/website/cashier-counter.png",
    content:
      "顾客端负责便捷点单和会员体验，商户端承接商品、订单、营销和门店经营，平台端统一管理商户、服务配置与运营内容。\n\n三端共享一致的业务数据，减少重复录入，让门店能够更专注地服务顾客。",
    publishedAt: "2026-07-26 09:30:00",
    isFeatured: false,
  },
  {
    id: "3",
    slug: "for-small-food-business",
    title: "为小型餐饮场景设计的数字化工具",
    summary: "覆盖咖啡摊、夜市摊、快餐与轻量门店，保留灵活经营方式。",
    coverUrl: "/website/scan-ordering.png",
    content:
      "不同于重型餐饮系统，摊伴更关注小型餐饮经营中的速度、灵活与低学习成本。\n\n产品会围绕出摊、收摊、快速接单、取餐通知、会员复购和经营复盘持续优化。",
    publishedAt: "2026-07-20 16:20:00",
    isFeatured: false,
  },
];

export const API_BASE_URL =
  process.env.NEXT_PUBLIC_TANBAN_API_URL?.replace(/\/$/, "") ||
  "https://api.tanban.com.cn/api/v1";

export function formatArticleDate(value: string) {
  const normalized = value.replace(" ", "T");
  const date = new Date(normalized);
  if (Number.isNaN(date.getTime())) return value.slice(0, 10);
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}
