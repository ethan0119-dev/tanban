package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// memberRoutes is intentionally self-contained so the merchant router can mount
// the whole CRM domain without weakening its existing order/staff boundaries.
func (s *Server) memberRoutes(r chi.Router) {
	r.Group(func(managers chi.Router) {
		managers.Use(requireMerchantCapability(capabilityManageMembers))
		managers.Get("/member-summary", s.getMemberSummary)
		managers.Get("/customers", s.listCustomers)
		managers.Post("/customers", s.createCustomer)
		managers.Get("/customers/{customerID}", s.getCustomer)
		managers.Put("/customers/{customerID}", s.updateCustomer)
		managers.With(requireMerchantCapability(capabilityArchiveCustomers)).Delete("/customers/{customerID}", s.archiveCustomer)
		managers.Put("/customers/{customerID}/tags", s.replaceCustomerTags)

		managers.Get("/customer-tags", s.listCustomerTags)
		managers.Post("/customer-tags", s.createCustomerTag)
		managers.Put("/customer-tags/{tagID}", s.updateCustomerTag)
		managers.Delete("/customer-tags/{tagID}", s.deleteCustomerTag)

		managers.Get("/member-levels", s.listMemberLevels)
		managers.Post("/member-levels", s.createMemberLevel)
		managers.Put("/member-levels/{levelID}", s.updateMemberLevel)
		managers.Delete("/member-levels/{levelID}", s.deleteMemberLevel)
		managers.Get("/membership-settings", s.getMembershipSettings)
		managers.Put("/membership-settings", s.updateMembershipSettings)
		managers.Get("/member-card-issuances", s.listMemberCardIssuances)
		managers.Post("/member-card-issuances", s.issueMemberCard)
		managers.Get("/member-level-orders", s.listMemberLevelOrders)
		managers.Post("/member-level-orders", s.createMemberLevelOrder)

		managers.Get("/balance-ledger", s.listBalanceLedger)
		managers.With(requireMerchantCapability(capabilityAdjustCustomerBalance)).Post("/customers/{customerID}/balance-adjustments", s.adjustCustomerBalance)

		managers.Get("/stored-value-rules", s.listStoredValueRules)
		managers.Post("/stored-value-rules", s.createStoredValueRule)
		managers.Put("/stored-value-rules/{ruleID}", s.updateStoredValueRule)
		managers.Delete("/stored-value-rules/{ruleID}", s.deleteStoredValueRule)
		managers.Get("/stored-value-settings", s.getStoredValueSettings)
		managers.Put("/stored-value-settings", s.updateStoredValueSettings)
		managers.Get("/stored-value-records", s.listStoredValueRecords)
		managers.With(requireMerchantCapability(capabilityCreateStoredValueRecord)).Post("/stored-value-records", s.createStoredValueRecord)
	})
}

type memberSummary struct {
	CustomerCount              int64 `json:"customer_count"`
	MemberCount                int64 `json:"member_count"`
	BalanceCents               int64 `json:"balance_cents"`
	BlockedCustomerCount       int64 `json:"blocked_customer_count"`
	StoredValuePrincipalCents  int64 `json:"stored_value_principal_cents"`
	StoredValueGiftCents       int64 `json:"stored_value_gift_cents"`
	StoredValueCustomerCount   int64 `json:"stored_value_customer_count"`
	ActiveStoredValueRuleCount int64 `json:"active_stored_value_rule_count"`
}

const memberSummaryQuery = `SELECT
	(SELECT COUNT(*) FROM customers WHERE tenant_id=? AND deleted_at IS NULL),
	(SELECT COUNT(*) FROM members m JOIN customers c ON c.id=m.customer_id AND c.tenant_id=m.tenant_id WHERE m.tenant_id=? AND c.deleted_at IS NULL),
	(SELECT COALESCE(SUM(ba.principal_cents+ba.bonus_cents),0) FROM balance_accounts ba JOIN customers c ON c.id=ba.customer_id AND c.tenant_id=ba.tenant_id WHERE ba.tenant_id=? AND c.deleted_at IS NULL),
	(SELECT COUNT(*) FROM customers WHERE tenant_id=? AND deleted_at IS NULL AND status='BLOCKED'),
	(SELECT COALESCE(SUM(principal_cents),0) FROM stored_value_records WHERE tenant_id=? AND status='CONFIRMED'),
	(SELECT COALESCE(SUM(gift_cents),0) FROM stored_value_records WHERE tenant_id=? AND status='CONFIRMED'),
	(SELECT COUNT(DISTINCT customer_id) FROM stored_value_records WHERE tenant_id=? AND status='CONFIRMED'),
	(SELECT COUNT(*) FROM stored_value_rules WHERE tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL)`

func (s *Server) getMemberSummary(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var summary memberSummary
	err := s.DB.QueryRowContext(r.Context(), memberSummaryQuery,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
		actor.TenantID,
	).Scan(
		&summary.CustomerCount,
		&summary.MemberCount,
		&summary.BalanceCents,
		&summary.BlockedCustomerCount,
		&summary.StoredValuePrincipalCents,
		&summary.StoredValueGiftCents,
		&summary.StoredValueCustomerCount,
		&summary.ActiveStoredValueRuleCount,
	)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, summary)
}

