package app

import (
	"context"
	"net/http"
	"strings"
)

type marketingAppDefinition struct {
	Key         string
	Name        string
	Description string
	Route       string
	CountQuery  string
	CountArgs   func(tenantID, storeID int64) []any
}

type marketingAppStatusCounts struct {
	Draft  int
	Active int
	Paused int
	Ended  int
}

func (counts marketingAppStatusCounts) summary() string {
	if counts.Draft+counts.Active+counts.Paused+counts.Ended == 0 {
		return "NOT_CONFIGURED"
	}
	parts := make([]string, 0, 4)
	if counts.Active > 0 {
		parts = append(parts, "ACTIVE:"+int64String(int64(counts.Active)))
	}
	if counts.Paused > 0 {
		parts = append(parts, "PAUSED:"+int64String(int64(counts.Paused)))
	}
	if counts.Draft > 0 {
		parts = append(parts, "DRAFT:"+int64String(int64(counts.Draft)))
	}
	if counts.Ended > 0 {
		parts = append(parts, "ENDED:"+int64String(int64(counts.Ended)))
	}
	return strings.Join(parts, " / ")
}

func (s *Server) marketingAppStatus(ctx context.Context, query string, args ...any) (marketingAppStatusCounts, error) {
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return marketingAppStatusCounts{}, err
	}
	defer rows.Close()
	var counts marketingAppStatusCounts
	for rows.Next() {
		var status string
		var count int
		if err = rows.Scan(&status, &count); err != nil {
			return marketingAppStatusCounts{}, err
		}
		switch strings.ToUpper(status) {
		case "DRAFT":
			counts.Draft += count
		case "ACTIVE":
			counts.Active += count
		case "PAUSED":
			counts.Paused += count
		case "ENDED":
			counts.Ended += count
		}
	}
	return counts, rows.Err()
}

func (s *Server) listMarketingApps(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}

	couponCountQuery := `SELECT status,COUNT(*) FROM coupon_campaigns
		WHERE tenant_id=? AND store_id=? AND coupon_type=? AND deleted_at IS NULL GROUP BY status`
	definitions := []marketingAppDefinition{
		{
			Key: "COUPON", Name: "优惠券", Route: "/marketing/coupons",
			Description: "创建代金券，配置领取周期、库存、每人限领和适用订单类型",
			CountQuery:  couponCountQuery,
			CountArgs:   func(tenantID, storeID int64) []any { return []any{tenantID, storeID, "CASH"} },
		},
		{
			Key: "FULL_REDUCTION", Name: "满额立减", Route: "/marketing/full-reductions",
			Description: "达到订单门槛后自动减免，可与一张优惠券叠加",
			CountQuery: `SELECT status,COUNT(*) FROM store_full_reduction_campaigns
				WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL GROUP BY status`,
			CountArgs: func(tenantID, storeID int64) []any { return []any{tenantID, storeID} },
		},
		{
			Key: "POPUP_AD", Name: "弹窗广告", Route: "/marketing/popup-ads",
			Description: "配置首页、点单、结算、订单结果和会员中心弹窗的素材、频次、动作与优先级",
			CountQuery: `SELECT status,COUNT(*) FROM marketing_placements
				WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL GROUP BY status`,
			CountArgs: func(tenantID, storeID int64) []any { return []any{tenantID, storeID} },
		},
		{
			Key: "LOTTERY", Name: "抽奖活动", Route: "/marketing/lottery",
			Description: "配置免费抽奖周期、参与次数、奖项权重、优惠券奖品和库存",
			CountQuery: `SELECT status,COUNT(*) FROM lottery_campaigns
				WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL GROUP BY status`,
			CountArgs: func(tenantID, storeID int64) []any { return []any{tenantID, storeID} },
		},
	}

	items := make([]map[string]any, 0, len(definitions))
	for _, definition := range definitions {
		counts, queryErr := s.marketingAppStatus(r.Context(), definition.CountQuery, definition.CountArgs(actor.TenantID, storeID)...)
		if queryErr != nil {
			handleSQLError(w, queryErr)
			return
		}
		items = append(items, map[string]any{
			"key": definition.Key, "name": definition.Name, "status": counts.summary(), "available": true,
			"description": definition.Description, "route": definition.Route,
		})
	}
	writeData(w, http.StatusOK, items)
}
