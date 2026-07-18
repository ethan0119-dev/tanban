"use client";

import { useMemo, useState } from "react";

type Surface = "platform" | "merchant" | "customer";
type MerchantPage = "dashboard" | "orders" | "products" | "members" | "marketing";

const money = new Intl.NumberFormat("zh-CN", {
  style: "currency",
  currency: "CNY",
  minimumFractionDigits: 0,
});

const surfaceMeta: Record<Surface, { label: string; eyebrow: string; title: string }> = {
  platform: {
    label: "平台管理端",
    eyebrow: "SaaS CONTROL CENTER",
    title: "把每一个小摊，经营成一个好品牌",
  },
  merchant: {
    label: "商户运营端",
    eyebrow: "STORE OPERATIONS",
    title: "码农咖啡 · 今晚出摊",
  },
  customer: {
    label: "顾客点单端",
    eyebrow: "WECHAT MINI PROGRAM",
    title: "扫一扫，三十秒点好一杯咖啡",
  },
};

const navIcons: Record<string, string> = {
  经营总览: "总",
  商户管理: "商",
  套餐计费: "费",
  小程序管理: "微",
  能力中心: "能",
  支付分账: "账",
  服务工单: "服",
  系统设置: "设",
  今日概况: "今",
  订单处理: "单",
  商品库存: "品",
  用户会员: "客",
  营销应用: "营",
  财务对账: "财",
  店铺装修: "装",
  数据分析: "数",
};

const tenants = [
  { name: "码农咖啡", owner: "杨先生", type: "移动咖啡摊", plan: "成长版", status: "营业中", gmv: "¥12,840" },
  { name: "一山烧鸟", owner: "陈女士", type: "夜市餐车", plan: "专业版", status: "营业中", gmv: "¥28,420" },
  { name: "叶子刨冰", owner: "王先生", type: "固定摊位", plan: "基础版", status: "已打烊", gmv: "¥6,920" },
  { name: "小野热狗", owner: "陆女士", type: "移动餐车", plan: "成长版", status: "待配置", gmv: "—" },
];

const initialOrders = [
  { id: "A083", time: "21:18", items: "经典美式 × 1 · 海盐拿铁 × 1", amount: 34, status: "待接单", source: "小程序自提" },
  { id: "A082", time: "21:12", items: "生椰拿铁 × 2", amount: 44, status: "制作中", source: "小程序自提" },
  { id: "A081", time: "21:05", items: "橙C气泡水 × 1", amount: 12, status: "待取餐", source: "现场扫码" },
  { id: "A080", time: "20:54", items: "桂花美式 × 1", amount: 16, status: "已完成", source: "现场扫码" },
];

const products = [
  { name: "经典美式", category: "美式", price: 13, sold: 48, stock: 949, on: true },
  { name: "海盐拿铁", category: "拿铁", price: 20, sold: 10, stock: 988, on: true },
  { name: "生椰拿铁", category: "拿铁", price: 22, sold: 8, stock: 992, on: true },
  { name: "桂花美式", category: "美式", price: 16, sold: 7, stock: 996, on: true },
  { name: "青柠话梅气泡水", category: "气泡水", price: 12, sold: 6, stock: 994, on: true },
  { name: "天津老味儿刨冰", category: "刨冰", price: 28, sold: 3, stock: 0, on: false },
];

const customerProducts = [
  { id: 1, name: "经典美式", subtitle: "坚果调 · 黑巧尾韵", price: 13, tag: "本店招牌", tone: "dark" },
  { id: 2, name: "海盐拿铁", subtitle: "咸甜奶盖 · 双倍浓缩", price: 20, tag: "人气 No.1", tone: "cream" },
  { id: 3, name: "生椰拿铁", subtitle: "清爽椰乳 · 低糖", price: 22, tag: "清爽", tone: "green" },
  { id: 4, name: "桂花美式", subtitle: "桂花蜜 · 冰爽回甘", price: 16, tag: "夜市限定", tone: "amber" },
  { id: 5, name: "橙C气泡水", subtitle: "鲜橙 · 气泡 · 0咖啡因", price: 12, tag: "解暑", tone: "orange" },
];