type customerInput struct {
	StoreID int64  `json:"source_store_id"`
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Avatar  string `json:"avatar_url"`
	OpenID  string `json:"wechat_openid"`
	UnionID string `json:"unionid"`
	Source  string `json:"source"`
	Status  string `json:"status"`
	Remark  string `json:"remark"`
}

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	where := " WHERE c.tenant_id=? AND c.deleted_at IS NULL"
	args := []any{actor.TenantID}
	if keyword := strings.TrimSpace(r.URL.Query().Get("keyword")); keyword != "" {
		where += " AND (c.name LIKE ? OR c.phone LIKE ? OR c.public_id LIKE ? OR m.member_no LIKE ?)"
		like := "%" + keyword + "%"
		args = append(args, like, like, like, like)
	}
	if status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status"))); status != "" {
		where += " AND c.status=?"
		args = append(args, status)
	}
	if levelID, err := optionalPositiveInt64(r.URL.Query().Get("level_id")); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_LEVEL", "level_id must be a positive integer")
		return
	} else if levelID > 0 {
		where += " AND m.current_level_id=?"
		args = append(args, levelID)
	}
	if tagID, err := optionalPositiveInt64(r.URL.Query().Get("tag_id")); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_TAG", "tag_id must be a positive integer")
		return
	} else if tagID > 0 {
		where += " AND EXISTS(SELECT 1 FROM customer_tag_assignments cta WHERE cta.tenant_id=c.tenant_id AND cta.customer_id=c.id AND cta.tag_id=?)"
		args = append(args, tagID)
	}
	base := ` FROM customers c
		LEFT JOIN stores st ON st.id=c.source_store_id AND st.tenant_id=c.tenant_id
		LEFT JOIN members m ON m.customer_id=c.id AND m.tenant_id=c.tenant_id
		LEFT JOIN member_levels ml ON ml.id=m.current_level_id AND ml.tenant_id=m.tenant_id`
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*)"+base+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT c.id,c.public_id,c.name,c.phone,c.avatar_url,c.source,c.status,c.remark,
		COALESCE(st.name,''),COALESCE(m.id,0),COALESCE(m.member_no,''),COALESCE(ml.id,0),COALESCE(ml.name,''),
		COALESCE(ba.principal_cents,0),COALESCE(ba.bonus_cents,0),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=c.tenant_id AND o.customer_id=c.id),
		(SELECT COALESCE(SUM(o.paid_cents-o.refunded_cents),0) FROM orders o WHERE o.tenant_id=c.tenant_id AND o.customer_id=c.id),
		DATE_FORMAT(c.registered_at,'%Y-%m-%d %H:%i:%s'),IF(c.last_seen_at IS NULL,NULL,DATE_FORMAT(c.last_seen_at,'%Y-%m-%d %H:%i:%s'))`+
		base+` LEFT JOIN balance_accounts ba ON ba.tenant_id=c.tenant_id AND ba.customer_id=c.id`+where+" ORDER BY c.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	itemIndexes := map[int64]int{}
	customerIDs := []int64{}
	for rows.Next() {
		var id, memberID, levelID, principal, bonus, spent int64
		var orderCount int
		var publicID, name, phone, avatar, source, status, remark, storeName, memberNo, levelName, registered string
		var lastSeen sql.NullString
		if err := rows.Scan(&id, &publicID, &name, &phone, &avatar, &source, &status, &remark, &storeName, &memberID, &memberNo, &levelID, &levelName, &principal, &bonus, &orderCount, &spent, &registered, &lastSeen); err != nil {
			handleSQLError(w, err)
			return
		}
		item := map[string]any{"id": id, "public_id": publicID, "name": name, "phone_masked": maskPhone(phone), "avatar_url": avatar, "source": source, "status": status, "remark": remark, "source_store_name": storeName, "member_id": memberID, "member_no": memberNo, "level_id": levelID, "level_name": levelName, "principal_cents": principal, "bonus_cents": bonus, "balance_cents": principal + bonus, "order_count": orderCount, "net_spent_cents": spent, "registered_at": registered, "tags": []map[string]any{}}
		if lastSeen.Valid {
			item["last_seen_at"] = lastSeen.String
		}
		itemIndexes[id] = len(items)
		customerIDs = append(customerIDs, id)
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		handleSQLError(w, err)
		return
	}
	if len(customerIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(customerIDs)), ",")
		tagArgs := []any{actor.TenantID}
		for _, id := range customerIDs {
			tagArgs = append(tagArgs, id)
		}
		tagRows, tagErr := s.DB.QueryContext(r.Context(), `SELECT a.customer_id,t.id,t.name,t.color FROM customer_tag_assignments a JOIN customer_tags t ON t.id=a.tag_id AND t.tenant_id=a.tenant_id WHERE a.tenant_id=? AND a.customer_id IN (`+placeholders+") AND t.deleted_at IS NULL ORDER BY t.name", tagArgs...)
		if tagErr != nil {
			handleSQLError(w, tagErr)
			return
		}
		for tagRows.Next() {
			var customerID, tagID int64
			var tagName, color string
			if scanErr := tagRows.Scan(&customerID, &tagID, &tagName, &color); scanErr != nil {
				tagRows.Close()
				handleSQLError(w, scanErr)
				return
			}
			index := itemIndexes[customerID]
			customerTags := items[index]["tags"].([]map[string]any)
			items[index]["tags"] = append(customerTags, map[string]any{"id": tagID, "name": tagName, "color": color})
		}
		tagRows.Close()
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) createCustomer(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input customerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !normalizeCustomerInput(w, &input) {
		return
	}
	if input.StoreID != 0 && !s.tenantOwnsStore(r.Context(), actor.TenantID, input.StoreID) {
		writeError(w, http.StatusBadRequest, "INVALID_STORE", "source store does not belong to this tenant")
		return
	}
	var openID, unionID any
	if input.OpenID != "" {
		openID = input.OpenID
	}
	if input.UnionID != "" {
		unionID = input.UnionID
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO customers(tenant_id,source_store_id,public_id,wechat_openid,unionid,name,avatar_url,phone,source,status,remark)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, actor.TenantID, nullableID(input.StoreID), newBusinessNo("CU"), openID, unionID, input.Name, input.Avatar, input.Phone, input.Source, input.Status, input.Remark)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), actor, "customer.create", "customer", int64String(id), map[string]any{"source": input.Source}, r)
	s.getCustomerByID(w, r, actor.TenantID, id)
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "customerID")
	if ok {
		s.getCustomerByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getCustomerByID(w http.ResponseWriter, r *http.Request, tenantID, customerID int64) {
	var id, memberID, levelID, principal, bonus, growth, spent, refunded int64
	var sourceStoreID sql.NullInt64
	var orderCount int
	var publicID, name, phone, avatar, source, status, remark, storeName, memberNo, memberStatus, levelName, registered string
	var joined, expires, lastSeen sql.NullString
	err := s.DB.QueryRowContext(r.Context(), `SELECT c.id,c.public_id,c.name,c.phone,c.avatar_url,c.source,c.status,c.remark,c.source_store_id,COALESCE(st.name,''),
		COALESCE(m.id,0),COALESCE(m.member_no,''),COALESCE(m.status,''),COALESCE(m.growth_value,0),COALESCE(ml.id,0),COALESCE(ml.name,''),
		COALESCE(ba.principal_cents,0),COALESCE(ba.bonus_cents,0),
		(SELECT COUNT(*) FROM orders o WHERE o.tenant_id=c.tenant_id AND o.customer_id=c.id),
		(SELECT COALESCE(SUM(o.paid_cents-o.refunded_cents),0) FROM orders o WHERE o.tenant_id=c.tenant_id AND o.customer_id=c.id),
		(SELECT COALESCE(SUM(o.refunded_cents),0) FROM orders o WHERE o.tenant_id=c.tenant_id AND o.customer_id=c.id),
		DATE_FORMAT(c.registered_at,'%Y-%m-%d %H:%i:%s'),IF(c.last_seen_at IS NULL,NULL,DATE_FORMAT(c.last_seen_at,'%Y-%m-%d %H:%i:%s')),
		IF(m.joined_at IS NULL,NULL,DATE_FORMAT(m.joined_at,'%Y-%m-%d %H:%i:%s')),IF(m.expires_at IS NULL,NULL,DATE_FORMAT(m.expires_at,'%Y-%m-%d %H:%i:%s'))
		FROM customers c LEFT JOIN stores st ON st.id=c.source_store_id AND st.tenant_id=c.tenant_id
		LEFT JOIN members m ON m.customer_id=c.id AND m.tenant_id=c.tenant_id LEFT JOIN member_levels ml ON ml.id=m.current_level_id AND ml.tenant_id=m.tenant_id
		LEFT JOIN balance_accounts ba ON ba.tenant_id=c.tenant_id AND ba.customer_id=c.id
		WHERE c.id=? AND c.tenant_id=? AND c.deleted_at IS NULL`, customerID, tenantID).
		Scan(&id, &publicID, &name, &phone, &avatar, &source, &status, &remark, &sourceStoreID, &storeName, &memberID, &memberNo, &memberStatus, &growth, &levelID, &levelName, &principal, &bonus, &orderCount, &spent, &refunded, &registered, &lastSeen, &joined, &expires)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	tagRows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.name,t.color FROM customer_tag_assignments a JOIN customer_tags t ON t.id=a.tag_id AND t.tenant_id=a.tenant_id WHERE a.tenant_id=? AND a.customer_id=? AND t.deleted_at IS NULL ORDER BY t.name`, tenantID, customerID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	tags := []map[string]any{}
	for tagRows.Next() {
		var tagID int64
		var tagName, color string
		if err := tagRows.Scan(&tagID, &tagName, &color); err != nil {
			tagRows.Close()
			handleSQLError(w, err)
			return
		}
		tags = append(tags, map[string]any{"id": tagID, "name": tagName, "color": color})
	}
	tagRows.Close()
	item := map[string]any{"id": id, "public_id": publicID, "name": name, "phone": phone, "phone_masked": maskPhone(phone), "avatar_url": avatar, "source": source, "status": status, "remark": remark, "source_store_name": storeName, "member_id": memberID, "member_no": memberNo, "member_status": memberStatus, "growth_value": growth, "level_id": levelID, "level_name": levelName, "principal_cents": principal, "bonus_cents": bonus, "balance_cents": principal + bonus, "order_count": orderCount, "net_spent_cents": spent, "refunded_cents": refunded, "registered_at": registered, "tags": tags}
	if sourceStoreID.Valid {
		item["source_store_id"] = sourceStoreID.Int64
	}
	if lastSeen.Valid {
		item["last_seen_at"] = lastSeen.String
	}
	if joined.Valid {
		item["joined_at"] = joined.String
	}
	if expires.Valid {
		item["expires_at"] = expires.String
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "customerID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input customerInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if !normalizeCustomerInput(w, &input) {
		return
	}
	var currentStore sql.NullInt64
	var currentAvatar string
	if err := s.DB.QueryRowContext(r.Context(), "SELECT source_store_id,avatar_url FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID).Scan(&currentStore, &currentAvatar); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.StoreID == 0 && currentStore.Valid {
		input.StoreID = currentStore.Int64
	}
	if input.Avatar == "" {
		input.Avatar = currentAvatar
	}
	if input.StoreID != 0 && !s.tenantOwnsStore(r.Context(), actor.TenantID, input.StoreID) {
		writeError(w, http.StatusBadRequest, "INVALID_STORE", "source store does not belong to this tenant")
		return
	}
	_, err := s.DB.ExecContext(r.Context(), `UPDATE customers SET source_store_id=?,name=?,avatar_url=?,phone=?,source=?,status=?,remark=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, nullableID(input.StoreID), input.Name, input.Avatar, input.Phone, input.Source, input.Status, input.Remark, id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "customer.update", "customer", int64String(id), map[string]any{"status": input.Status}, r)
	s.getCustomerByID(w, r, actor.TenantID, id)
}

func (s *Server) archiveCustomer(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "customerID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var found int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", id, actor.TenantID).Scan(&found); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO balance_accounts(tenant_id,customer_id) VALUES(?,?) ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)", actor.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	var principal, bonus int64
	if err = tx.QueryRowContext(r.Context(), "SELECT principal_cents,bonus_cents FROM balance_accounts WHERE tenant_id=? AND customer_id=? FOR UPDATE", actor.TenantID, id).Scan(&principal, &bonus); err != nil {
		handleSQLError(w, err)
		return
	}
	if principal != 0 || bonus != 0 {
		writeError(w, http.StatusConflict, "CUSTOMER_HAS_BALANCE", "customer balance must be zero before archival")
		return
	}
	result, err := tx.ExecContext(r.Context(), "UPDATE customers SET status='ARCHIVED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		handleSQLError(w, sql.ErrNoRows)
		return
	}
	if err = auditTx(r.Context(), tx, actor, "customer.archive", "customer", int64String(id), nil, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeCustomerInput(w http.ResponseWriter, input *customerInput) bool {
	input.Name = strings.TrimSpace(input.Name)
	input.Phone = strings.TrimSpace(input.Phone)
	input.Source = strings.ToUpper(strings.TrimSpace(input.Source))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return false
	}
	if input.Source == "" {
		input.Source = "MANUAL"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if !validStatus(input.Status, "ACTIVE", "BLOCKED", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE, BLOCKED or DISABLED")
		return false
	}
	return true
}

type customerTagInput struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

func (s *Server) listCustomerTags(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	rows, err := s.DB.QueryContext(r.Context(), `SELECT t.id,t.name,t.color,t.description,t.status,COUNT(a.customer_id),DATE_FORMAT(t.created_at,'%Y-%m-%d %H:%i:%s') FROM customer_tags t LEFT JOIN customer_tag_assignments a ON a.tenant_id=t.tenant_id AND a.tag_id=t.id WHERE t.tenant_id=? AND t.deleted_at IS NULL GROUP BY t.id ORDER BY t.id DESC`, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var count int
		var name, color, description, status, created string
		if err := rows.Scan(&id, &name, &color, &description, &status, &count, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "color": color, "description": description, "status": status, "customer_count": count, "created_at": created})
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createCustomerTag(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input customerTagInput
	if !decodeJSON(w, r, &input) || !normalizeTagInput(w, &input) {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "INSERT INTO customer_tags(tenant_id,name,color,description,status) VALUES(?,?,?,?,?)", actor.TenantID, input.Name, input.Color, input.Description, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), actor, "customer_tag.create", "customer_tag", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusCreated, map[string]any{"id": id, "name": input.Name, "color": input.Color, "description": input.Description, "status": input.Status})
}

func (s *Server) updateCustomerTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tagID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input customerTagInput
	if !decodeJSON(w, r, &input) || !normalizeTagInput(w, &input) {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE customer_tags SET name=?,color=?,description=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Name, input.Color, input.Description, input.Status, id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		handleSQLError(w, sql.ErrNoRows)
		return
	}
	s.audit(r.Context(), actor, "customer_tag.update", "customer_tag", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusOK, map[string]any{"id": id, "name": input.Name, "color": input.Color, "description": input.Description, "status": input.Status})
}

func (s *Server) deleteCustomerTag(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tagID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM customer_tag_assignments WHERE tenant_id=? AND tag_id=?", actor.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), "UPDATE customer_tags SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		handleSQLError(w, sql.ErrNoRows)
		return
	}
	if err = auditTx(r.Context(), tx, actor, "customer_tag.delete", "customer_tag", int64String(id), nil, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func normalizeTagInput(w http.ResponseWriter, input *customerTagInput) bool {
	input.Name = strings.TrimSpace(input.Name)
	input.Color = strings.TrimSpace(input.Color)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return false
	}
	if input.Color == "" {
		input.Color = "blue"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
		return false
	}
	return true
}

type replaceTagsInput struct {
	TagIDs []int64 `json:"tag_ids"`
}

func (s *Server) replaceCustomerTags(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "customerID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input replaceTagsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	seen := map[int64]bool{}
	for _, tagID := range input.TagIDs {
		if tagID <= 0 || seen[tagID] {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "tag_ids must contain unique positive integers")
			return
		}
		seen[tagID] = true
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var found int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", id, actor.TenantID).Scan(&found); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM customer_tag_assignments WHERE tenant_id=? AND customer_id=?", actor.TenantID, id); err != nil {
		handleSQLError(w, err)
		return
	}
	for _, tagID := range input.TagIDs {
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO customer_tag_assignments(tenant_id,customer_id,tag_id,assigned_by)
			SELECT ?,?,?,? FROM customer_tags WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL`, actor.TenantID, id, tagID, actor.UserID, tagID, actor.TenantID)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		if n, _ := result.RowsAffected(); n == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_TAG", "a tag does not belong to this tenant")
			return
		}
	}
	if err = auditTx(r.Context(), tx, actor, "customer.tags.replace", "customer", int64String(id), map[string]any{"tag_ids": input.TagIDs}, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.getCustomerByID(w, r, actor.TenantID, id)
}

