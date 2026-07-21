package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

var allowedCatalogResourceTypes = map[string]bool{
	"PACKAGE": true, "TEMP_PRODUCT": true, "UNIT": true, "PRODUCT_TAG": true,
	"PRINT_LABEL": true, "NOTE": true, "SPEC_TEMPLATE": true,
	"ATTRIBUTE_TEMPLATE": true, "MODIFIER_TEMPLATE": true,
}

const (
	maxCatalogUnitPriceCents = int64(10_000_000)  // ¥100,000 per item
	maxCatalogOrderCents     = int64(100_000_000) // ¥1,000,000 per order
)

type catalogResourceDTO struct {
	ID           int64          `json:"id"`
	ResourceType string         `json:"resource_type"`
	Code         string         `json:"code"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	PriceCents   int64          `json:"price_cents"`
	Config       map[string]any `json:"config"`
	SortOrder    int            `json:"sort_order"`
	Status       string         `json:"status"`
}

type catalogResourceInput struct {
	ResourceType string         `json:"resource_type"`
	Code         string         `json:"code"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	PriceCents   int64          `json:"price_cents"`
	Config       map[string]any `json:"config"`
	SortOrder    int            `json:"sort_order"`
	Status       string         `json:"status"`
}

func normalizeCatalogResourceInput(input *catalogResourceInput) error {
	input.ResourceType = strings.ToUpper(strings.TrimSpace(input.ResourceType))
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if !allowedCatalogResourceTypes[input.ResourceType] {
		return errors.New("unsupported resource_type")
	}
	if input.Name == "" || len([]rune(input.Name)) > 120 {
		return errors.New("name is required and must not exceed 120 characters")
	}
	if input.PriceCents < 0 || input.PriceCents > maxCatalogUnitPriceCents {
		return errors.New("price_cents is outside the allowed range")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	return nil
}

func (s *Server) listCatalogResources(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	resourceType := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("type")))
	if resourceType != "" && !allowedCatalogResourceTypes[resourceType] {
		writeError(w, http.StatusBadRequest, "INVALID_RESOURCE_TYPE", "unsupported resource type")
		return
	}
	query := `SELECT id,resource_type,code,name,description,price_cents,config_json,sort_order,status
		FROM catalog_resources WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`
	args := []any{identity.TenantID, storeID}
	if resourceType != "" {
		query += " AND resource_type=?"
		args = append(args, resourceType)
	}
	query += " ORDER BY resource_type,sort_order,id"
	rows, err := s.DB.QueryContext(r.Context(), query, args...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []catalogResourceDTO{}
	for rows.Next() {
		var item catalogResourceDTO
		var raw string
		if err = rows.Scan(&item.ID, &item.ResourceType, &item.Code, &item.Name, &item.Description, &item.PriceCents, &raw, &item.SortOrder, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		item.Config = map[string]any{}
		_ = json.Unmarshal([]byte(raw), &item.Config)
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createCatalogResource(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input catalogResourceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeCatalogResourceInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	raw, _ := json.Marshal(input.Config)
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO catalog_resources
		(tenant_id,store_id,resource_type,code,name,description,price_cents,config_json,sort_order,status)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, identity.TenantID, storeID, input.ResourceType, input.Code, input.Name, input.Description, input.PriceCents, string(raw), input.SortOrder, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "catalog_resource.create", "catalog_resource", int64String(id), map[string]any{"type": input.ResourceType, "name": input.Name}, r)
	s.getCatalogResourceByID(w, r, identity.TenantID, storeID, id)
}

func (s *Server) updateCatalogResource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "resourceID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input catalogResourceInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeCatalogResourceInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	raw, _ := json.Marshal(input.Config)
	result, err := s.DB.ExecContext(r.Context(), `UPDATE catalog_resources SET resource_type=?,code=?,name=?,description=?,price_cents=?,config_json=?,sort_order=?,status=?
		WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.ResourceType, input.Code, input.Name, input.Description, input.PriceCents, string(raw), input.SortOrder, input.Status, id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "catalog resource not found")
		return
	}
	s.audit(r.Context(), identity, "catalog_resource.update", "catalog_resource", int64String(id), map[string]any{"type": input.ResourceType}, r)
	s.getCatalogResourceByID(w, r, identity.TenantID, storeID, id)
}

func (s *Server) deleteCatalogResource(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "resourceID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var bindings int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM product_resource_bindings b JOIN products p ON p.id=b.product_id
		WHERE b.resource_id=? AND b.tenant_id=? AND b.store_id=? AND p.deleted_at IS NULL`, id, identity.TenantID, storeID).Scan(&bindings); err != nil {
		handleSQLError(w, err)
		return
	}
	if bindings > 0 {
		writeError(w, http.StatusConflict, "RESOURCE_IN_USE", "resource is still bound to products")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE catalog_resources SET deleted_at=NOW(3),status='DISABLED' WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "catalog resource not found")
		return
	}
	s.audit(r.Context(), identity, "catalog_resource.delete", "catalog_resource", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getCatalogResourceByID(w http.ResponseWriter, r *http.Request, tenantID, storeID, id int64) {
	var item catalogResourceDTO
	var raw string
	err := s.DB.QueryRowContext(r.Context(), `SELECT id,resource_type,code,name,description,price_cents,config_json,sort_order,status
		FROM catalog_resources WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, id, tenantID, storeID).
		Scan(&item.ID, &item.ResourceType, &item.Code, &item.Name, &item.Description, &item.PriceCents, &raw, &item.SortOrder, &item.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	item.Config = map[string]any{}
	_ = json.Unmarshal([]byte(raw), &item.Config)
	writeData(w, http.StatusOK, item)
}

type modifierItemDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	ImageURL   string `json:"image_url"`
	SortOrder  int    `json:"sort_order"`
	Status     string `json:"status"`
}

type modifierItemInput struct {
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	ImageURL   string `json:"image_url"`
	SortOrder  int    `json:"sort_order"`
	Status     string `json:"status"`
}

func validateModifierItem(input *modifierItemInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || input.PriceCents < 0 || input.PriceCents > maxCatalogUnitPriceCents {
		return errors.New("name and a price_cents inside the allowed range are required")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	return nil
}

func (s *Server) listModifierItems(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,price_cents,image_url,sort_order,status FROM modifier_items
		WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY sort_order,id`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []modifierItemDTO{}
	for rows.Next() {
		var item modifierItemDTO
		if err = rows.Scan(&item.ID, &item.Name, &item.PriceCents, &item.ImageURL, &item.SortOrder, &item.Status); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createModifierItem(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input modifierItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateModifierItem(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO modifier_items(tenant_id,store_id,name,price_cents,image_url,sort_order,status) VALUES(?,?,?,?,?,?,?)`, identity.TenantID, storeID, input.Name, input.PriceCents, input.ImageURL, input.SortOrder, input.Status)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "modifier_item.create", "modifier_item", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusCreated, modifierItemDTO{ID: id, Name: input.Name, PriceCents: input.PriceCents, ImageURL: input.ImageURL, SortOrder: input.SortOrder, Status: input.Status})
}

func (s *Server) updateModifierItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "itemID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input modifierItemInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateModifierItem(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if input.Status == "DISABLED" {
		var brokenGroups int
		err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM modifier_groups mg
			JOIN modifier_group_items selected ON selected.group_id=mg.id AND selected.modifier_item_id=?
			WHERE mg.tenant_id=? AND mg.store_id=? AND mg.status='ACTIVE' AND mg.deleted_at IS NULL
			AND (SELECT COUNT(*) FROM modifier_group_items links JOIN modifier_items mi ON mi.id=links.modifier_item_id
				WHERE links.group_id=mg.id AND mi.status='ACTIVE' AND mi.deleted_at IS NULL AND mi.id<>?) < mg.min_select`, id, identity.TenantID, storeID, id).Scan(&brokenGroups)
		if err != nil {
			handleSQLError(w, err)
			return
		}
		if brokenGroups > 0 {
			writeError(w, http.StatusConflict, "MODIFIER_REQUIRED", "remove this item from active required groups before disabling it")
			return
		}
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE modifier_items SET name=?,price_cents=?,image_url=?,sort_order=?,status=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.Name, input.PriceCents, input.ImageURL, input.SortOrder, input.Status, id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "modifier item not found")
		return
	}
	s.audit(r.Context(), identity, "modifier_item.update", "modifier_item", int64String(id), nil, r)
	writeData(w, http.StatusOK, modifierItemDTO{ID: id, Name: input.Name, PriceCents: input.PriceCents, ImageURL: input.ImageURL, SortOrder: input.SortOrder, Status: input.Status})
}

func (s *Server) deleteModifierItem(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "itemID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var bindings int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM modifier_group_items links
		JOIN modifier_groups mg ON mg.id=links.group_id AND mg.tenant_id=links.tenant_id AND mg.store_id=links.store_id
		WHERE links.modifier_item_id=? AND links.tenant_id=? AND links.store_id=? AND mg.deleted_at IS NULL`, id, identity.TenantID, storeID).Scan(&bindings); err != nil {
		handleSQLError(w, err)
		return
	}
	if bindings > 0 {
		writeError(w, http.StatusConflict, "MODIFIER_IN_USE", "modifier item is still used by a group")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), "UPDATE modifier_items SET deleted_at=NOW(3),status='DISABLED' WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "modifier item not found")
		return
	}
	s.audit(r.Context(), identity, "modifier_item.delete", "modifier_item", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

type modifierGroupItemDTO struct {
	ModifierItemID    int64  `json:"modifier_item_id"`
	Name              string `json:"name"`
	DefaultPriceCents int64  `json:"default_price_cents"`
	PriceCents        int64  `json:"price_cents"`
	IsDefault         bool   `json:"is_default"`
	SortOrder         int    `json:"sort_order"`
}

type modifierGroupDTO struct {
	ID        int64                  `json:"id"`
	Name      string                 `json:"name"`
	MinSelect int                    `json:"min_select"`
	MaxSelect int                    `json:"max_select"`
	SortOrder int                    `json:"sort_order"`
	Status    string                 `json:"status"`
	Items     []modifierGroupItemDTO `json:"items"`
}

type modifierGroupItemInput struct {
	ModifierItemID int64  `json:"modifier_item_id"`
	PriceOverride  *int64 `json:"price_override_cents"`
	IsDefault      bool   `json:"is_default"`
	SortOrder      int    `json:"sort_order"`
}

type modifierGroupInput struct {
	Name      string                   `json:"name"`
	MinSelect int                      `json:"min_select"`
	MaxSelect int                      `json:"max_select"`
	SortOrder int                      `json:"sort_order"`
	Status    string                   `json:"status"`
	Items     []modifierGroupItemInput `json:"items"`
}

func validateModifierGroup(input *modifierGroupInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || input.MinSelect < 0 || input.MaxSelect < 1 || input.MinSelect > input.MaxSelect {
		return errors.New("name and valid min_select/max_select are required")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	seen := map[int64]bool{}
	defaultCount := 0
	for _, item := range input.Items {
		if item.ModifierItemID <= 0 || seen[item.ModifierItemID] || (item.PriceOverride != nil && (*item.PriceOverride < 0 || *item.PriceOverride > maxCatalogUnitPriceCents)) {
			return errors.New("modifier items must be unique and prices non-negative")
		}
		seen[item.ModifierItemID] = true
		if item.IsDefault {
			defaultCount++
		}
	}
	if len(input.Items) < input.MinSelect {
		return errors.New("group has fewer items than min_select")
	}
	if defaultCount > input.MaxSelect {
		return errors.New("default modifier items exceed max_select")
	}
	return nil
}

func (s *Server) listModifierGroups(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	items, err := s.loadModifierGroups(r.Context(), identity.TenantID, storeID, 0, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createModifierGroup(w http.ResponseWriter, r *http.Request) {
	s.saveModifierGroup(w, r, 0)
}

func (s *Server) updateModifierGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "groupID")
	if ok {
		s.saveModifierGroup(w, r, id)
	}
}

func (s *Server) saveModifierGroup(w http.ResponseWriter, r *http.Request, id int64) {
	identity := currentIdentity(r.Context())
	var input modifierGroupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateModifierGroup(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if id == 0 {
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO modifier_groups(tenant_id,store_id,name,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?,?)`, identity.TenantID, storeID, input.Name, input.MinSelect, input.MaxSelect, input.SortOrder, input.Status)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		id, _ = result.LastInsertId()
	} else {
		result, updateErr := tx.ExecContext(r.Context(), `UPDATE modifier_groups SET name=?,min_select=?,max_select=?,sort_order=?,status=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.Name, input.MinSelect, input.MaxSelect, input.SortOrder, input.Status, id, identity.TenantID, storeID)
		if updateErr != nil {
			handleSQLError(w, updateErr)
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "modifier group not found")
			return
		}
		if _, err = tx.ExecContext(r.Context(), "DELETE FROM modifier_group_items WHERE group_id=? AND tenant_id=? AND store_id=?", id, identity.TenantID, storeID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	for _, item := range input.Items {
		var itemStatus string
		if err = tx.QueryRowContext(r.Context(), `SELECT status FROM modifier_items WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, item.ModifierItemID, identity.TenantID, storeID).Scan(&itemStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusBadRequest, "INVALID_MODIFIER_ITEM", "modifier item does not belong to this store")
				return
			}
			handleSQLError(w, err)
			return
		}
		if input.Status == "ACTIVE" && itemStatus != "ACTIVE" {
			writeError(w, http.StatusBadRequest, "INVALID_MODIFIER_ITEM", "active modifier groups can only contain active items")
			return
		}
		if _, err = tx.ExecContext(r.Context(), `INSERT INTO modifier_group_items(tenant_id,store_id,group_id,modifier_item_id,price_override_cents,is_default,sort_order) VALUES(?,?,?,?,?,?,?)`, identity.TenantID, storeID, id, item.ModifierItemID, item.PriceOverride, item.IsDefault, item.SortOrder); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	action := "modifier_group.update"
	status := http.StatusOK
	if id > 0 {
		// create and update use the same response contract; the audit detail is sufficient.
		if r.Method == http.MethodPost {
			action, status = "modifier_group.create", http.StatusCreated
		}
	}
	s.audit(r.Context(), identity, action, "modifier_group", int64String(id), map[string]any{"name": input.Name}, r)
	groups, err := s.loadModifierGroups(r.Context(), identity.TenantID, storeID, id, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, status, groups[0])
}