export function CoffeeSaaS() {
  const [surface, setSurface] = useState<Surface>("platform");
  const meta = surfaceMeta[surface];

  return (
    <main className={`site-shell surface-${surface}`}>
      <header className="global-header">
        <button className="brand" onClick={() => setSurface("platform")} aria-label="返回平台管理端">
          <span className="brand-mark">伴</span>
          <span>
            <strong>摊伴</strong>
            <small>TANBAN</small>
          </span>
        </button>
        <div className="surface-switcher" aria-label="选择体验端">
          {(Object.keys(surfaceMeta) as Surface[]).map((key) => (
            <button key={key} className={surface === key ? "active" : ""} onClick={() => setSurface(key)}>
              {surfaceMeta[key].label}
            </button>
          ))}
        </div>
        <div className="header-actions">
          <span className="live-dot"><i /> 三端联动演示</span>
          <button className="avatar" aria-label="账户菜单">YY</button>
        </div>
      </header>

      <section className="context-bar">
        <div>
          <p>{meta.eyebrow}</p>
          <h1>{meta.title}</h1>
        </div>
        <div className="context-chips">
          <span>微信支付已连接</span>
          <span>打印机在线</span>
          <span>订单提醒正常</span>
        </div>
      </section>

      {surface === "platform" && <PlatformDashboard />}
      {surface === "merchant" && <MerchantDashboard />}
      {surface === "customer" && <CustomerMiniProgram />}
    </main>
  );
}

function SideNav({
  items,
  active,
  onSelect,
  footer,
}: {
  items: string[];
  active: string;
  onSelect: (item: string) => void;
  footer: React.ReactNode;
}) {
  return (
    <aside className="side-nav">
      <div className="nav-items">
        {items.map((item) => (
          <button className={active === item ? "active" : ""} key={item} onClick={() => onSelect(item)}>
            <span>{navIcons[item] ?? item.slice(0, 1)}</span>
            {item}
          </button>
        ))}
      </div>
      {footer}
    </aside>
  );
}

function PlatformDashboard() {
  const nav = ["经营总览", "商户管理", "套餐计费", "小程序管理", "能力中心", "支付分账", "服务工单", "系统设置"];
  const [active, setActive] = useState("经营总览");

  return (
    <div className="workspace">
      <SideNav
        items={nav}
        active={active}
        onSelect={setActive}
        footer={
          <div className="nav-card">
            <small>本月平台收入</small>
            <strong>¥482,600</strong>
            <span>较上月 +12.8%</span>
          </div>
        }
      />
      <section className="workspace-content">
        <div className="page-heading">
          <div>
            <span className="kicker">平台运营中枢</span>
            <h2>{active}</h2>
            <p>统一管理租户、版本能力、计费与服务质量。</p>
          </div>
          <div className="heading-actions">
            <button className="button secondary">导出经营月报</button>
            <button className="button primary">+ 新增商户</button>
          </div>
        </div>

        {active === "经营总览" ? <PlatformOverview /> : <PlatformModule name={active} />}
      </section>
    </div>
  );
}