type memberLevelInput struct {
	Name            string `json:"name"`
	RankNo          int    `json:"rank_no"`
	AcquireType     string `json:"acquire_type"`
	GrowthThreshold int64  `json:"growth_threshold"`
	PriceCents      int64  `json:"price_cents"`
	ValidDays       int    `json:"valid_days"`
	Benefits        any    `json:"benefits"`
	UpgradeGift     any    `json:"upgrade_gift"`
	IsDefault       bool   `json:"is_default"`
	Status          string `json:"status"`
}

func (s *Server) listMemberLevels(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	rows, err := s.DB.QueryContext(r.Context(), `SELECT l.id,l.name,l.rank_no,l.acquire_type,l.growth_threshold,l.price_cents,l.valid_days,l.benefits_json,l.upgrade_gift_json,l.is_default,l.status,COUNT(m.id),DATE_FORMAT(l.created_at,'%Y-%m-%d %H:%i:%s') FROM member_levels l LEFT JOIN members m ON m.tenant_id=l.tenant_id AND m.current_level_id=l.id WHERE l.tenant_id=? AND l.deleted_at IS NULL GROUP BY l.id ORDER BY l.rank_no,l.id`, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, growth, price int64
		var rank, days, count int
		var name, acquire, benefits, gift, status, created string
		var isDefault bool
		if err := rows.Scan(&id, &name, &rank, &acquire, &growth, &price, &days, &benefits, &gift, &isDefault, &status, &count, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "name": name, "rank_no": rank, "acquire_type": acquire, "growth_threshold": growth, "price_cents": price, "valid_days": days, "benefits": jsonValue(benefits), "upgrade_gift": jsonValue(gift), "is_default": isDefault, "status": status, "member_count": count, "created_at": created})
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createMemberLevel(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input memberLevelInput
	if !decodeJSON(w, r, &input) || !normalizeLevelInput(w, &input) {
		return
	}
	id, err := s.saveMemberLevel(r.Context(), actor, 0, input)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "member_level.create", "member_level", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusCreated, map[string]any{"id": id})
}
func (s *Server) updateMemberLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "levelID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input memberLevelInput
	if !decodeJSON(w, r, &input) || !normalizeLevelInput(w, &input) {
		return
	}
	if _, err := s.saveMemberLevel(r.Context(), actor, id, input); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "member_level.update", "member_level", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusOK, map[string]any{"id": id})
}

