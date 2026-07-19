package app

import (
	"database/sql"
	"net/http"
)

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_transactions WHERE tenant_id=?", identity.TenantID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT p.id,p.order_id,o.order_no,p.provider,p.provider_order_no,p.amount_cents,p.status,IF(p.paid_at IS NULL,NULL,DATE_FORMAT(p.paid_at,'%Y-%m-%dT%H:%i:%sZ')),o.refunded_cents,DATE_FORMAT(p.created_at,'%Y-%m-%dT%H:%i:%sZ') FROM payment_transactions p JOIN orders o ON o.id=p.order_id WHERE p.tenant_id=? ORDER BY p.id DESC LIMIT ? OFFSET ?`, identity.TenantID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, orderID, amount, refunded int64
		var orderNo, providerName, providerNo, status, created string
		var paidAt sql.NullString
		if err := rows.Scan(&id, &orderID, &orderNo, &providerName, &providerNo, &amount, &status, &paidAt, &refunded, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		var paid any
		if paidAt.Valid {
			paid = paidAt.String
		}
		if status == "SUCCESS" && refunded > 0 {
			status = "PARTIALLY_REFUNDED"
			if refunded >= amount {
				status = "REFUNDED"
			}
		}
		items = append(items, map[string]any{"id": id, "order_id": orderID, "order_no": orderNo, "provider": providerName, "provider_order_no": providerNo, "amount_cents": amount, "status": status, "paid_at": paid, "refunded_cents": refunded, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}