function PlatformOverview() {
  return (
    <>
      <div className="metric-grid platform-metrics">
        <MetricCard label="活跃商户" value="1,286" change="+38 本月" tone="mint" />
        <MetricCard label="平台交易额" value="¥326.8万" change="+18.6%" tone="amber" />
        <MetricCard label="订阅收入 MRR" value="¥48.26万" change="+12.8%" tone="violet" />
        <MetricCard label="在线门店" value="842" change="峰值 1,024" tone="blue" />
      </div>

      <div className="dashboard-grid">
        <article className="panel wide">
          <div className="panel-title">
            <div><small>GMV TREND</small><h3>近 7 日交易趋势</h3></div>
            <span className="tag positive">环比 +18.6%</span>
          </div>
          <div className="chart" aria-label="近七日交易额柱状图">
            {[52, 68, 61, 78, 82, 76, 94].map((height, index) => (
              <div key={height + index} className="chart-col">
                <span style={{ height: `${height}%` }}><i /></span>
                <small>{["一", "二", "三", "四", "五", "六", "日"][index]}</small>
              </div>
            ))}
          </div>
        </article>
        <article className="panel">
          <div className="panel-title"><div><small>PLAN MIX</small><h3>套餐分布</h3></div></div>
          <div className="donut-row">
            <div className="donut"><strong>1,286</strong><span>商户</span></div>
            <div className="legend">
              <p><i className="legend-a" />基础版 <b>38%</b></p>
              <p><i className="legend-b" />成长版 <b>44%</b></p>
              <p><i className="legend-c" />专业版 <b>18%</b></p>
            </div>
          </div>
        </article>
      </div>

      <article className="panel table-panel">
        <div className="panel-title">
          <div><small>TENANT PULSE</small><h3>商户经营脉搏</h3></div>
          <button className="text-button">查看全部 →</button>
        </div>
        <div className="data-table tenant-table">
          <div className="table-row table-head"><span>商户</span><span>经营形态</span><span>套餐</span><span>本月交易额</span><span>状态</span></div>
          {tenants.map((tenant) => (
            <div className="table-row" key={tenant.name}>
              <span className="merchant-cell"><i>{tenant.name.slice(0, 1)}</i><b>{tenant.name}<small>{tenant.owner}</small></b></span>
              <span>{tenant.type}</span><span>{tenant.plan}</span><span><b>{tenant.gmv}</b></span>
              <span><em className={`status ${tenant.status === "营业中" ? "online" : tenant.status === "待配置" ? "waiting" : "closed"}`}>{tenant.status}</em></span>
            </div>
          ))}
        </div>
      </article>

      <div className="system-strip">
        <div><i className="pulse" /><span><b>平台服务稳定</b><small>API 成功率 99.98%</small></span></div>
        <div><b>24 ms</b><small>订单平均延迟</small></div>
        <div><b>842 / 846</b><small>打印设备在线</small></div>
        <div><b>2</b><small>待处理工单</small></div>
      </div>
    </>
  );
}

const platformModules: Record<string, { summary: string; actions: string[]; rows: string[][] }> = {
  商户管理: { summary: "覆盖入驻、门店、员工、经营状态和风控全生命周期。", actions: ["商户审核", "批量导入", "停用/恢复"], rows: [["码农咖啡", "成长版", "天津", "正常"], ["一山烧鸟", "专业版", "北京", "正常"], ["小野热狗", "成长版", "上海", "待配置"]] },
  套餐计费: { summary: "按版本打包经营能力，支持试用、续费、升级与优惠。", actions: ["新建套餐", "定价策略", "优惠码"], rows: [["基础版", "¥299/年", "482 户", "在售"], ["成长版", "¥699/年", "566 户", "主推"], ["专业版", "¥1,299/年", "238 户", "在售"]] },
  小程序管理: { summary: "集中维护小程序授权、版本、模板、审核与灰度发布。", actions: ["模板版本", "提交审核", "灰度发布"], rows: [["点单标准版", "v3.8.2", "1,068 户", "已发布"], ["餐车轻量版", "v2.4.0", "218 户", "已发布"], ["会员商城版", "v1.9.1", "96 户", "审核中"]] },
  能力中心: { summary: "用功能开关控制订单、商品、会员、营销、打印与配送能力。", actions: ["能力开关", "版本映射", "用量限制"], rows: [["小票云打印", "全版本", "99.7%", "正常"], ["会员储值", "成长版起", "38.2%", "正常"], ["同城配送", "专业版", "16.8%", "正常"]] },
  支付分账: { summary: "统一查看微信支付交易、退款、结算与服务费分账。", actions: ["渠道配置", "结算规则", "异常对账"], rows: [["微信支付", "¥326.8万", "T+1", "正常"], ["余额支付", "¥18.2万", "实时", "正常"], ["线下记账", "¥6.4万", "—", "人工核对"]] },
  服务工单: { summary: "将商户问题按支付、打印、商品和小程序分类流转。", actions: ["待分配", "服务 SLA", "知识库"], rows: [["#2084", "打印机掉线", "码农咖啡", "处理中"], ["#2083", "退款未到账", "叶子刨冰", "待回复"], ["#2082", "小程序审核", "一山烧鸟", "已解决"]] },
  系统设置: { summary: "维护平台品牌、组织权限、通知、审计与安全策略。", actions: ["组织角色", "消息通道", "审计日志"], rows: [["超级管理员", "全部权限", "3 人", "启用"], ["商户运营", "租户/套餐", "8 人", "启用"], ["技术支持", "工单/日志", "6 人", "启用"]] },
};