func (s *Server) saveMemberLevel(ctx context.Context, actor identity, id int64, input memberLevelInput) (int64, error) {
	benefits, _ := json.Marshal(valueOrObject(input.Benefits))
	gift, _ := json.Marshal(valueOrObject(input.UpgradeGift))
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if input.IsDefault {
		if _, err = tx.ExecContext(ctx, "UPDATE member_levels SET is_default=0 WHERE tenant_id=?", actor.TenantID); err != nil {
			return 0, err
		}
	}
	if id == 0 {
		result, insertErr := tx.ExecContext(ctx, "INSERT INTO member_levels(tenant_id,name,rank_no,acquire_type,growth_threshold,price_cents,valid_days,benefits_json,upgrade_gift_json,is_default,status) VALUES(?,?,?,?,?,?,?,?,?,?,?)", actor.TenantID, input.Name, input.RankNo, input.AcquireType, input.GrowthThreshold, input.PriceCents, input.ValidDays, string(benefits), string(gift), input.IsDefault, input.Status)
		if insertErr != nil {
			return 0, insertErr
		}
		id, _ = result.LastInsertId()
	} else {
		result, updateErr := tx.ExecContext(ctx, "UPDATE member_levels SET name=?,rank_no=?,acquire_type=?,growth_threshold=?,price_cents=?,valid_days=?,benefits_json=?,upgrade_gift_json=?,is_default=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Name, input.RankNo, input.AcquireType, input.GrowthThreshold, input.PriceCents, input.ValidDays, string(benefits), string(gift), input.IsDefault, input.Status, id, actor.TenantID)
		if updateErr != nil {
			return 0, updateErr
		}
		if n, _ := result.RowsAffected(); n == 0 {
			return 0, sql.ErrNoRows
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func normalizeLevelInput(w http.ResponseWriter, input *memberLevelInput) bool {
	input.Name = strings.TrimSpace(input.Name)
	input.AcquireType = strings.ToUpper(strings.TrimSpace(input.AcquireType))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required")
		return false
	}
	if input.AcquireType == "" {
		input.AcquireType = "GROWTH"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if !validStatus(input.AcquireType, "FREE", "GROWTH", "PAID") || !validStatus(input.Status, "ACTIVE", "DISABLED") || input.GrowthThreshold < 0 || input.PriceCents < 0 || input.ValidDays < 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid level configuration")
		return false
	}
	return true
}

func (s *Server) deleteMemberLevel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "levelID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var used int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM members WHERE tenant_id=? AND current_level_id=?", actor.TenantID, id).Scan(&used); err != nil {
		handleSQLError(w, err)
		return
	}
	if used > 0 {
		writeError(w, http.StatusConflict, "LEVEL_IN_USE", "a member level in use can only be disabled")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE member_levels SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		handleSQLError(w, sql.ErrNoRows)
		return
	}
	s.audit(r.Context(), actor, "member_level.delete", "member_level", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

type membershipSettingsInput struct {
	Enabled        bool   `json:"enabled"`
	CardName       string `json:"card_name"`
	CardColor      string `json:"card_color"`
	CardImageURL   string `json:"card_image_url"`
	AutoEnroll     bool   `json:"auto_enroll"`
	DefaultLevelID int64  `json:"default_level_id"`
	GrowthPerYuan  int    `json:"growth_per_yuan"`
	AgreementURL   string `json:"agreement_url"`
	ShowBalance    bool   `json:"show_balance"`
}

func (s *Server) getMembershipSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input membershipSettingsInput
	var defaultID sql.NullInt64
	err := s.DB.QueryRowContext(r.Context(), "SELECT enabled,card_name,card_color,card_image_url,auto_enroll,default_level_id,growth_per_yuan,agreement_url,show_balance FROM membership_settings WHERE tenant_id=?", actor.TenantID).Scan(&input.Enabled, &input.CardName, &input.CardColor, &input.CardImageURL, &input.AutoEnroll, &defaultID, &input.GrowthPerYuan, &input.AgreementURL, &input.ShowBalance)
	if errors.Is(err, sql.ErrNoRows) {
		input = membershipSettingsInput{Enabled: true, CardName: "会员卡", CardColor: "#8b5635", AutoEnroll: true, GrowthPerYuan: 1, ShowBalance: true}
	} else if err != nil {
		handleSQLError(w, err)
		return
	}
	if defaultID.Valid {
		input.DefaultLevelID = defaultID.Int64
	}
	writeData(w, http.StatusOK, input)
}
func (s *Server) updateMembershipSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input membershipSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.CardName = strings.TrimSpace(input.CardName)
	if input.CardName == "" || input.GrowthPerYuan < 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "card_name is required and growth_per_yuan cannot be negative")
		return
	}
	if input.DefaultLevelID > 0 && !s.tenantOwnsLevel(r.Context(), actor.TenantID, input.DefaultLevelID) {
		writeError(w, http.StatusBadRequest, "INVALID_LEVEL", "default level does not belong to this tenant")
		return
	}
	_, err := s.DB.ExecContext(r.Context(), `INSERT INTO membership_settings(tenant_id,enabled,card_name,card_color,card_image_url,auto_enroll,default_level_id,growth_per_yuan,agreement_url,show_balance,updated_by) VALUES(?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE enabled=VALUES(enabled),card_name=VALUES(card_name),card_color=VALUES(card_color),card_image_url=VALUES(card_image_url),auto_enroll=VALUES(auto_enroll),default_level_id=VALUES(default_level_id),growth_per_yuan=VALUES(growth_per_yuan),agreement_url=VALUES(agreement_url),show_balance=VALUES(show_balance),updated_by=VALUES(updated_by)`, actor.TenantID, input.Enabled, input.CardName, input.CardColor, input.CardImageURL, input.AutoEnroll, nullableID(input.DefaultLevelID), input.GrowthPerYuan, input.AgreementURL, input.ShowBalance, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "membership_settings.update", "settings", "membership", map[string]any{"enabled": input.Enabled, "default_level_id": input.DefaultLevelID}, r)
	writeData(w, http.StatusOK, input)
}

type issueMemberInput struct {
	CustomerID  int64  `json:"customer_id"`
	LevelID     int64  `json:"level_id"`
	IssueSource string `json:"issue_source"`
	Remark      string `json:"remark"`
}

func (s *Server) issueMemberCard(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input issueMemberInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.CustomerID <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "customer_id is required")
		return
	}
	input.IssueSource = strings.ToUpper(strings.TrimSpace(input.IssueSource))
	if input.IssueSource == "" {
		input.IssueSource = "MANUAL"
	}
	fingerprint := requestFingerprint(input)
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var existing int64
	var existingFingerprint string
	if err = tx.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM member_card_issuances WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); err == nil {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different card issuance")
			return
		}
		tx.Rollback()
		writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	var customer int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", input.CustomerID, actor.TenantID).Scan(&customer); err != nil {
		handleSQLError(w, err)
		return
	}
	levelSnapshot := "{}"
	validDays := 0
	if input.LevelID > 0 {
		var levelName string
		if err = tx.QueryRowContext(r.Context(), "SELECT name,valid_days FROM member_levels WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL", input.LevelID, actor.TenantID).Scan(&levelName, &validDays); err != nil {
			handleSQLError(w, err)
			return
		}
		body, _ := json.Marshal(map[string]any{"id": input.LevelID, "name": levelName, "valid_days": validDays})
		levelSnapshot = string(body)
	}
	var memberID int64
	err = tx.QueryRowContext(r.Context(), "SELECT id FROM members WHERE tenant_id=? AND customer_id=? FOR UPDATE", actor.TenantID, input.CustomerID).Scan(&memberID)
	var expires any
	if validDays > 0 {
		expires = time.Now().AddDate(0, 0, validDays)
	}
	if errors.Is(err, sql.ErrNoRows) {
		result, insertErr := tx.ExecContext(r.Context(), "INSERT INTO members(tenant_id,customer_id,member_no,current_level_id,expires_at) VALUES(?,?,?,?,?)", actor.TenantID, input.CustomerID, newBusinessNo("MB"), nullableID(input.LevelID), expires)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		memberID, _ = result.LastInsertId()
	} else if err != nil {
		handleSQLError(w, err)
		return
	} else {
		if _, err = tx.ExecContext(r.Context(), "UPDATE members SET current_level_id=?,status='ACTIVE',expires_at=? WHERE id=? AND tenant_id=?", nullableID(input.LevelID), expires, memberID, actor.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	result, err := tx.ExecContext(r.Context(), "INSERT INTO member_card_issuances(tenant_id,issue_no,customer_id,member_id,level_id,issue_source,idempotency_key,request_fingerprint,level_snapshot_json,valid_to,created_by) VALUES(?,?,?,?,?,?,?,?,?,?,?)", actor.TenantID, newBusinessNo("MC"), input.CustomerID, memberID, nullableID(input.LevelID), input.IssueSource, key, fingerprint, levelSnapshot, expires, actor.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if loadErr := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM member_card_issuances WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); loadErr == nil {
				if existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different card issuance")
					return
				}
				writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	issueID, _ := result.LastInsertId()
	if err = auditTx(r.Context(), tx, actor, "member_card.issue", "member_card", int64String(issueID), map[string]any{"customer_id": input.CustomerID, "level_id": input.LevelID}, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"id": issueID, "member_id": memberID})
}

