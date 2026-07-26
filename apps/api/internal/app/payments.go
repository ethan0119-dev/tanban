package app

import (
	"database/sql"
	"net/http"
)

func (s *Server) listOrderPayments(w http.ResponseWriter, r *http.Request) {
	orderID, ok := pathID(w, r, "orderID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var exists int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM orders WHERE tenant_id=? AND id=?",
		identity.TenantID, orderID).Scan(&exists); err != nil {
		handleSQLError(w, err)
		return
	}
	if exists != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "订单不存在")
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,provider,payment_method,provider_order_no,amount_cents,status,
		IF(paid_at IS NULL,NULL,DATE_FORMAT(paid_at,'%Y-%m-%d %H:%i:%s')),
		DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s')
		FROM payment_transactions WHERE tenant_id=? AND order_id=? ORDER BY id DESC`,
		identity.TenantID, orderID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, amount int64
		var providerName, paymentMethod, providerNo, status, createdAt string
		var paidAt sql.NullString
		if err = rows.Scan(&id, &providerName, &paymentMethod, &providerNo, &amount, &status, &paidAt, &createdAt); err != nil {
			handleSQLError(w, err)
			return
		}
		var paid any
		if paidAt.Valid {
			paid = paidAt.String
		}
		items = append(items, map[string]any{
			"id": id, "order_id": orderID, "provider": providerName, "payment_method": paymentMethod,
			"provider_order_no": providerNo, "amount_cents": amount, "status": status,
			"paid_at": paid, "created_at": createdAt,
		})
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	balanceRows, err := s.DB.QueryContext(r.Context(), `SELECT id,amount_cents,status,
		DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM order_balance_payments
		WHERE tenant_id=? AND order_id=? ORDER BY id DESC`, identity.TenantID, orderID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer balanceRows.Close()
	for balanceRows.Next() {
		var id, amount int64
		var status, createdAt string
		if err = balanceRows.Scan(&id, &amount, &status, &createdAt); err != nil {
			handleSQLError(w, err)
			return
		}
		if status == "APPLIED" {
			status = "SUCCESS"
		}
		items = append(items, map[string]any{
			"id": "balance-" + int64String(id), "order_id": orderID, "provider": "balance",
			"payment_method": "BALANCE", "provider_order_no": "BALANCE-" + int64String(id),
			"amount_cents": amount, "status": status,
			"paid_at": createdAt, "created_at": createdAt,
		})
	}
	if err = balanceRows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM payment_transactions WHERE tenant_id=?", identity.TenantID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT p.id,p.order_id,o.order_no,p.provider,p.provider_order_no,p.amount_cents,p.status,
		IF(p.paid_at IS NULL,NULL,DATE_FORMAT(p.paid_at,'%Y-%m-%d %H:%i:%s')),
		COALESCE((SELECT SUM(rf.amount_cents) FROM refunds rf WHERE rf.payment_id=p.id AND rf.status IN ('SUCCESS','REFUNDED')),0),
		DATE_FORMAT(p.created_at,'%Y-%m-%d %H:%i:%s')
		FROM payment_transactions p JOIN orders o ON o.id=p.order_id
		WHERE p.tenant_id=? ORDER BY p.id DESC LIMIT ? OFFSET ?`, identity.TenantID, size, offset)
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