function PlatformModule({ name }: { name: string }) {
  const module = platformModules[name] ?? platformModules["商户管理"];
  return (
    <div className="module-layout">
      <article className="panel module-hero">
        <div><span className="module-icon">{navIcons[name]}</span><small>PLATFORM MODULE</small><h3>{name}</h3><p>{module.summary}</p></div>
        <button className="button primary">进入配置</button>
      </article>
      <div className="action-cards">
        {module.actions.map((action, index) => <button key={action}><i>0{index + 1}</i><b>{action}</b><span>查看与管理 →</span></button>)}
      </div>
      <article className="panel table-panel">
        <div className="panel-title"><div><small>LATEST RECORDS</small><h3>最近记录</h3></div><button className="text-button">筛选</button></div>
        <div className="simple-records">
          {module.rows.map((row) => <div key={row.join("-")}>{row.map((cell, i) => <span key={cell}>{i === 0 ? <b>{cell}</b> : cell}</span>)}</div>)}
        </div>
      </article>
    </div>
  );
}

function MerchantDashboard() {
  const navMap: Record<string, MerchantPage> = { 今日概况: "dashboard", 订单处理: "orders", 商品库存: "products", 用户会员: "members", 营销应用: "marketing" };
  const [page, setPage] = useState<MerchantPage>("dashboard");
  const active = Object.entries(navMap).find(([, value]) => value === page)?.[0] ?? "今日概况";
  const [open, setOpen] = useState(true);

  return (
    <div className="workspace merchant-workspace">
      <SideNav
        items={["今日概况", "订单处理", "商品库存", "用户会员", "营销应用", "财务对账", "店铺装修", "数据分析"]}
        active={active}
        onSelect={(item) => setPage(navMap[item] ?? "dashboard")}
        footer={
          <div className="store-card">
            <span className={open ? "store-status" : "store-status off"}><i /> {open ? "营业中" : "已打烊"}</span>
            <strong>码农咖啡</strong>
            <small>天津 · 移动咖啡摊</small>
            <button onClick={() => setOpen(!open)}>{open ? "暂停接单" : "开始营业"}</button>
          </div>
        }
      />
      <section className="workspace-content">
        {page === "dashboard" && <MerchantOverview open={open} />}
        {page === "orders" && <OrderBoard />}
        {page === "products" && <ProductManager />}
        {page === "members" && <MemberCenter />}
        {page === "marketing" && <MarketingCenter />}
      </section>
    </div>
  );
}

function MerchantOverview({ open }: { open: boolean }) {
  return (
    <>
      <div className="page-heading merchant-heading">
        <div><span className="kicker">7 月 18 日 · 周六</span><h2>晚上好，杨老板</h2><p>{open ? "摊位营业中，打印机和新订单提醒均正常。" : "当前已暂停接单，顾客端会显示休息中。"}</p></div>
        <div className="heading-actions"><button className="button secondary">查看小程序码</button><button className="button primary">进入收银台</button></div>
      </div>
      <div className="metric-grid merchant-metrics">
        <MetricCard label="今日营业额" value="¥826.00" change="较昨日 +18.2%" tone="dark" />
        <MetricCard label="有效订单" value="42 单" change="客单价 ¥19.67" tone="mint" />
        <MetricCard label="待制作" value="3 杯" change="预计 8 分钟" tone="amber" />
        <MetricCard label="今日新客" value="16 人" change="新客占比 38%" tone="cream" />
      </div>
      <div className="dashboard-grid merchant-grid">
        <article className="panel live-orders">
          <div className="panel-title"><div><small>LIVE ORDERS</small><h3>正在出杯</h3></div><span className="tag positive"><i /> 实时更新</span></div>
          {initialOrders.slice(0, 3).map((order, index) => (
            <div className="compact-order" key={order.id}>
              <span className={`order-number n${index + 1}`}>{order.id}</span>
              <span><b>{order.items}</b><small>{order.time} · {order.source}</small></span>
              <em>{order.status}</em>
            </div>
          ))}
          <button className="full-link">查看全部订单</button>
        </article>
        <article className="panel product-rank">
          <div className="panel-title"><div><small>TOP PRODUCTS</small><h3>今日热卖</h3></div><span className="tag">销量</span></div>
          {[{ name: "经典美式", count: 18, width: 92 }, { name: "海盐拿铁", count: 11, width: 68 }, { name: "生椰拿铁", count: 8, width: 52 }, { name: "桂花美式", count: 5, width: 36 }].map((p, i) => (
            <div className="rank-row" key={p.name}><i>{i + 1}</i><span><b>{p.name}</b><small><em style={{ width: `${p.width}%` }} /></small></span><strong>{p.count}</strong></div>
          ))}
        </article>
      </div>
      <article className="panel device-panel">
        <div><span className="device-icon">印</span><span><b>飞鹅云打印机 FE-2219</b><small>最后心跳：刚刚 · 自动接单并打印</small></span></div>
        <span className="status online">设备在线</span>
        <button className="button secondary small">打印测试页</button>
      </article>
    </>
  );
}