func (s *Server) listMemberCardIssuances(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	where := " WHERE i.tenant_id=?"
	args := []any{actor.TenantID}
	if id, err := optionalPositiveInt64(r.URL.Query().Get("customer_id")); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CUSTOMER", "customer_id must be positive")
		return
	} else if id > 0 {
		where += " AND i.customer_id=?"
		args = append(args, id)
	}
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM member_card_issuances i"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT i.id,i.issue_no,i.customer_id,c.name,m.member_no,COALESCE(l.name,''),i.issue_source,i.status,DATE_FORMAT(i.valid_from,'%Y-%m-%d %H:%i:%s'),IF(i.valid_to IS NULL,NULL,DATE_FORMAT(i.valid_to,'%Y-%m-%d %H:%i:%s')),DATE_FORMAT(i.created_at,'%Y-%m-%d %H:%i:%s') FROM member_card_issuances i JOIN customers c ON c.id=i.customer_id AND c.tenant_id=i.tenant_id JOIN members m ON m.id=i.member_id AND m.tenant_id=i.tenant_id LEFT JOIN member_levels l ON l.id=i.level_id AND l.tenant_id=i.tenant_id`+where+" ORDER BY i.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, customerID int64
		var issueNo, name, memberNo, levelName, source, status, validFrom, created string
		var validTo sql.NullString
		if err := rows.Scan(&id, &issueNo, &customerID, &name, &memberNo, &levelName, &source, &status, &validFrom, &validTo, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		item := map[string]any{"id": id, "issue_no": issueNo, "customer_id": customerID, "customer_name": name, "member_no": memberNo, "level_name": levelName, "issue_source": source, "status": status, "valid_from": validFrom, "created_at": created}
		if validTo.Valid {
			item["valid_to"] = validTo.String
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

type levelOrderInput struct {
	CustomerID    int64    `json:"customer_id"`
	LevelID       int64    `json:"level_id"`
	AmountCents   int64    `json:"amount_cents"`
	LegacyAmount  *float64 `json:"amount"`
	PaymentMethod string   `json:"payment_method"`
	Status        string   `json:"status"`
	Remark        string   `json:"remark"`
}

func applyLegacyLevelOrderAmount(input *levelOrderInput) error {
	if input.LegacyAmount == nil {
		return nil
	}
	amount := *input.LegacyAmount
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > float64(maxBusinessAmountCents)/100 {
		return errors.New("amount must be a supported non-negative value")
	}
	legacyCents := int64(math.Round(amount * 100))
	if input.AmountCents != 0 && input.AmountCents != legacyCents {
		return errors.New("amount and amount_cents do not match")
	}
	input.AmountCents = legacyCents
	input.LegacyAmount = nil
	return nil
}

func (s *Server) createMemberLevelOrder(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input levelOrderInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := applyLegacyLevelOrderAmount(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if input.CustomerID <= 0 || input.LevelID <= 0 || input.AmountCents < 0 || input.AmountCents > maxBusinessAmountCents {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "customer_id, level_id and non-negative amount_cents are required")
		return
	}
	input.PaymentMethod = strings.ToUpper(strings.TrimSpace(input.PaymentMethod))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.PaymentMethod == "" {
		input.PaymentMethod = "MANUAL"
	}
	if input.Status == "" {
		input.Status = "COMPLETED"
	}
	if !validStatus(input.Status, "RECORDED", "COMPLETED", "CANCELLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid status")
		return
	}
	fingerprint := requestFingerprint(input)
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var existing int64
	var existingFingerprint string
	if err = tx.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM member_level_orders WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); err == nil {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different level order")
			return
		}
		tx.Rollback()
		writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	var customer int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.CustomerID, actor.TenantID).Scan(&customer); err != nil {
		handleSQLError(w, err)
		return
	}
	var levelName string
	var configuredPrice int64
	if err = tx.QueryRowContext(r.Context(), "SELECT name,price_cents FROM member_levels WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.LevelID, actor.TenantID).Scan(&levelName, &configuredPrice); err != nil {
		handleSQLError(w, err)
		return
	}
	snapshot, _ := json.Marshal(map[string]any{"id": input.LevelID, "name": levelName, "configured_price_cents": configuredPrice})
	var memberID sql.NullInt64
	_ = tx.QueryRowContext(r.Context(), "SELECT id FROM members WHERE tenant_id=? AND customer_id=?", actor.TenantID, input.CustomerID).Scan(&memberID)
	completed := any(nil)
	if input.Status == "COMPLETED" {
		completed = time.Now()
	}
	result, err := tx.ExecContext(r.Context(), "INSERT INTO member_level_orders(tenant_id,order_no,customer_id,member_id,level_id,level_snapshot_json,amount_cents,payment_method,payment_status,status,remark,idempotency_key,request_fingerprint,created_by,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", actor.TenantID, newBusinessNo("ML"), input.CustomerID, nullableNullInt64(memberID), input.LevelID, string(snapshot), input.AmountCents, input.PaymentMethod, "RECORDED", input.Status, input.Remark, key, fingerprint, actor.UserID, completed)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			_ = tx.Rollback()
			if loadErr := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM member_level_orders WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); loadErr == nil {
				if existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different level order")
					return
				}
				writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if err = auditTx(r.Context(), tx, actor, "member_level_order.create", "member_level_order", int64String(id), map[string]any{"customer_id": input.CustomerID, "level_id": input.LevelID, "amount_cents": input.AmountCents}, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) listMemberLevelOrders(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM member_level_orders WHERE tenant_id=?", actor.TenantID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT o.id,o.order_no,o.customer_id,c.name,l.name,o.amount_cents,o.payment_method,o.payment_status,o.status,o.remark,DATE_FORMAT(o.created_at,'%Y-%m-%d %H:%i:%s') FROM member_level_orders o JOIN customers c ON c.id=o.customer_id AND c.tenant_id=o.tenant_id JOIN member_levels l ON l.id=o.level_id AND l.tenant_id=o.tenant_id WHERE o.tenant_id=? ORDER BY o.id DESC LIMIT ? OFFSET ?`, actor.TenantID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, customerID, amount int64
		var orderNo, customerName, levelName, method, paymentStatus, status, remark, created string
		if err := rows.Scan(&id, &orderNo, &customerID, &customerName, &levelName, &amount, &method, &paymentStatus, &status, &remark, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "order_no": orderNo, "customer_id": customerID, "customer_name": customerName, "level_name": levelName, "amount_cents": amount, "payment_method": method, "payment_status": paymentStatus, "status": status, "remark": remark, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

type balanceAdjustmentInput struct {
	Bucket      string `json:"bucket"`
	Direction   string `json:"direction"`
	AmountCents int64  `json:"amount_cents"`
	Remark      string `json:"remark"`
}
type balanceLedgerEntry struct {
	ID           int64  `json:"id"`
	CustomerID   int64  `json:"customer_id"`
	Bucket       string `json:"bucket"`
	DeltaCents   int64  `json:"delta_cents"`
	BeforeCents  int64  `json:"balance_before_cents"`
	AfterCents   int64  `json:"balance_after_cents"`
	EntryType    string `json:"entry_type"`
	BusinessType string `json:"business_type"`
	BusinessNo   string `json:"business_no"`
	Remark       string `json:"remark"`
}

func (s *Server) adjustCustomerBalance(w http.ResponseWriter, r *http.Request) {
	customerID, ok := pathID(w, r, "customerID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input balanceAdjustmentInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Bucket = strings.ToUpper(strings.TrimSpace(input.Bucket))
	input.Direction = strings.ToUpper(strings.TrimSpace(input.Direction))
	input.Remark = strings.TrimSpace(input.Remark)
	if !validStatus(input.Bucket, "PRINCIPAL", "BONUS") || !validStatus(input.Direction, "CREDIT", "DEBIT") || input.AmountCents <= 0 || input.AmountCents > maxBusinessAmountCents || input.Remark == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "bucket, direction, positive amount_cents and remark are required")
		return
	}
	delta := input.AmountCents
	if input.Direction == "DEBIT" {
		delta = -delta
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	entry, replayed, err := applyBalanceDeltaTx(r.Context(), tx, actor.TenantID, customerID, input.Bucket, delta, "ADJUSTMENT", "MANUAL_ADJUSTMENT", newBusinessNo("BA"), key, actor.UserID, input.Remark)
	if errors.Is(err, errInsufficientBalance) {
		writeError(w, http.StatusConflict, "INSUFFICIENT_BALANCE", "balance cannot become negative")
		return
	} else if errors.Is(err, errIdempotencyKeyReused) {
		writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different balance adjustment")
		return
	} else if errors.Is(err, errBalanceLimitExceeded) {
		writeError(w, http.StatusConflict, "BALANCE_LIMIT_EXCEEDED", "the resulting customer balance would exceed the configured limit")
		return
	} else if err != nil {
		handleSQLError(w, err)
		return
	}
	if !replayed {
		if err = auditTx(r.Context(), tx, actor, "balance.adjust", "customer", int64String(customerID), map[string]any{"ledger_id": entry.ID, "bucket": input.Bucket, "delta_cents": delta, "remark": input.Remark}, r); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, mapStatus(replayed, http.StatusCreated, http.StatusOK), map[string]any{"entry": entry, "idempotent_replay": replayed})
}

func (s *Server) listBalanceLedger(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	where := " WHERE l.tenant_id=?"
	args := []any{actor.TenantID}
	if id, err := optionalPositiveInt64(r.URL.Query().Get("customer_id")); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CUSTOMER", "customer_id must be positive")
		return
	} else if id > 0 {
		where += " AND l.customer_id=?"
		args = append(args, id)
	}
	if bucket := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("bucket"))); bucket != "" {
		where += " AND l.account_bucket=?"
		args = append(args, bucket)
	}
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM balance_ledger l"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT l.id,l.customer_id,c.name,l.account_bucket,l.delta_cents,l.balance_before_cents,l.balance_after_cents,l.entry_type,l.business_type,l.business_no,l.remark,l.operator_user_id,DATE_FORMAT(l.created_at,'%Y-%m-%d %H:%i:%s') FROM balance_ledger l JOIN customers c ON c.id=l.customer_id AND c.tenant_id=l.tenant_id`+where+" ORDER BY l.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, customerID, delta, before, after, operator int64
		var customerName, bucket, entryType, businessType, businessNo, remark, created string
		if err := rows.Scan(&id, &customerID, &customerName, &bucket, &delta, &before, &after, &entryType, &businessType, &businessNo, &remark, &operator, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "customer_id": customerID, "customer_name": customerName, "bucket": bucket, "delta_cents": delta, "balance_before_cents": before, "balance_after_cents": after, "entry_type": entryType, "business_type": businessType, "business_no": businessNo, "remark": remark, "operator_user_id": operator, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

var (
	errInsufficientBalance  = errors.New("insufficient balance")
	errBalanceLimitExceeded = errors.New("balance limit exceeded")
	errIdempotencyKeyReused = errors.New("idempotency key reused with a different request")
)

func applyBalanceDeltaTx(ctx context.Context, tx *sql.Tx, tenantID, customerID int64, bucket string, delta int64, entryType, businessType, businessNo, idempotencyKey string, operatorID int64, remark string) (balanceLedgerEntry, bool, error) {
	existing, found, err := loadBalanceLedgerByKey(ctx, tx, tenantID, idempotencyKey)
	if err != nil {
		return balanceLedgerEntry{}, false, err
	}
	if found {
		return compareBalanceReplay(existing, customerID, bucket, delta, entryType, businessType, remark)
	}
	var lockedCustomerID int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", customerID, tenantID).Scan(&lockedCustomerID); err != nil {
		return balanceLedgerEntry{}, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO balance_accounts(tenant_id,customer_id) SELECT ?,id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)`, tenantID, customerID, tenantID)
	if err != nil {
		return balanceLedgerEntry{}, false, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		var found int
		if findErr := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL", customerID, tenantID).Scan(&found); findErr != nil {
			return balanceLedgerEntry{}, false, findErr
		}
		if found == 0 {
			return balanceLedgerEntry{}, false, sql.ErrNoRows
		}
	}
	var principal, bonus int64
	if err = tx.QueryRowContext(ctx, "SELECT principal_cents,bonus_cents FROM balance_accounts WHERE tenant_id=? AND customer_id=? FOR UPDATE", tenantID, customerID).Scan(&principal, &bonus); err != nil {
		return balanceLedgerEntry{}, false, err
	}
	before := principal
	if bucket == "BONUS" {
		before = bonus
	}
	limit := int64(1000000)
	if settingsErr := tx.QueryRowContext(ctx, "SELECT max_balance_cents FROM stored_value_settings WHERE tenant_id=?", tenantID).Scan(&limit); settingsErr != nil && !errors.Is(settingsErr, sql.ErrNoRows) {
		return balanceLedgerEntry{}, false, settingsErr
	}
	if limit <= 0 || limit > maxBusinessAmountCents || principal < 0 || bonus < 0 || !nonNegativeSumWithin(limit, principal, bonus) || delta > maxBusinessAmountCents || delta < -maxBusinessAmountCents {
		return balanceLedgerEntry{}, false, errBalanceLimitExceeded
	}
	if delta < 0 && delta < -before {
		return balanceLedgerEntry{}, false, errInsufficientBalance
	}
	if delta > 0 && !nonNegativeSumWithin(limit, principal, bonus, delta) {
		return balanceLedgerEntry{}, false, errBalanceLimitExceeded
	}
	after := before + delta
	column := "principal_cents"
	if bucket == "BONUS" {
		column = "bonus_cents"
	}
	if bucket != "PRINCIPAL" && bucket != "BONUS" {
		return balanceLedgerEntry{}, false, fmt.Errorf("invalid balance bucket %q", bucket)
	}
	result, err = tx.ExecContext(ctx, "INSERT INTO balance_ledger(tenant_id,customer_id,account_bucket,delta_cents,balance_before_cents,balance_after_cents,entry_type,business_type,business_no,idempotency_key,operator_user_id,remark) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", tenantID, customerID, bucket, delta, before, after, entryType, businessType, businessNo, idempotencyKey, operatorID, remark)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			existing, found, loadErr := loadBalanceLedgerByKey(ctx, tx, tenantID, idempotencyKey)
			if loadErr != nil {
				return balanceLedgerEntry{}, false, loadErr
			}
			if found {
				return compareBalanceReplay(existing, customerID, bucket, delta, entryType, businessType, remark)
			}
		}
		return balanceLedgerEntry{}, false, err
	}
	ledgerID, _ := result.LastInsertId()
	if _, err = tx.ExecContext(ctx, "UPDATE balance_accounts SET "+column+"=?,version=version+1 WHERE tenant_id=? AND customer_id=?", after, tenantID, customerID); err != nil {
		return balanceLedgerEntry{}, false, err
	}
	return balanceLedgerEntry{ID: ledgerID, CustomerID: customerID, Bucket: bucket, DeltaCents: delta, BeforeCents: before, AfterCents: after, EntryType: entryType, BusinessType: businessType, BusinessNo: businessNo, Remark: remark}, false, nil
}

func loadBalanceLedgerByKey(ctx context.Context, tx *sql.Tx, tenantID int64, key string) (balanceLedgerEntry, bool, error) {
	var entry balanceLedgerEntry
	err := tx.QueryRowContext(ctx, `SELECT id,customer_id,account_bucket,delta_cents,balance_before_cents,balance_after_cents,entry_type,business_type,business_no,remark FROM balance_ledger WHERE tenant_id=? AND idempotency_key=?`, tenantID, key).Scan(&entry.ID, &entry.CustomerID, &entry.Bucket, &entry.DeltaCents, &entry.BeforeCents, &entry.AfterCents, &entry.EntryType, &entry.BusinessType, &entry.BusinessNo, &entry.Remark)
	if errors.Is(err, sql.ErrNoRows) {
		return balanceLedgerEntry{}, false, nil
	}
	return entry, err == nil, err
}

func compareBalanceReplay(existing balanceLedgerEntry, customerID int64, bucket string, delta int64, entryType, businessType, remark string) (balanceLedgerEntry, bool, error) {
	if existing.CustomerID != customerID || existing.Bucket != bucket || existing.DeltaCents != delta || existing.EntryType != entryType || existing.BusinessType != businessType || existing.Remark != remark {
		return balanceLedgerEntry{}, false, errIdempotencyKeyReused
	}
	return existing, true, nil
}

type storedValueRuleInput struct {
	Name             string  `json:"name"`
	RechargeCents    int64   `json:"recharge_cents"`
	GiftCents        int64   `json:"gift_cents"`
	GiftGrowth       int64   `json:"gift_growth"`
	Benefits         any     `json:"benefits"`
	PerCustomerLimit int     `json:"per_customer_limit"`
	StartsAt         *string `json:"starts_at"`
	EndsAt           *string `json:"ends_at"`
	Status           string  `json:"status"`
}

func (s *Server) listStoredValueRules(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,recharge_cents,gift_cents,gift_growth,benefits_json,per_customer_limit,IF(starts_at IS NULL,NULL,DATE_FORMAT(starts_at,'%Y-%m-%d %H:%i:%s')),IF(ends_at IS NULL,NULL,DATE_FORMAT(ends_at,'%Y-%m-%d %H:%i:%s')),status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM stored_value_rules WHERE tenant_id=? AND deleted_at IS NULL ORDER BY recharge_cents,id`, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, recharge, gift, growth int64
		var limit int
		var name, benefits, status, created string
		var starts, ends sql.NullString
		if err := rows.Scan(&id, &name, &recharge, &gift, &growth, &benefits, &limit, &starts, &ends, &status, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		item := map[string]any{"id": id, "name": name, "recharge_cents": recharge, "gift_cents": gift, "gift_growth": growth, "benefits": jsonValue(benefits), "per_customer_limit": limit, "status": status, "created_at": created}
		if starts.Valid {
			item["starts_at"] = starts.String
		}
		if ends.Valid {
			item["ends_at"] = ends.String
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}
func (s *Server) createStoredValueRule(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input storedValueRuleInput
	if !decodeJSON(w, r, &input) || !normalizeStoredRule(w, &input) || !validateStoredRuleTimes(w, input) {
		return
	}
	id, err := s.saveStoredValueRule(r.Context(), actor, 0, input)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "stored_value_rule.create", "stored_value_rule", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusCreated, map[string]any{"id": id})
}
func (s *Server) updateStoredValueRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ruleID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input storedValueRuleInput
	if !decodeJSON(w, r, &input) || !normalizeStoredRule(w, &input) || !validateStoredRuleTimes(w, input) {
		return
	}
	if _, err := s.saveStoredValueRule(r.Context(), actor, id, input); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "stored_value_rule.update", "stored_value_rule", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusOK, map[string]any{"id": id})
}
func (s *Server) saveStoredValueRule(ctx context.Context, actor identity, id int64, input storedValueRuleInput) (int64, error) {
	benefits, _ := json.Marshal(valueOrObject(input.Benefits))
	starts, err := optionalTime(input.StartsAt)
	if err != nil {
		return 0, err
	}
	ends, err := optionalTime(input.EndsAt)
	if err != nil {
		return 0, err
	}
	if starts != nil && ends != nil && !ends.After(*starts) {
		return 0, fmt.Errorf("ends_at must be after starts_at")
	}
	if id == 0 {
		result, insertErr := s.DB.ExecContext(ctx, "INSERT INTO stored_value_rules(tenant_id,name,recharge_cents,gift_cents,gift_growth,benefits_json,per_customer_limit,starts_at,ends_at,status) VALUES(?,?,?,?,?,?,?,?,?,?)", actor.TenantID, input.Name, input.RechargeCents, input.GiftCents, input.GiftGrowth, string(benefits), input.PerCustomerLimit, starts, ends, input.Status)
		if insertErr != nil {
			return 0, insertErr
		}
		id, _ = result.LastInsertId()
		return id, nil
	}
	result, updateErr := s.DB.ExecContext(ctx, "UPDATE stored_value_rules SET name=?,recharge_cents=?,gift_cents=?,gift_growth=?,benefits_json=?,per_customer_limit=?,starts_at=?,ends_at=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.Name, input.RechargeCents, input.GiftCents, input.GiftGrowth, string(benefits), input.PerCustomerLimit, starts, ends, input.Status, id, actor.TenantID)
	if updateErr != nil {
		return 0, updateErr
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return 0, sql.ErrNoRows
	}
	return id, nil
}
func normalizeStoredRule(w http.ResponseWriter, input *storedValueRuleInput) bool {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || input.RechargeCents <= 0 || input.GiftCents < 0 || !nonNegativeSumWithin(maxBusinessAmountCents, input.RechargeCents, input.GiftCents) || input.GiftGrowth < 0 || input.PerCustomerLimit < 0 || !validStatus(input.Status, "ACTIVE", "DISABLED") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid stored-value rule")
		return false
	}
	return true
}