func (s *Server) deleteModifierGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var bindings int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM product_modifier_groups links
		JOIN products p ON p.id=links.product_id AND p.tenant_id=links.tenant_id AND p.store_id=links.store_id
		WHERE links.modifier_group_id=? AND links.tenant_id=? AND links.store_id=? AND p.deleted_at IS NULL`, id, identity.TenantID, storeID).Scan(&bindings); err != nil {
		handleSQLError(w, err)
		return
	}
	if bindings > 0 {
		writeError(w, http.StatusConflict, "MODIFIER_GROUP_IN_USE", "modifier group is still bound to products")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM product_modifier_groups WHERE modifier_group_id=? AND tenant_id=? AND store_id=?", id, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM modifier_group_items WHERE group_id=? AND tenant_id=? AND store_id=?", id, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), "UPDATE modifier_groups SET deleted_at=NOW(3),status='DISABLED' WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "modifier group not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "modifier_group.delete", "modifier_group", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadModifierGroups(ctx context.Context, tenantID, storeID, onlyID int64, activeOnly bool) ([]modifierGroupDTO, error) {
	query := `SELECT id,name,min_select,max_select,sort_order,status FROM modifier_groups WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`
	args := []any{tenantID, storeID}
	if onlyID > 0 {
		query += " AND id=?"
		args = append(args, onlyID)
	}
	if activeOnly {
		query += " AND status='ACTIVE'"
	}
	query += " ORDER BY sort_order,id"
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []modifierGroupDTO{}
	for rows.Next() {
		var item modifierGroupDTO
		if err = rows.Scan(&item.ID, &item.Name, &item.MinSelect, &item.MaxSelect, &item.SortOrder, &item.Status); err != nil {
			return nil, err
		}
		item.Items = []modifierGroupItemDTO{}
		items = append(items, item)
	}
	for index := range items {
		itemRows, itemErr := s.DB.QueryContext(ctx, `SELECT mi.id,mi.name,mi.price_cents,COALESCE(mgi.price_override_cents,mi.price_cents),mgi.is_default,mgi.sort_order
			FROM modifier_group_items mgi JOIN modifier_items mi ON mi.id=mgi.modifier_item_id
			WHERE mgi.group_id=? AND mgi.tenant_id=? AND mgi.store_id=? AND mi.deleted_at IS NULL`, items[index].ID, tenantID, storeID)
		if itemErr != nil {
			return nil, itemErr
		}
		for itemRows.Next() {
			var child modifierGroupItemDTO
			if itemErr = itemRows.Scan(&child.ModifierItemID, &child.Name, &child.DefaultPriceCents, &child.PriceCents, &child.IsDefault, &child.SortOrder); itemErr != nil {
				itemRows.Close()
				return nil, itemErr
			}
			items[index].Items = append(items[index].Items, child)
		}
		itemRows.Close()
		sort.Slice(items[index].Items, func(a, b int) bool { return items[index].Items[a].SortOrder < items[index].Items[b].SortOrder })
	}
	return items, rows.Err()
}

type productOptionValueDTO struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"price_delta_cents"`
	IsDefault       bool   `json:"is_default"`
	SortOrder       int    `json:"sort_order"`
	Status          string `json:"status"`
}

