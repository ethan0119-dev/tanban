package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

const (
	orderTypeDineIn   = "DINE_IN"
	orderTypeTakeout  = "TAKEOUT"
	orderTypeDelivery = "DELIVERY"
)

type tableAreaDTO struct {
	ID        int64  `json:"id"`
	StoreID   int64  `json:"storeId"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
	Status    string `json:"status"`
}

type tableAreaInput struct {
	StoreID       int64  `json:"storeId"`
	LegacyStoreID int64  `json:"store_id"`
	Name          string `json:"name"`
	SortOrder     int    `json:"sortOrder"`
	Status        string `json:"status"`
}

type tableCodeDTO struct {
	ID          int64  `json:"id"`
	StoreID     int64  `json:"storeId"`
	AreaID      int64  `json:"areaId"`
	AreaName    string `json:"areaName"`
	Name        string `json:"name"`
	TableCode   string `json:"tableCode"`
	PublicID    string `json:"publicId"`
	Capacity    int    `json:"capacity"`
	Remark      string `json:"remark"`
	SortOrder   int    `json:"sortOrder"`
	Status      string `json:"status"`
	QRScene     string `json:"qrScene"`
	MiniappPath string `json:"miniappPath"`
}

type tableCodeInput struct {
	StoreID       int64  `json:"storeId"`
	LegacyStoreID int64  `json:"store_id"`
	AreaID        int64  `json:"areaId"`
	LegacyAreaID  int64  `json:"area_id"`
	Name          string `json:"name"`
	TableCode     string `json:"tableCode"`
	LegacyCode    string `json:"table_code"`
	Capacity      int    `json:"capacity"`
	Remark        string `json:"remark"`
	SortOrder     int    `json:"sortOrder"`
	LegacySort    int    `json:"sort_order"`
	Status        string `json:"status"`
}

type orderTableReference struct {
	ID        int64
	PublicID  string
	AreaName  string
	Name      string
	TableCode string
}

func normalizeOrderType(orderType, orderScene, fulfillment string) (string, error) {
	value := strings.ToUpper(strings.TrimSpace(orderType))
	if value == "" {
		value = strings.ToUpper(strings.TrimSpace(orderScene))
	}
	if value == "" {
		value = strings.ToUpper(strings.TrimSpace(fulfillment))
	}
	switch value {
	case "", "PICKUP", orderTypeTakeout:
		return orderTypeTakeout, nil
	case orderTypeDineIn:
		return orderTypeDineIn, nil
	case orderTypeDelivery:
		return orderTypeDelivery, nil
	default:
		return "", errors.New("order_type must be DINE_IN, TAKEOUT or DELIVERY")
	}
}

func legacyFulfillmentType(orderType string) string {
	if orderType == orderTypeDineIn {
		return orderTypeDineIn
	}
	if orderType == orderTypeDelivery {
		return orderTypeDelivery
	}
	return "PICKUP"
}

func normalizeOrderTypeFilter(r *http.Request) (string, error) {
	value := r.URL.Query().Get("order_type")
	if value == "" {
		value = r.URL.Query().Get("orderType")
	}
	if value == "" {
		value = r.URL.Query().Get("order_scene")
	}
	if value == "" {
		return "", nil
	}
	return normalizeOrderType(value, "", "")
}

func newTablePublicID() (string, error) {
	// WeChat mini-program-code scenes are limited to 32 visible characters.
	// Fourteen random bytes encoded as lower-case hex leave room for the "tc="
	// discriminator (31 characters total) and still provide 112 bits of entropy.
	raw := make([]byte, 14)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func normalizeAreaInput(input *tableAreaInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return errors.New("name is required and must not exceed 80 characters")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	return nil
}

func normalizeTableCodeInput(input *tableCodeInput) error {
	if input.StoreID == 0 {
		input.StoreID = input.LegacyStoreID
	}
	if input.AreaID == 0 {
		input.AreaID = input.LegacyAreaID
	}
	if input.TableCode == "" {
		input.TableCode = input.LegacyCode
	}
	if input.SortOrder == 0 {
		input.SortOrder = input.LegacySort
	}
	input.Name = strings.TrimSpace(input.Name)
	input.TableCode = strings.TrimSpace(input.TableCode)
	input.Remark = strings.TrimSpace(input.Remark)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return errors.New("name is required and must not exceed 80 characters")
	}
	if input.AreaID <= 0 {
		return errors.New("areaId is required")
	}
	if len(input.TableCode) > 64 || len([]rune(input.Remark)) > 255 {
		return errors.New("tableCode or remark is too long")
	}
	if input.Capacity == 0 {
		input.Capacity = 1
	}
	if input.Capacity < 1 || input.Capacity > 999 {
		return errors.New("capacity must be between 1 and 999")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	return nil
}

func (s *Server) listTableAreas(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,name,sort_order,status FROM table_areas
		WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY sort_order,id`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []tableAreaDTO{}
	for rows.Next() {
		var item tableAreaDTO
		if err = rows.Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createTableArea(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input tableAreaInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.StoreID == 0 {
		input.StoreID = input.LegacyStoreID
	}
	if err := normalizeAreaInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID := input.StoreID
	if storeID == 0 {
		var err error
		storeID, err = s.tenantStoreID(r, identity.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO table_areas(tenant_id,store_id,name,sort_order,status)
		SELECT ?,id,?,?,? FROM stores WHERE id=? AND tenant_id=? AND status='ACTIVE' AND deleted_at IS NULL`, identity.TenantID, input.Name, input.SortOrder, input.Status, storeID, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if count, _ := result.RowsAffected(); count != 1 {
		writeError(w, http.StatusNotFound, "STORE_NOT_FOUND", "store not found")
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "table_area.create", "table_area", int64String(id), input, r)
	s.getTableAreaByID(w, r, identity.TenantID, id)
}

func (s *Server) getTableAreaByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item tableAreaDTO
	err := s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,sort_order,status FROM table_areas
		WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, tenantID).
		Scan(&item.ID, &item.StoreID, &item.Name, &item.SortOrder, &item.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateTableArea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "areaID")
	if !ok {
		return
	}
	var input tableAreaInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeAreaInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	_, err := s.DB.ExecContext(r.Context(), `UPDATE table_areas SET name=?,sort_order=?,status=?
		WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.Name, input.SortOrder, input.Status, id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "table_area.update", "table_area", int64String(id), input, r)
	s.getTableAreaByID(w, r, identity.TenantID, id)
}

func (s *Server) deleteTableArea(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "areaID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var tableCount int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM table_codes c JOIN table_areas a ON a.id=c.area_id
		WHERE a.id=? AND a.tenant_id=? AND c.tenant_id=? AND c.deleted_at IS NULL`, id, identity.TenantID, identity.TenantID).Scan(&tableCount); err != nil {
		handleSQLError(w, err)
		return
	}
	if tableCount > 0 {
		writeError(w, http.StatusConflict, "AREA_IN_USE", "remove or move the tables in this area first")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE table_areas SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if count, _ := result.RowsAffected(); count == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "table area not found")
		return
	}
	s.audit(r.Context(), identity, "table_area.delete", "table_area", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listTableCodes(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	where := " WHERE c.tenant_id=? AND c.store_id=? AND c.deleted_at IS NULL AND a.deleted_at IS NULL"
	args := []any{identity.TenantID, storeID}
	if raw := r.URL.Query().Get("area_id"); raw != "" {
		areaID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || areaID <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_ID", "area_id must be a positive integer")
			return
		}
		where += " AND c.area_id=?"
		args = append(args, areaID)
	}
	if status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status"))); status != "" {
		if !validStatus(status, "ACTIVE", "DISABLED") {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "status must be ACTIVE or DISABLED")
			return
		}
		where += " AND c.status=?"
		args = append(args, status)
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT c.id,c.store_id,c.area_id,a.name,c.name,c.table_code,c.public_scene,c.capacity,c.remark,c.sort_order,c.status
		FROM table_codes c JOIN table_areas a ON a.id=c.area_id AND a.tenant_id=c.tenant_id AND a.store_id=c.store_id`+where+" ORDER BY a.sort_order,c.sort_order,c.id", args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []tableCodeDTO{}
	for rows.Next() {
		var item tableCodeDTO
		if err = scanTableCode(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createTableCode(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input tableCodeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeTableCodeInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID := input.StoreID
	if storeID == 0 {
		var err error
		storeID, err = s.tenantStoreID(r, identity.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	var areaID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT a.id FROM table_areas a JOIN stores st ON st.id=a.store_id AND st.tenant_id=a.tenant_id
		WHERE a.id=? AND a.tenant_id=? AND a.store_id=? AND a.status='ACTIVE' AND a.deleted_at IS NULL AND st.status='ACTIVE' AND st.deleted_at IS NULL`, input.AreaID, identity.TenantID, storeID).Scan(&areaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "INVALID_TABLE_AREA", "table area does not belong to the selected store or is disabled")
			return
		}
		handleSQLError(w, err)
		return
	}
	publicID, err := newTablePublicID()
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if input.TableCode == "" {
		input.TableCode = "T" + strings.ToUpper(publicID[:8])
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO table_codes(tenant_id,store_id,area_id,name,table_code,public_scene,capacity,remark,sort_order,status)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, identity.TenantID, storeID, areaID, input.Name, input.TableCode, publicID, input.Capacity, input.Remark, input.SortOrder, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "table_code.create", "table_code", int64String(id), map[string]any{"store_id": storeID, "area_id": areaID, "table_code": input.TableCode}, r)
	s.getTableCodeByID(w, r, identity.TenantID, id)
}

func scanTableCode(row scanner, item *tableCodeDTO) error {
	if err := row.Scan(&item.ID, &item.StoreID, &item.AreaID, &item.AreaName, &item.Name, &item.TableCode, &item.PublicID, &item.Capacity, &item.Remark, &item.SortOrder, &item.Status); err != nil {
		return err
	}
	item.QRScene = "tc=" + item.PublicID
	item.MiniappPath = "pages/home/index?scene=tc%3D" + item.PublicID
	return nil
}

func (s *Server) getTableCode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tableID")
	if ok {
		s.getTableCodeByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getTableCodeByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item tableCodeDTO
	err := scanTableCode(s.DB.QueryRowContext(r.Context(), `SELECT c.id,c.store_id,c.area_id,a.name,c.name,c.table_code,c.public_scene,c.capacity,c.remark,c.sort_order,c.status
		FROM table_codes c JOIN table_areas a ON a.id=c.area_id AND a.tenant_id=c.tenant_id AND a.store_id=c.store_id
		WHERE c.id=? AND c.tenant_id=? AND c.deleted_at IS NULL AND a.deleted_at IS NULL`, id, tenantID), &item)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateTableCode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tableID")
	if !ok {
		return
	}
	var input tableCodeInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeTableCodeInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	var currentStoreID int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT store_id FROM table_codes WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID).Scan(&currentStoreID); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.StoreID != 0 && input.StoreID != currentStoreID {
		writeError(w, http.StatusBadRequest, "STORE_IMMUTABLE", "a table code cannot be moved to another store")
		return
	}
	if input.TableCode == "" {
		if err := s.DB.QueryRowContext(r.Context(), "SELECT table_code FROM table_codes WHERE id=? AND tenant_id=?", id, identity.TenantID).Scan(&input.TableCode); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	var validAreaID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT id FROM table_areas WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.AreaID, identity.TenantID, currentStoreID).Scan(&validAreaID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusBadRequest, "INVALID_TABLE_AREA", "table area does not belong to this store")
			return
		}
		handleSQLError(w, err)
		return
	}
	_, err := s.DB.ExecContext(r.Context(), `UPDATE table_codes SET area_id=?,name=?,table_code=?,capacity=?,remark=?,sort_order=?,status=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, validAreaID, input.Name, input.TableCode, input.Capacity, input.Remark, input.SortOrder, input.Status, id, identity.TenantID, currentStoreID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "table_code.update", "table_code", int64String(id), input, r)
	s.getTableCodeByID(w, r, identity.TenantID, id)
}

func (s *Server) deleteTableCode(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "tableID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), "UPDATE table_codes SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if count, _ := result.RowsAffected(); count == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "table code not found")
		return
	}
	s.audit(r.Context(), identity, "table_code.delete", "table_code", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func resolveOrderTable(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, publicID string) (orderTableReference, error) {
	var table orderTableReference
	err := queryer.QueryRowContext(ctx, `SELECT c.id,c.public_scene,a.name,c.name,c.table_code
		FROM table_codes c JOIN table_areas a ON a.id=c.area_id AND a.tenant_id=c.tenant_id AND a.store_id=c.store_id
		JOIN stores st ON st.id=c.store_id AND st.tenant_id=c.tenant_id
		WHERE c.public_scene=? AND c.tenant_id=? AND c.store_id=?
		AND c.status='ACTIVE' AND c.deleted_at IS NULL AND a.status='ACTIVE' AND a.deleted_at IS NULL
		AND st.status='ACTIVE' AND st.deleted_at IS NULL`, publicID, tenantID, storeID).
		Scan(&table.ID, &table.PublicID, &table.AreaName, &table.Name, &table.TableCode)
	return table, err
}

func (s *Server) publicResolveTableCode(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimSpace(chi.URLParam(r, "code"))
	if len(publicID) < 20 || len(publicID) > 32 {
		writeError(w, http.StatusNotFound, "TABLE_CODE_NOT_FOUND", "table code not found")
		return
	}
	var storeID, tenantID int64
	var storeCode, storeName string
	var table orderTableReference
	var capacity int
	var remark string
	err := s.DB.QueryRowContext(r.Context(), `SELECT st.id,st.tenant_id,st.code,st.name,c.id,c.public_scene,a.name,c.name,c.table_code,c.capacity,c.remark
		FROM table_codes c JOIN table_areas a ON a.id=c.area_id AND a.tenant_id=c.tenant_id AND a.store_id=c.store_id
		JOIN stores st ON st.id=c.store_id AND st.tenant_id=c.tenant_id JOIN tenants t ON t.id=c.tenant_id
		WHERE c.public_scene=? AND c.status='ACTIVE' AND c.deleted_at IS NULL
		AND a.status='ACTIVE' AND a.deleted_at IS NULL AND st.status='ACTIVE' AND st.deleted_at IS NULL
		AND t.status='ACTIVE' AND t.deleted_at IS NULL`, publicID).
		Scan(&storeID, &tenantID, &storeCode, &storeName, &table.ID, &table.PublicID, &table.AreaName, &table.Name, &table.TableCode, &capacity, &remark)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "TABLE_CODE_NOT_FOUND", "table code not found")
			return
		}
		handleSQLError(w, err)
		return
	}
	_ = tenantID // Deliberately not exposed by the public contract.
	writeData(w, http.StatusOK, map[string]any{
		"store":      map[string]any{"id": storeID, "code": storeCode, "name": storeName},
		"table":      map[string]any{"publicId": table.PublicID, "name": table.Name, "areaName": table.AreaName, "tableCode": table.TableCode, "capacity": capacity, "remark": remark},
		"tableCode":  table.TableCode,
		"orderScene": orderTypeDineIn,
	})
}