func validateStoredRuleTimes(w http.ResponseWriter, input storedValueRuleInput) bool {
	starts, err := optionalTime(input.StartsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return false
	}
	ends, err := optionalTime(input.EndsAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return false
	}
	if starts != nil && ends != nil && !ends.After(*starts) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ends_at must be after starts_at")
		return false
	}
	return true
}
func (s *Server) deleteStoredValueRule(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ruleID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), "UPDATE stored_value_rules SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		handleSQLError(w, sql.ErrNoRows)
		return
	}
	s.audit(r.Context(), actor, "stored_value_rule.delete", "stored_value_rule", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

type storedValueSettingsInput struct {
	Enabled          bool   `json:"enabled"`
	MinRechargeCents int64  `json:"min_recharge_cents"`
	MaxRechargeCents int64  `json:"max_recharge_cents"`
	MaxBalanceCents  int64  `json:"max_balance_cents"`
	DeductionOrder   string `json:"deduction_order"`
	RefundPolicy     string `json:"refund_policy"`
	AgreementURL     string `json:"agreement_url"`
	ShowInMiniapp    bool   `json:"show_in_miniapp"`
}

func (s *Server) getStoredValueSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	input := storedValueSettingsInput{MinRechargeCents: 100, MaxRechargeCents: 1000000, MaxBalanceCents: 1000000, DeductionOrder: "BONUS_FIRST", RefundPolicy: "MANUAL_REVIEW"}
	err := s.DB.QueryRowContext(r.Context(), "SELECT enabled,min_recharge_cents,max_recharge_cents,max_balance_cents,deduction_order,refund_policy,agreement_url,show_in_miniapp FROM stored_value_settings WHERE tenant_id=?", actor.TenantID).Scan(&input.Enabled, &input.MinRechargeCents, &input.MaxRechargeCents, &input.MaxBalanceCents, &input.DeductionOrder, &input.RefundPolicy, &input.AgreementURL, &input.ShowInMiniapp)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, input)
}
func (s *Server) updateStoredValueSettings(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input storedValueSettingsInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.DeductionOrder = strings.ToUpper(strings.TrimSpace(input.DeductionOrder))
	input.RefundPolicy = strings.ToUpper(strings.TrimSpace(input.RefundPolicy))
	if input.MinRechargeCents <= 0 || input.MaxRechargeCents < input.MinRechargeCents || input.MaxRechargeCents > maxBusinessAmountCents || input.MaxBalanceCents < input.MinRechargeCents || input.MaxBalanceCents > maxBusinessAmountCents || !validStatus(input.DeductionOrder, "BONUS_FIRST", "PRINCIPAL_FIRST") || !validStatus(input.RefundPolicy, "MANUAL_REVIEW", "REJECT_AFTER_USE") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid stored-value settings")
		return
	}
	if input.ShowInMiniapp {
		writeError(w, http.StatusConflict, "MINIAPP_RECHARGE_NOT_READY", "mini-program recharge is not connected in V1")
		return
	}
	_, err := s.DB.ExecContext(r.Context(), `INSERT INTO stored_value_settings(tenant_id,enabled,min_recharge_cents,max_recharge_cents,max_balance_cents,deduction_order,refund_policy,agreement_url,show_in_miniapp,updated_by) VALUES(?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE enabled=VALUES(enabled),min_recharge_cents=VALUES(min_recharge_cents),max_recharge_cents=VALUES(max_recharge_cents),max_balance_cents=VALUES(max_balance_cents),deduction_order=VALUES(deduction_order),refund_policy=VALUES(refund_policy),agreement_url=VALUES(agreement_url),show_in_miniapp=VALUES(show_in_miniapp),updated_by=VALUES(updated_by)`, actor.TenantID, input.Enabled, input.MinRechargeCents, input.MaxRechargeCents, input.MaxBalanceCents, input.DeductionOrder, input.RefundPolicy, input.AgreementURL, input.ShowInMiniapp, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "stored_value_settings.update", "settings", "stored_value", map[string]any{"enabled": input.Enabled}, r)
	writeData(w, http.StatusOK, input)
}