type productOptionGroupDTO struct {
	ID            int64                   `json:"id"`
	Name          string                  `json:"name"`
	Kind          string                  `json:"kind"`
	SelectionMode string                  `json:"selection_mode"`
	MinSelect     int                     `json:"min_select"`
	MaxSelect     int                     `json:"max_select"`
	SortOrder     int                     `json:"sort_order"`
	Status        string                  `json:"status"`
	Values        []productOptionValueDTO `json:"values"`
}

type productConfigurationDTO struct {
	OptionGroups     []productOptionGroupDTO `json:"option_groups"`
	ModifierGroups   []modifierGroupDTO      `json:"modifier_groups"`
	ResourceBindings []int64                 `json:"resource_ids"`
}

type productOptionValueInput struct {
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"price_delta_cents"`
	IsDefault       bool   `json:"is_default"`
	SortOrder       int    `json:"sort_order"`
	Status          string `json:"status"`
}

type productOptionGroupInput struct {
	Name          string                    `json:"name"`
	Kind          string                    `json:"kind"`
	SelectionMode string                    `json:"selection_mode"`
	MinSelect     int                       `json:"min_select"`
	MaxSelect     int                       `json:"max_select"`
	SortOrder     int                       `json:"sort_order"`
	Status        string                    `json:"status"`
	Values        []productOptionValueInput `json:"values"`
}

type productConfigurationInput struct {
	OptionGroups     []productOptionGroupInput `json:"option_groups"`
	ModifierGroupIDs []int64                   `json:"modifier_group_ids"`
	ResourceIDs      []int64                   `json:"resource_ids"`
}

func validateProductConfiguration(input *productConfigurationInput) error {
	for index := range input.OptionGroups {
		group := &input.OptionGroups[index]
		group.Name = strings.TrimSpace(group.Name)
		group.Kind = strings.ToUpper(strings.TrimSpace(group.Kind))
		group.SelectionMode = strings.ToUpper(strings.TrimSpace(group.SelectionMode))
		group.Status = strings.ToUpper(strings.TrimSpace(group.Status))
		if group.Kind == "" {
			group.Kind = "ATTRIBUTE"
		}
		if group.SelectionMode == "" {
			group.SelectionMode = "SINGLE"
		}
		if group.Status == "" {
			group.Status = "ACTIVE"
		}
		if group.Name == "" || group.Kind != "ATTRIBUTE" || !validStatus(group.SelectionMode, "SINGLE", "MULTIPLE") || !validStatus(group.Status, "ACTIVE", "DISABLED") {
			return fmt.Errorf("option group %d has invalid name, kind, mode or status", index+1)
		}
		if group.MinSelect < 0 || group.MaxSelect < 1 || group.MinSelect > group.MaxSelect || group.MaxSelect > len(group.Values) {
			return fmt.Errorf("option group %d has invalid selection limits", index+1)
		}
		if group.SelectionMode == "SINGLE" && group.MaxSelect != 1 {
			return fmt.Errorf("single option group %d must have max_select=1", index+1)
		}
		seen := map[string]bool{}
		activeCount := 0
		activeDefaultCount := 0
		for valueIndex := range group.Values {
			value := &group.Values[valueIndex]
			value.Name = strings.TrimSpace(value.Name)
			value.Status = strings.ToUpper(strings.TrimSpace(value.Status))
			if value.Status == "" {
				value.Status = "ACTIVE"
			}
			key := strings.ToLower(value.Name)
			if value.Name == "" || value.PriceDeltaCents < 0 || value.PriceDeltaCents > maxCatalogUnitPriceCents || seen[key] || !validStatus(value.Status, "ACTIVE", "DISABLED") {
				return fmt.Errorf("option group %d has an invalid or duplicate value", index+1)
			}
			seen[key] = true
			if value.Status == "ACTIVE" {
				activeCount++
				if value.IsDefault {
					activeDefaultCount++
				}
			}
		}
		if group.Status == "ACTIVE" && activeCount < group.MinSelect {
			return fmt.Errorf("option group %d has fewer active values than min_select", index+1)
		}
		if activeDefaultCount > group.MaxSelect {
			return fmt.Errorf("option group %d has too many default values", index+1)
		}
	}
	return nil
}

func (s *Server) getProductConfiguration(w http.ResponseWriter, r *http.Request) {
	productID, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var storeID int64
	err := s.DB.QueryRowContext(r.Context(), "SELECT store_id FROM products WHERE id=? AND tenant_id=? AND deleted_at IS NULL", productID, identity.TenantID).Scan(&storeID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		} else {
			handleSQLError(w, err)
		}
		return
	}
	config, err := s.loadProductConfiguration(r.Context(), identity.TenantID, storeID, productID, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, config)
}

func (s *Server) updateProductConfiguration(w http.ResponseWriter, r *http.Request) {
	productID, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input productConfigurationInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := validateProductConfiguration(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID int64
	if err = tx.QueryRowContext(r.Context(), "SELECT store_id FROM products WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", productID, identity.TenantID).Scan(&storeID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		} else {
			handleSQLError(w, err)
		}
		return
	}
	if _, err = tx.ExecContext(r.Context(), `DELETE v FROM product_option_values v JOIN product_option_groups g ON g.id=v.group_id WHERE g.product_id=? AND g.tenant_id=? AND g.store_id=?`, productID, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM product_option_groups WHERE product_id=? AND tenant_id=? AND store_id=?", productID, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	for _, group := range input.OptionGroups {
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO product_option_groups(tenant_id,store_id,product_id,name,kind,selection_mode,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?,?,?,?,?)`, identity.TenantID, storeID, productID, group.Name, group.Kind, group.SelectionMode, group.MinSelect, group.MaxSelect, group.SortOrder, group.Status)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		groupID, _ := result.LastInsertId()
		for _, value := range group.Values {
			if _, err = tx.ExecContext(r.Context(), `INSERT INTO product_option_values(tenant_id,store_id,group_id,name,price_delta_cents,is_default,sort_order,status) VALUES(?,?,?,?,?,?,?,?)`, identity.TenantID, storeID, groupID, value.Name, value.PriceDeltaCents, value.IsDefault, value.SortOrder, value.Status); err != nil {
				handleSQLError(w, err)
				return
			}
		}
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM product_modifier_groups WHERE product_id=? AND tenant_id=? AND store_id=?", productID, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	seenGroups := map[int64]bool{}
	for index, groupID := range input.ModifierGroupIDs {
		if groupID <= 0 || seenGroups[groupID] {
			writeError(w, http.StatusBadRequest, "INVALID_MODIFIER_GROUP", "modifier groups must be unique")
			return
		}
		seenGroups[groupID] = true
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO product_modifier_groups(tenant_id,store_id,product_id,modifier_group_id,sort_order)
			SELECT ?,?,?,id,? FROM modifier_groups WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, identity.TenantID, storeID, productID, index, groupID, identity.TenantID, storeID)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_MODIFIER_GROUP", "modifier group does not belong to this store")
			return
		}
	}
	if _, err = tx.ExecContext(r.Context(), "DELETE FROM product_resource_bindings WHERE product_id=? AND tenant_id=? AND store_id=?", productID, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	seenResources := map[int64]bool{}
	for index, resourceID := range input.ResourceIDs {
		if resourceID <= 0 || seenResources[resourceID] {
			writeError(w, http.StatusBadRequest, "INVALID_RESOURCE", "resources must be unique")
			return
		}
		seenResources[resourceID] = true
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO product_resource_bindings(tenant_id,store_id,product_id,resource_id,binding_type,sort_order)
			SELECT ?,?,?,id,resource_type,? FROM catalog_resources WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, identity.TenantID, storeID, productID, index, resourceID, identity.TenantID, storeID)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			writeError(w, http.StatusBadRequest, "INVALID_RESOURCE", "resource does not belong to this store")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "product.configuration.update", "product", int64String(productID), map[string]any{"option_groups": len(input.OptionGroups), "modifier_groups": len(input.ModifierGroupIDs)}, r)
	config, err := s.loadProductConfiguration(r.Context(), identity.TenantID, storeID, productID, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, config)
}