function OrderBoard() {
  const [orders, setOrders] = useState(initialOrders);
  const advance = (id: string) => {
    const flow = ["待接单", "制作中", "待取餐", "已完成"];
    setOrders((current) => current.map((order) => order.id === id ? { ...order, status: flow[Math.min(flow.indexOf(order.status) + 1, flow.length - 1)] } : order));
  };
  return (
    <>
      <div className="page-heading"><div><span className="kicker">ORDER WORKBENCH</span><h2>订单处理</h2><p>新订单自动接单并打印，按出杯进度推进状态。</p></div><div className="heading-actions"><span className="auto-print"><i /> 自动接单已开启</span><button className="button primary">+ 现场开单</button></div></div>
      <div className="order-board">
        {["待接单", "制作中", "待取餐", "已完成"].map((status) => (
          <section key={status}>
            <header><h3>{status}</h3><span>{orders.filter(o => o.status === status).length}</span></header>
            {orders.filter(o => o.status === status).map((order) => (
              <article className="order-card" key={order.id}>
                <div><strong>#{order.id}</strong><span>{order.time}</span></div>
                <p>{order.items}</p><small>{order.source}</small><b>{money.format(order.amount)}</b>
                {status !== "已完成" && <button onClick={() => advance(order.id)}>{status === "待接单" ? "接单并打印" : status === "制作中" ? "完成制作" : "确认取餐"} →</button>}
              </article>
            ))}
            {orders.filter(o => o.status === status).length === 0 && <div className="empty-column">暂无订单</div>}
          </section>
        ))}
      </div>
    </>
  );
}

function ProductManager() {
  const [catalog, setCatalog] = useState(products);
  return (
    <>
      <div className="page-heading"><div><span className="kicker">CATALOG & STOCK</span><h2>商品库存</h2><p>同一商品可分别控制外卖、店内、快递渠道与实时库存。</p></div><div className="heading-actions"><button className="button secondary">分类管理</button><button className="button primary">+ 添加商品</button></div></div>
      <div className="filter-row"><button className="active">全部 31</button><button>已上架 29</button><button>库存不足 1</button><button>已售罄 1</button><label><span>⌕</span><input placeholder="搜索商品" /></label></div>
      <article className="panel table-panel product-table">
        <div className="data-table">
          <div className="table-row table-head"><span>商品</span><span>分类</span><span>售价</span><span>销量</span><span>库存</span><span>小程序上架</span></div>
          {catalog.map((product) => (
            <div className="table-row" key={product.name}>
              <span className="product-cell"><i>{product.name.slice(0, 1)}</i><b>{product.name}<small>{product.on ? "外卖 · 店内" : "已售罄"}</small></b></span>
              <span>{product.category}</span><span><b>{money.format(product.price)}</b></span><span>{product.sold}</span><span className={product.stock === 0 ? "danger" : ""}>{product.stock}</span>
              <span><button className={`toggle ${product.on ? "on" : ""}`} onClick={() => setCatalog((items) => items.map(p => p.name === product.name ? { ...p, on: !p.on } : p))}><i /></button></span>
            </div>
          ))}
        </div>
      </article>
    </>
  );
}