type storedValueRecordInput struct {
	CustomerID     int64  `json:"customer_id"`
	RuleID         int64  `json:"rule_id"`
	PrincipalCents int64  `json:"principal_cents"`
	GiftCents      int64  `json:"gift_cents"`
	PaymentMethod  string `json:"payment_method"`
	Remark         string `json:"remark"`
}

func (s *Server) createStoredValueRecord(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	key, ok := requiredIdempotencyKey(w, r)
	if !ok {
		return
	}
	var input storedValueRecordInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.CustomerID <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "customer_id is required")
		return
	}
	input.PaymentMethod = strings.ToUpper(strings.TrimSpace(input.PaymentMethod))
	if input.PaymentMethod == "" {
		input.PaymentMethod = "MANUAL"
	}
	if !validStatus(input.PaymentMethod, "MANUAL", "CASH", "TRANSFER") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "payment_method must be MANUAL, CASH or TRANSFER")
		return
	}
	input.Remark = strings.TrimSpace(input.Remark)
	if input.Remark == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "remark is required for a manual stored-value record")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	fingerprint := requestFingerprint(input)
	var existing int64
	var existingFingerprint string
	if err = tx.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM stored_value_records WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); err == nil {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different stored-value request")
			return
		}
		tx.Rollback()
		writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	var customer int64
	if err = tx.QueryRowContext(r.Context(), "SELECT id FROM customers WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", input.CustomerID, actor.TenantID).Scan(&customer); err != nil {
		handleSQLError(w, err)
		return
	}
	// Recheck after the customer lock. Two concurrent requests can both miss the
	// first read before the winning transaction commits.
	if err = tx.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM stored_value_records WHERE tenant_id=? AND idempotency_key=? FOR UPDATE", actor.TenantID, key).Scan(&existing, &existingFingerprint); err == nil {
		if existingFingerprint != fingerprint {
			writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different stored-value request")
			return
		}
		writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	snapshot := "{}"
	if input.RuleID > 0 {
		var ruleName string
		var perCustomerLimit int
		if err = tx.QueryRowContext(r.Context(), "SELECT name,recharge_cents,gift_cents,per_customer_limit FROM stored_value_rules WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL AND (starts_at IS NULL OR starts_at<=NOW(3)) AND (ends_at IS NULL OR ends_at>=NOW(3))", input.RuleID, actor.TenantID).Scan(&ruleName, &input.PrincipalCents, &input.GiftCents, &perCustomerLimit); err != nil {
			handleSQLError(w, err)
			return
		}
		if perCustomerLimit > 0 {
			var priorCount int
			if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM stored_value_records WHERE tenant_id=? AND customer_id=? AND rule_id=? AND status='CONFIRMED'", actor.TenantID, input.CustomerID, input.RuleID).Scan(&priorCount); err != nil {
				handleSQLError(w, err)
				return
			}
			if priorCount >= perCustomerLimit {
				writeError(w, http.StatusConflict, "RULE_LIMIT_REACHED", "the customer has reached this stored-value rule limit")
				return
			}
		}
		body, _ := json.Marshal(map[string]any{"id": input.RuleID, "name": ruleName, "recharge_cents": input.PrincipalCents, "gift_cents": input.GiftCents})
		snapshot = string(body)
	}
	if input.PrincipalCents <= 0 || input.PrincipalCents > maxBusinessAmountCents || input.GiftCents < 0 || input.GiftCents > maxBusinessAmountCents {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "principal_cents must be positive and gift_cents non-negative")
		return
	}
	minRecharge, maxRecharge, maxBalance := int64(100), int64(1000000), int64(1000000)
	if settingsErr := tx.QueryRowContext(r.Context(), "SELECT min_recharge_cents,max_recharge_cents,max_balance_cents FROM stored_value_settings WHERE tenant_id=?", actor.TenantID).Scan(&minRecharge, &maxRecharge, &maxBalance); settingsErr != nil && !errors.Is(settingsErr, sql.ErrNoRows) {
		handleSQLError(w, settingsErr)
		return
	}
	if input.PrincipalCents < minRecharge {
		writeError(w, http.StatusConflict, "RECHARGE_BELOW_MINIMUM", "principal amount is below the configured single recharge minimum")
		return
	}
	if input.PrincipalCents > maxRecharge {
		writeError(w, http.StatusConflict, "RECHARGE_LIMIT_EXCEEDED", "principal amount exceeds the configured single recharge limit")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `INSERT INTO balance_accounts(tenant_id,customer_id) VALUES(?,?) ON DUPLICATE KEY UPDATE customer_id=VALUES(customer_id)`, actor.TenantID, input.CustomerID); err != nil {
		handleSQLError(w, err)
		return
	}
	var currentPrincipal, currentBonus int64
	if err = tx.QueryRowContext(r.Context(), "SELECT principal_cents,bonus_cents FROM balance_accounts WHERE tenant_id=? AND customer_id=? FOR UPDATE", actor.TenantID, input.CustomerID).Scan(&currentPrincipal, &currentBonus); err != nil {
		handleSQLError(w, err)
		return
	}
	if maxBalance < 0 || maxBalance > maxBusinessAmountCents || !nonNegativeSumWithin(maxBalance, currentPrincipal, currentBonus, input.PrincipalCents, input.GiftCents) {
		writeError(w, http.StatusConflict, "BALANCE_LIMIT_EXCEEDED", "the resulting customer balance would exceed the configured limit")
		return
	}
	recordNo := newBusinessNo("SV")
	result, err := tx.ExecContext(r.Context(), "INSERT INTO stored_value_records(tenant_id,record_no,customer_id,rule_id,rule_snapshot_json,principal_cents,gift_cents,payment_method,idempotency_key,request_fingerprint,created_by,remark) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)", actor.TenantID, recordNo, input.CustomerID, nullableID(input.RuleID), snapshot, input.PrincipalCents, input.GiftCents, input.PaymentMethod, key, fingerprint, actor.UserID, input.Remark)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			// The competing transaction may have committed after this transaction's
			// REPEATABLE READ snapshot was created. Roll back and load through a fresh
			// database statement so an idempotent race returns the winner reliably.
			_ = tx.Rollback()
			if loadErr := s.DB.QueryRowContext(r.Context(), "SELECT id,request_fingerprint FROM stored_value_records WHERE tenant_id=? AND idempotency_key=?", actor.TenantID, key).Scan(&existing, &existingFingerprint); loadErr == nil {
				if existingFingerprint != fingerprint {
					writeError(w, http.StatusConflict, "IDEMPOTENCY_KEY_REUSED", "Idempotency-Key was already used with a different stored-value request")
					return
				}
				writeData(w, http.StatusOK, map[string]any{"id": existing, "idempotent_replay": true})
				return
			}
		}
		handleSQLError(w, err)
		return
	}
	recordID, _ := result.LastInsertId()
	if _, _, err = applyBalanceDeltaTx(r.Context(), tx, actor.TenantID, input.CustomerID, "PRINCIPAL", input.PrincipalCents, "RECHARGE", "STORED_VALUE", recordNo, key+":principal", actor.UserID, input.Remark); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.GiftCents > 0 {
		if _, _, err = applyBalanceDeltaTx(r.Context(), tx, actor.TenantID, input.CustomerID, "BONUS", input.GiftCents, "RECHARGE_GIFT", "STORED_VALUE", recordNo, key+":bonus", actor.UserID, input.Remark); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = auditTx(r.Context(), tx, actor, "stored_value_record.create", "stored_value_record", int64String(recordID), map[string]any{"customer_id": input.CustomerID, "principal_cents": input.PrincipalCents, "gift_cents": input.GiftCents, "payment_method": input.PaymentMethod}, r); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, map[string]any{"id": recordID, "record_no": recordNo})
}