func (s *Server) loadProductConfiguration(ctx context.Context, tenantID, storeID, productID int64, activeOnly bool) (productConfigurationDTO, error) {
	var productExists int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM products WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", productID, tenantID, storeID).Scan(&productExists); err != nil {
		return productConfigurationDTO{}, err
	}
	if productExists == 0 {
		return productConfigurationDTO{}, sql.ErrNoRows
	}
	query := `SELECT id,name,kind,selection_mode,min_select,max_select,sort_order,status FROM product_option_groups WHERE product_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`
	if activeOnly {
		query += " AND status='ACTIVE'"
	}
	query += " ORDER BY sort_order,id"
	rows, err := s.DB.QueryContext(ctx, query, productID, tenantID, storeID)
	if err != nil {
		return productConfigurationDTO{}, err
	}
	config := productConfigurationDTO{OptionGroups: []productOptionGroupDTO{}, ModifierGroups: []modifierGroupDTO{}, ResourceBindings: []int64{}}
	for rows.Next() {
		var group productOptionGroupDTO
		if err = rows.Scan(&group.ID, &group.Name, &group.Kind, &group.SelectionMode, &group.MinSelect, &group.MaxSelect, &group.SortOrder, &group.Status); err != nil {
			rows.Close()
			return config, err
		}
		group.Values = []productOptionValueDTO{}
		config.OptionGroups = append(config.OptionGroups, group)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return config, err
	}
	rows.Close()
	for index := range config.OptionGroups {
		valueQuery := `SELECT id,name,price_delta_cents,is_default,sort_order,status FROM product_option_values WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`
		if activeOnly {
			valueQuery += " AND status='ACTIVE'"
		}
		valueQuery += " ORDER BY sort_order,id"
		valueRows, valueErr := s.DB.QueryContext(ctx, valueQuery, config.OptionGroups[index].ID, tenantID, storeID)
		if valueErr != nil {
			return config, valueErr
		}
		for valueRows.Next() {
			var value productOptionValueDTO
			if valueErr = valueRows.Scan(&value.ID, &value.Name, &value.PriceDeltaCents, &value.IsDefault, &value.SortOrder, &value.Status); valueErr != nil {
				valueRows.Close()
				return config, valueErr
			}
			config.OptionGroups[index].Values = append(config.OptionGroups[index].Values, value)
		}
		if valueErr = valueRows.Err(); valueErr != nil {
			valueRows.Close()
			return config, valueErr
		}
		valueRows.Close()
	}
	modifierRows, err := s.DB.QueryContext(ctx, `SELECT mg.id,mg.name,mg.min_select,mg.max_select,pmg.sort_order,mg.status
		FROM product_modifier_groups pmg JOIN modifier_groups mg ON mg.id=pmg.modifier_group_id
		WHERE pmg.product_id=? AND pmg.tenant_id=? AND pmg.store_id=? AND mg.deleted_at IS NULL`+func() string {
		if activeOnly {
			return " AND mg.status='ACTIVE'"
		}
		return ""
	}()+` ORDER BY pmg.sort_order,mg.id`, productID, tenantID, storeID)
	if err != nil {
		return config, err
	}
	for modifierRows.Next() {
		var group modifierGroupDTO
		if err = modifierRows.Scan(&group.ID, &group.Name, &group.MinSelect, &group.MaxSelect, &group.SortOrder, &group.Status); err != nil {
			modifierRows.Close()
			return config, err
		}
		group.Items = []modifierGroupItemDTO{}
		config.ModifierGroups = append(config.ModifierGroups, group)
	}
	if err = modifierRows.Err(); err != nil {
		modifierRows.Close()
		return config, err
	}
	modifierRows.Close()
	for index := range config.ModifierGroups {
		itemRows, itemErr := s.DB.QueryContext(ctx, `SELECT mi.id,mi.name,mi.price_cents,COALESCE(mgi.price_override_cents,mi.price_cents),mgi.is_default,mgi.sort_order
			FROM modifier_group_items mgi JOIN modifier_items mi ON mi.id=mgi.modifier_item_id
			WHERE mgi.group_id=? AND mgi.tenant_id=? AND mgi.store_id=? AND mi.deleted_at IS NULL`+func() string {
			if activeOnly {
				return " AND mi.status='ACTIVE'"
			}
			return ""
		}()+` ORDER BY mgi.sort_order,mi.id`, config.ModifierGroups[index].ID, tenantID, storeID)
		if itemErr != nil {
			return config, itemErr
		}
		for itemRows.Next() {
			var item modifierGroupItemDTO
			if itemErr = itemRows.Scan(&item.ModifierItemID, &item.Name, &item.DefaultPriceCents, &item.PriceCents, &item.IsDefault, &item.SortOrder); itemErr != nil {
				itemRows.Close()
				return config, itemErr
			}
			config.ModifierGroups[index].Items = append(config.ModifierGroups[index].Items, item)
		}
		if itemErr = itemRows.Err(); itemErr != nil {
			itemRows.Close()
			return config, itemErr
		}
		itemRows.Close()
	}
	resourceRows, err := s.DB.QueryContext(ctx, `SELECT resource_id FROM product_resource_bindings WHERE product_id=? AND tenant_id=? AND store_id=? ORDER BY sort_order,resource_id`, productID, tenantID, storeID)
	if err != nil {
		return config, err
	}
	for resourceRows.Next() {
		var id int64
		if err = resourceRows.Scan(&id); err != nil {
			resourceRows.Close()
			return config, err
		}
		config.ResourceBindings = append(config.ResourceBindings, id)
	}
	if err = resourceRows.Err(); err != nil {
		resourceRows.Close()
		return config, err
	}
	resourceRows.Close()
	return config, nil
}