function MemberCenter() {
  return (
    <>
      <div className="page-heading"><div><span className="kicker">MEMBER GROWTH</span><h2>用户会员</h2><p>沉淀扫码顾客，统一管理标签、等级、余额与积分。</p></div><div className="heading-actions"><button className="button primary">发放会员权益</button></div></div>
      <div className="metric-grid"><MetricCard label="累计顾客" value="1,842" change="本月 +126" tone="cream" /><MetricCard label="付费会员" value="286" change="会员占比 15.5%" tone="mint" /><MetricCard label="储值余额" value="¥38,420" change="待履约资金" tone="amber" /><MetricCard label="30 日复购率" value="32.8%" change="+4.2%" tone="violet" /></div>
      <div className="dashboard-grid"><article className="panel member-list"><div className="panel-title"><div><small>RECENT CUSTOMERS</small><h3>最近到店顾客</h3></div></div>{[["林小满", "咖啡爱好者", "18 次", "¥426"], ["Kiki", "夜市常客", "11 次", "¥238"], ["Jason", "新客", "2 次", "¥42"], ["阿布", "高价值会员", "26 次", "¥682"]].map((r, i) => <div key={r[0]}><i>{r[0].slice(0, 1)}</i><span><b>{r[0]}</b><small>{r[1]}</small></span><span>{r[2]}</span><strong>{r[3]}</strong></div>)}</article><article className="panel retention-card"><div className="panel-title"><div><small>RETENTION</small><h3>会员复购漏斗</h3></div></div>{[["首次下单", "100%"], ["二次复购", "48%"], ["加入会员", "31%"], ["月度活跃", "18%"]].map((r, i) => <div key={r[0]}><span>{r[0]}<b>{r[1]}</b></span><i><em style={{ width: r[1] }} /></i></div>)}</article></div>
    </>
  );
}

function MarketingCenter() {
  const apps = [["券", "优惠券", "拉新促活，提升复购", "已启用"], ["满", "满额立减", "提高客单价与下单转化", "已启用"], ["储", "会员储值", "锁定长期消费", "去配置"], ["点", "集点返礼", "培养高频忠实顾客", "去启用"], ["新", "新客立减", "扫码首单自动优惠", "已启用"], ["签", "积分签到", "每日到店互动", "去启用"]];
  return (
    <><div className="page-heading"><div><span className="kicker">GROWTH APPS</span><h2>营销应用</h2><p>围绕拉新、转化和复购，按需开启轻量经营工具。</p></div><div className="heading-actions"><button className="button secondary">活动数据</button></div></div><div className="app-grid">{apps.map((app) => <article key={app[1]}><i>{app[0]}</i><div><h3>{app[1]}</h3><p>{app[2]}</p></div><button className={app[3] === "已启用" ? "enabled" : ""}>{app[3]}</button></article>)}</div></>
  );
}