func (s *Server) listStoredValueRecords(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	page, size, offset := pagination(r)
	where := " WHERE v.tenant_id=?"
	args := []any{actor.TenantID}
	if id, err := optionalPositiveInt64(r.URL.Query().Get("customer_id")); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_CUSTOMER", "customer_id must be positive")
		return
	} else if id > 0 {
		where += " AND v.customer_id=?"
		args = append(args, id)
	}
	var total int
	if err := s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM stored_value_records v"+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT v.id,v.record_no,v.customer_id,c.name,COALESCE(r.name,''),v.principal_cents,v.gift_cents,v.payment_method,v.status,v.remark,DATE_FORMAT(v.created_at,'%Y-%m-%d %H:%i:%s') FROM stored_value_records v JOIN customers c ON c.id=v.customer_id AND c.tenant_id=v.tenant_id LEFT JOIN stored_value_rules r ON r.id=v.rule_id AND r.tenant_id=v.tenant_id`+where+" ORDER BY v.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, customerID, principal, gift int64
		var recordNo, customerName, ruleName, method, status, remark, created string
		if err := rows.Scan(&id, &recordNo, &customerID, &customerName, &ruleName, &principal, &gift, &method, &status, &remark, &created); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "record_no": recordNo, "customer_id": customerID, "customer_name": customerName, "rule_name": ruleName, "principal_cents": principal, "gift_cents": gift, "payment_method": method, "status": status, "remark": remark, "created_at": created})
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func requiredIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		writeError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Idempotency-Key header is required and must not exceed 128 characters")
		return "", false
	}
	return key, true
}
func optionalPositiveInt64(raw string) (int64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid positive integer")
	}
	return value, nil
}
func optionalTime(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	parsed, err := parseBeijingDateTime(*value)
	if err != nil {
		return nil, fmt.Errorf("time must use Beijing time such as 2026-07-21 14:00:00")
	}
	return &parsed, nil
}
func nullableID(id int64) any {
	if id <= 0 {
		return nil
	}
	return id
}
func nullableNullInt64(id sql.NullInt64) any {
	if !id.Valid {
		return nil
	}
	return id.Int64
}
func valueOrObject(value any) any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
func jsonValue(raw string) any {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return map[string]any{}
	}
	return value
}
func maskPhone(phone string) string {
	runes := []rune(phone)
	if len(runes) < 7 {
		return phone
	}
	return string(runes[:3]) + "****" + string(runes[len(runes)-4:])
}
func mapStatus(replayed bool, normal, replay int) int {
	if replayed {
		return replay
	}
	return normal
}
func (s *Server) tenantOwnsStore(ctx context.Context, tenantID, storeID int64) bool {
	var found int
	return s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", storeID, tenantID).Scan(&found) == nil && found == 1
}
func (s *Server) tenantOwnsLevel(ctx context.Context, tenantID, levelID int64) bool {
	var found int
	return s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM member_levels WHERE id=? AND tenant_id=? AND deleted_at IS NULL", levelID, tenantID).Scan(&found) == nil && found == 1
}
func auditTx(ctx context.Context, tx *sql.Tx, actor identity, action, resourceType, resourceID string, details any, r *http.Request) error {
	payload, _ := jsonMarshal(details)
	ip := ""
	if r != nil {
		ip = r.RemoteAddr
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_logs(tenant_id,actor_user_id,action,resource_type,resource_id,request_id,ip,details_text) VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, actor.UserID, action, resourceType, resourceID, requestID(ctx), ip, string(payload))
	return err
}
