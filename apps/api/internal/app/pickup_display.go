package app

import (
	"net/http"
	"strings"
	"time"
)

type pickupDisplayOrder struct {
	ID         int64  `json:"id"`
	PickupCode string `json:"pickupCode"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type pickupDisplayResponse struct {
	StoreName    string               `json:"storeName"`
	StoreLogoURL string               `json:"storeLogoUrl,omitempty"`
	BusinessDate string               `json:"businessDate"`
	UpdatedAt    string               `json:"updatedAt"`
	Preparing    []pickupDisplayOrder `json:"preparing"`
	Ready        []pickupDisplayOrder `json:"ready"`
}

func pickupDisplayColumn(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "PAID", "ACCEPTED", "PREPARING":
		return "PREPARING"
	case "READY":
		return "READY"
	default:
		return ""
	}
}

func (s *Server) getPickupDisplay(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}

	var storeName, storeLogoURL, timezone string
	err = s.DB.QueryRowContext(r.Context(), `SELECT name,logo_url,timezone FROM stores
		WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL`, storeID, actor.TenantID).
		Scan(&storeName, &storeLogoURL, &timezone)
	if err != nil {
		handleSQLError(w, err)
		return
	}

	now := time.Now()
	state, err := storeBusinessStateAt(r.Context(), s.DB, actor.TenantID, storeID, timezone, now)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,pickup_code,status,
		DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s'),DATE_FORMAT(updated_at,'%Y-%m-%d %H:%i:%s')
		FROM orders
		WHERE tenant_id=? AND store_id=? AND order_type='TAKEOUT'
		  AND (
		    status IN ('PAID','ACCEPTED','PREPARING')
		    OR (status='READY' AND updated_at>=DATE_SUB(NOW(3), INTERVAL 1 DAY))
		  )
		  AND pickup_code<>''
		ORDER BY CASE WHEN status='READY' THEN 1 ELSE 0 END,updated_at,pickup_sequence,id`,
		actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()

	response := pickupDisplayResponse{
		StoreName:    storeName,
		StoreLogoURL: storeLogoURL,
		BusinessDate: state.BusinessDate,
		UpdatedAt:    formatBeijingDateTime(now),
		Preparing:    []pickupDisplayOrder{},
		Ready:        []pickupDisplayOrder{},
	}
	for rows.Next() {
		var item pickupDisplayOrder
		if err = rows.Scan(&item.ID, &item.PickupCode, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		switch pickupDisplayColumn(item.Status) {
		case "PREPARING":
			response.Preparing = append(response.Preparing, item)
		case "READY":
			response.Ready = append(response.Ready, item)
		}
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeData(w, http.StatusOK, response)
}
