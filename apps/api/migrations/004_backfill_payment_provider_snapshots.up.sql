UPDATE payment_transactions p
JOIN orders o ON o.id=p.order_id AND o.tenant_id=p.tenant_id
JOIN tenants t ON t.id=p.tenant_id
SET p.merchant_no=IF(p.merchant_no='',t.payment_merchant_no,p.merchant_no),
    p.sub_appid=IF(p.sub_appid='',t.payment_sub_appid,p.sub_appid),
    p.customer_openid=IF(p.customer_openid='',o.customer_openid,p.customer_openid);