type selectedModifierInput struct {
	GroupID        int64 `json:"groupId"`
	ModifierItemID int64 `json:"modifierItemId"`
	Quantity       int   `json:"quantity"`
}

type resolvedProductConfiguration struct {
	PriceDeltaCents int64
	SnapshotJSON    string
}

func resolveProductConfiguration(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, optionValueIDs []int64, modifiers []selectedModifierInput, itemRemark string) (resolvedProductConfiguration, error) {
	type optionValue struct {
		id, groupID, price int64
		groupName, name    string
	}
	type optionGroupLimit struct {
		name     string
		min, max int
	}
	groupRows, err := tx.QueryContext(ctx, `SELECT id,name,min_select,max_select
		FROM product_option_groups
		WHERE product_id=? AND tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL`, productID, tenantID, storeID)
	if err != nil {
		return resolvedProductConfiguration{}, err
	}
	groupLimits := map[int64]optionGroupLimit{}
	for groupRows.Next() {
		var id int64
		var limit optionGroupLimit
		if err = groupRows.Scan(&id, &limit.name, &limit.min, &limit.max); err != nil {
			groupRows.Close()
			return resolvedProductConfiguration{}, err
		}
		groupLimits[id] = limit
	}
	if err = groupRows.Err(); err != nil {
		groupRows.Close()
		return resolvedProductConfiguration{}, err
	}
	groupRows.Close()

	rows, err := tx.QueryContext(ctx, `SELECT v.id,g.id,g.name,v.name,v.price_delta_cents
		FROM product_option_groups g JOIN product_option_values v ON v.group_id=g.id
		WHERE g.product_id=? AND g.tenant_id=? AND g.store_id=? AND g.status='ACTIVE' AND g.deleted_at IS NULL AND v.status='ACTIVE' AND v.deleted_at IS NULL`, productID, tenantID, storeID)
	if err != nil {
		return resolvedProductConfiguration{}, err
	}
	availableOptions := map[int64]optionValue{}
	for rows.Next() {
		var value optionValue
		if err = rows.Scan(&value.id, &value.groupID, &value.groupName, &value.name, &value.price); err != nil {
			rows.Close()
			return resolvedProductConfiguration{}, err
		}
		availableOptions[value.id] = value
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return resolvedProductConfiguration{}, err
	}
	rows.Close()
	selectedOptions := []map[string]any{}
	selectedPerGroup := map[int64]int{}
	seenOptions := map[int64]bool{}
	var delta int64
	for _, id := range optionValueIDs {
		value, found := availableOptions[id]
		if !found || seenOptions[id] {
			return resolvedProductConfiguration{}, errors.New("invalid or duplicate product option")
		}
		seenOptions[id] = true
		selectedPerGroup[value.groupID]++
		if value.price > maxCatalogUnitPriceCents-delta {
			return resolvedProductConfiguration{}, errors.New("configured unit price exceeds the allowed range")
		}
		delta += value.price
		selectedOptions = append(selectedOptions, map[string]any{"groupId": value.groupID, "groupName": value.groupName, "valueId": value.id, "valueName": value.name, "priceDeltaCents": value.price})
	}
	for groupID, limit := range groupLimits {
		selected := selectedPerGroup[groupID]
		if selected < limit.min || selected > limit.max {
			return resolvedProductConfiguration{}, fmt.Errorf("option group %s requires %d to %d selections", limit.name, limit.min, limit.max)
		}
	}
	modifierGroupRows, err := tx.QueryContext(ctx, `SELECT mg.id,mg.name,mg.min_select,mg.max_select FROM product_modifier_groups pmg JOIN modifier_groups mg ON mg.id=pmg.modifier_group_id
		WHERE pmg.product_id=? AND pmg.tenant_id=? AND pmg.store_id=? AND mg.status='ACTIVE' AND mg.deleted_at IS NULL`, productID, tenantID, storeID)
	if err != nil {
		return resolvedProductConfiguration{}, err
	}
	type groupLimit struct {
		name     string
		min, max int
	}
	modifierLimits := map[int64]groupLimit{}
	for modifierGroupRows.Next() {
		var id int64
		var limit groupLimit
		if err = modifierGroupRows.Scan(&id, &limit.name, &limit.min, &limit.max); err != nil {
			modifierGroupRows.Close()
			return resolvedProductConfiguration{}, err
		}
		modifierLimits[id] = limit
	}
	if err = modifierGroupRows.Err(); err != nil {
		modifierGroupRows.Close()
		return resolvedProductConfiguration{}, err
	}
	modifierGroupRows.Close()
	selectedModifiers := []map[string]any{}
	modifierCounts := map[int64]int{}
	seenModifiers := map[string]bool{}
	for _, selected := range modifiers {
		if selected.Quantity <= 0 || selected.Quantity > 99 {
			return resolvedProductConfiguration{}, errors.New("modifier quantity must be between 1 and 99")
		}
		key := fmt.Sprintf("%d:%d", selected.GroupID, selected.ModifierItemID)
		if seenModifiers[key] {
			return resolvedProductConfiguration{}, errors.New("duplicate modifier selection")
		}
		seenModifiers[key] = true
		var groupName, itemName string
		var price int64
		err = tx.QueryRowContext(ctx, `SELECT mg.name,mi.name,COALESCE(mgi.price_override_cents,mi.price_cents)
			FROM product_modifier_groups pmg JOIN modifier_groups mg ON mg.id=pmg.modifier_group_id
			JOIN modifier_group_items mgi ON mgi.group_id=mg.id JOIN modifier_items mi ON mi.id=mgi.modifier_item_id
			WHERE pmg.product_id=? AND pmg.tenant_id=? AND pmg.store_id=? AND mg.id=? AND mi.id=?
			AND mg.status='ACTIVE' AND mg.deleted_at IS NULL AND mi.status='ACTIVE' AND mi.deleted_at IS NULL`, productID, tenantID, storeID, selected.GroupID, selected.ModifierItemID).Scan(&groupName, &itemName, &price)
		if err != nil {
			return resolvedProductConfiguration{}, errors.New("invalid modifier selection")
		}
		modifierCounts[selected.GroupID] += selected.Quantity
		if price > maxCatalogUnitPriceCents/int64(selected.Quantity) {
			return resolvedProductConfiguration{}, errors.New("configured unit price exceeds the allowed range")
		}
		addition := price * int64(selected.Quantity)
		if addition > maxCatalogUnitPriceCents-delta {
			return resolvedProductConfiguration{}, errors.New("configured unit price exceeds the allowed range")
		}
		delta += addition
		selectedModifiers = append(selectedModifiers, map[string]any{"groupId": selected.GroupID, "groupName": groupName, "modifierItemId": selected.ModifierItemID, "name": itemName, "quantity": selected.Quantity, "unitPriceCents": price})
	}
	for groupID, limit := range modifierLimits {
		count := modifierCounts[groupID]
		if count < limit.min || count > limit.max {
			return resolvedProductConfiguration{}, fmt.Errorf("modifier group %s requires %d to %d selections", limit.name, limit.min, limit.max)
		}
	}
	if len([]rune(itemRemark)) > 255 {
		return resolvedProductConfiguration{}, errors.New("item remark must not exceed 255 characters")
	}
	snapshot, _ := json.Marshal(map[string]any{"options": selectedOptions, "modifiers": selectedModifiers, "itemRemark": itemRemark})
	return resolvedProductConfiguration{PriceDeltaCents: delta, SnapshotJSON: string(snapshot)}, nil
}