function CustomerMiniProgram() {
  const [category, setCategory] = useState("咖啡");
  const [cart, setCart] = useState<Record<number, number>>({});
  const [checkout, setCheckout] = useState(false);
  const [placed, setPlaced] = useState(false);
  const count = Object.values(cart).reduce((a, b) => a + b, 0);
  const total = customerProducts.reduce((sum, p) => sum + (cart[p.id] ?? 0) * p.price, 0);
  const add = (id: number) => setCart((current) => ({ ...current, [id]: (current[id] ?? 0) + 1 }));
  const remove = (id: number) => setCart((current) => ({ ...current, [id]: Math.max((current[id] ?? 0) - 1, 0) }));

  return (
    <section className="customer-stage">
      <div className="customer-copy">
        <span className="kicker">从扫码到取杯，只有四步</span>
        <h2>顾客少等，摊主少忙。</h2>
        <p>顾客扫码打开小程序，选咖啡并微信支付；新订单同步到商户端、微信提醒与云打印机，制作完成后凭取餐码取杯。</p>
        <ol>
          <li><i>01</i><span><b>扫码进入</b><small>自动识别门店与桌码</small></span></li>
          <li><i>02</i><span><b>自助点单</b><small>规格、加料与优惠实时计算</small></span></li>
          <li><i>03</i><span><b>微信支付</b><small>支付成功即刻推送并打印</small></span></li>
          <li><i>04</i><span><b>凭码取杯</b><small>状态变化同步给顾客</small></span></li>
        </ol>
        <div className="integration-note"><span>一笔订单</span><b>小程序 → 商户端 → 微信提醒 → 小票打印</b></div>
      </div>

      <div className="phone-wrap">
        <div className="phone">
          <div className="phone-bar"><span>21:18</span><i>● ●● ▰</i></div>
          {!placed ? (
            <>
              <div className="store-hero">
                <div className="mini-nav"><span>‹</span><b>码农咖啡</b><span>••• ◉</span></div>
                <div className="store-info"><span className="cup-logo">码</span><div><h3>码农咖啡</h3><p><i /> 营业中 · 预计 8 分钟出杯</p></div><button>♡</button></div>
                <div className="notice">今晚在海河边，吹着晚风喝咖啡。</div>
              </div>
              <div className="mini-tabs"><button className="active">点单</button><button>评价 4.9</button><button>商家</button></div>
              <div className="menu-area">
                <aside>{["咖啡", "拿铁", "气泡水", "夜市限定"].map(c => <button className={category === c ? "active" : ""} onClick={() => setCategory(c)} key={c}>{c}</button>)}</aside>
                <div className="mini-products">
                  <div className="category-title"><b>{category}</b><span>现点现做</span></div>
                  {customerProducts.filter((p) => category === "咖啡" || (category === "拿铁" ? p.name.includes("拿铁") : category === "气泡水" ? p.name.includes("气泡") : p.name.includes("桂花"))).map((p) => (
                    <article key={p.id}>
                      <div className={`coffee-art ${p.tone}`}><span>{p.name.slice(0, 1)}</span></div>
                      <div><em>{p.tag}</em><h4>{p.name}</h4><p>{p.subtitle}</p><strong>¥{p.price}</strong></div>
                      <div className="quantity">
                        {(cart[p.id] ?? 0) > 0 && <><button onClick={() => remove(p.id)}>−</button><span>{cart[p.id]}</span></>}
                        <button className="plus" onClick={() => add(p.id)}>＋</button>
                      </div>
                    </article>
                  ))}
                </div>
              </div>
              {count > 0 && <div className="mini-cart"><button className="cart-icon" onClick={() => setCheckout(true)}>袋<span>{count}</span></button><div><b>¥{total}</b><small>已优惠 ¥2</small></div><button className="checkout-button" onClick={() => setCheckout(true)}>去结算</button></div>}
              {checkout && <div className="checkout-sheet"><button className="sheet-close" onClick={() => setCheckout(false)}>×</button><span className="sheet-handle" /><h3>确认订单</h3><div className="pickup-select"><button className="active"><b>到店自取</b><small>约 8 分钟可取</small></button><button><b>稍后自取</b><small>预约时间</small></button></div><div className="sheet-lines">{customerProducts.filter(p => cart[p.id]).map(p => <p key={p.id}><span>{p.name} × {cart[p.id]}</span><b>¥{p.price * cart[p.id]}</b></p>)}<p><span>优惠券</span><em>-¥2</em></p></div><label className="remark"><span>备注</span><input placeholder="少冰、少糖等" /></label><button className="pay-button" onClick={() => { setCheckout(false); setPlaced(true); }}>微信支付 ¥{Math.max(total - 2, 0)}</button></div>}
            </>
          ) : (
            <div className="success-screen"><div className="mini-nav"><span>‹</span><b>支付结果</b><span>••• ◉</span></div><div className="success-mark">✓</div><h3>支付成功</h3><p>订单已推送给码农咖啡</p><div className="pickup-code"><small>取餐码</small><strong>A084</strong><span>预计 21:28 可取</span></div><div className="success-flow"><span className="done"><i /> 已支付</span><b /><span className="active"><i /> 制作中</span><b /><span><i /> 待取餐</span></div><article><h4>订单详情</h4><p><span>{count} 件商品</span><b>¥{Math.max(total - 2, 0)}</b></p><p><span>取单方式</span><b>到店自取</b></p><p><span>订单编号</span><b>202607182118084</b></p></article><button className="back-menu" onClick={() => { setPlaced(false); setCart({}); }}>再来一单</button></div>
          )}
          <div className="home-indicator" />
        </div>
      </div>
    </section>
  );
}

function MetricCard({ label, value, change, tone }: { label: string; value: string; change: string; tone: string }) {
  return <article className={`metric-card ${tone}`}><div><span>{label}</span><i>↗</i></div><strong>{value}</strong><small>{change}</small></article>;
}
