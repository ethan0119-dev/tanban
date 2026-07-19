Page({
  data: { version: "v0.1.0" },
  goOrders() { wx.switchTab({ url: "/pages/orders/index" }); },
  contact() { wx.showModal({ title: "联系商家", content: "请在门店首页查看商家联系方式。", showCancel: false }); },
});
