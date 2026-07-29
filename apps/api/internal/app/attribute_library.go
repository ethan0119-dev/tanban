package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type attributeValueDTO struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"price_delta_cents"`
	IsDefault       bool   `json:"is_default"`
	SortOrder       int    `json:"sort_order"`
	Status          string `json:"status"`
}

type attributeGroupDTO struct {
	ID            int64               `json:"id"`
	Name          string              `json:"name"`
	SelectionMode string              `json:"selection_mode"`
	MinSelect     int                 `json:"min_select"`
	MaxSelect     int                 `json:"max_select"`
	SortOrder     int                 `json:"sort_order"`
	Status        string              `json:"status"`
	ProductCount  int                 `json:"product_count"`
	Values        []attributeValueDTO `json:"values"`
}

type attributeValueInput struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	PriceDeltaCents int64  `json:"price_delta_cents"`
	IsDefault       bool   `json:"is_default"`
	SortOrder       int    `json:"sort_order"`
	Status          string `json:"status"`
}

type attributeGroupInput struct {
	Name          string                `json:"name"`
	SelectionMode string                `json:"selection_mode"`
	MinSelect     int                   `json:"min_select"`
	MaxSelect     int                   `json:"max_select"`
	SortOrder     int                   `json:"sort_order"`
	Status        string                `json:"status"`
	Values        []attributeValueInput `json:"values"`
}

func validateAttributeGroup(input *attributeGroupInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.SelectionMode = strings.ToUpper(strings.TrimSpace(input.SelectionMode))
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.SelectionMode == "" {
		input.SelectionMode = "SINGLE"
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || len([]rune(input.Name)) > 100 {
		return errors.New("attribute group name is required and must not exceed 100 characters")
	}
	if !validStatus(input.SelectionMode, "SINGLE", "MULTIPLE") {
		return errors.New("selection_mode must be SINGLE or MULTIPLE")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	if len(input.Values) == 0 || input.MinSelect < 0 || input.MaxSelect < 1 || input.MinSelect > input.MaxSelect || input.MaxSelect > len(input.Values) {
		return errors.New("attribute group selection limits are invalid")
	}
	if input.SelectionMode == "SINGLE" && input.MaxSelect != 1 {
		return errors.New("single attribute group must have max_select=1")
	}
	seenNames := map[string]bool{}
	activeCount := 0
	defaultCount := 0
	seenIDs := map[int64]bool{}
	for index := range input.Values {
		value := &input.Values[index]
		value.Name = strings.TrimSpace(value.Name)
		value.Status = strings.ToUpper(strings.TrimSpace(value.Status))
		if value.Status == "" {
			value.Status = "ACTIVE"
		}
		key := strings.ToLower(value.Name)
		if value.Name == "" || len([]rune(value.Name)) > 100 || seenNames[key] {
			return fmt.Errorf("attribute value %d has an invalid or duplicate name", index+1)
		}
		if value.ID < 0 || value.ID > 0 && seenIDs[value.ID] {
			return fmt.Errorf("attribute value %d has an invalid or duplicate id", index+1)
		}
		if value.PriceDeltaCents < 0 || value.PriceDeltaCents > maxCatalogUnitPriceCents {
			return fmt.Errorf("attribute value %d has an invalid price", index+1)
		}
		if !validStatus(value.Status, "ACTIVE", "DISABLED") {
			return fmt.Errorf("attribute value %d has an invalid status", index+1)
		}
		seenNames[key] = true
		if value.ID > 0 {
			seenIDs[value.ID] = true
		}
		if value.Status == "ACTIVE" {
			activeCount++
			if value.IsDefault {
				defaultCount++
			}
		}
	}
	if input.Status == "ACTIVE" && activeCount < input.MinSelect {
		return errors.New("attribute group has fewer active values than min_select")
	}
	if defaultCount > input.MaxSelect {
		return errors.New("attribute group has too many default values")
	}
	return nil
}

func (s *Server) listAttributeGroups(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	items, err := s.loadAttributeGroups(r.Context(), actor.TenantID, storeID, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createAttributeGroup(w http.ResponseWriter, r *http.Request) {
	s.saveAttributeGroup(w, r, 0)
}

func (s *Server) updateAttributeGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	s.saveAttributeGroup(w, r, groupID)
}

func (s *Server) saveAttributeGroup(w http.ResponseWriter, r *http.Request, groupID int64) {
	creating := groupID == 0
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input attributeGroupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = validateAttributeGroup(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if groupID > 0 {
		var lockedID int64
		if err = tx.QueryRowContext(r.Context(), `SELECT id FROM attribute_groups WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, groupID, actor.TenantID, storeID).Scan(&lockedID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "attribute group not found")
			} else {
				handleSQLError(w, err)
			}
			return
		}
	}
	var duplicateCount int
	if err = tx.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM attribute_groups WHERE tenant_id=? AND store_id=? AND name=? AND id<>? AND deleted_at IS NULL`, actor.TenantID, storeID, input.Name, groupID).Scan(&duplicateCount); err != nil {
		handleSQLError(w, err)
		return
	}
	if duplicateCount > 0 {
		writeError(w, http.StatusConflict, "ATTRIBUTE_GROUP_EXISTS", "an attribute group with this name already exists")
		return
	}
	if groupID == 0 {
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO attribute_groups(tenant_id,store_id,name,selection_mode,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, storeID, input.Name, input.SelectionMode, input.MinSelect, input.MaxSelect, input.SortOrder, input.Status)
		if insertErr != nil {
			handleSQLError(w, insertErr)
			return
		}
		groupID, _ = result.LastInsertId()
	} else if _, err = tx.ExecContext(r.Context(), `UPDATE attribute_groups SET name=?,selection_mode=?,min_select=?,max_select=?,sort_order=?,status=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.Name, input.SelectionMode, input.MinSelect, input.MaxSelect, input.SortOrder, input.Status, groupID, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	existing := map[int64]bool{}
	rows, err := tx.QueryContext(r.Context(), `SELECT id FROM attribute_values WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, groupID, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			handleSQLError(w, err)
			return
		}
		existing[id] = true
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		handleSQLError(w, err)
		return
	}
	rows.Close()
	kept := map[int64]bool{}
	for index := range input.Values {
		value := &input.Values[index]
		if value.ID > 0 {
			if !existing[value.ID] {
				writeError(w, http.StatusBadRequest, "INVALID_ATTRIBUTE_VALUE", "attribute value does not belong to this group")
				return
			}
			if _, err = tx.ExecContext(r.Context(), `UPDATE attribute_values SET name=?,price_delta_cents=?,is_default=?,sort_order=?,status=? WHERE id=? AND group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, value.Name, value.PriceDeltaCents, value.IsDefault, value.SortOrder, value.Status, value.ID, groupID, actor.TenantID, storeID); err != nil {
				handleSQLError(w, err)
				return
			}
		} else {
			result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO attribute_values(tenant_id,store_id,group_id,name,price_delta_cents,is_default,sort_order,status) VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, storeID, groupID, value.Name, value.PriceDeltaCents, value.IsDefault, value.SortOrder, value.Status)
			if insertErr != nil {
				handleSQLError(w, insertErr)
				return
			}
			value.ID, _ = result.LastInsertId()
		}
		kept[value.ID] = true
	}
	for id := range existing {
		if kept[id] {
			continue
		}
		if _, err = tx.ExecContext(r.Context(), `DELETE FROM product_option_values WHERE attribute_value_id=? AND tenant_id=? AND store_id=?`, id, actor.TenantID, storeID); err != nil {
			handleSQLError(w, err)
			return
		}
		if _, err = tx.ExecContext(r.Context(), `UPDATE attribute_values SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, id, groupID, actor.TenantID, storeID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = syncLinkedAttributeGroup(r.Context(), tx, actor.TenantID, storeID, groupID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "attribute_group.save", "attribute_group", int64String(groupID), map[string]any{"name": input.Name, "value_count": len(input.Values)}, r)
	items, err := s.loadAttributeGroups(r.Context(), actor.TenantID, storeID, false)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	for _, item := range items {
		if item.ID == groupID {
			status := http.StatusOK
			if creating {
				status = http.StatusCreated
			}
			writeData(w, status, item)
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "ATTRIBUTE_GROUP_UNAVAILABLE", "saved attribute group could not be loaded")
}

func syncLinkedAttributeGroup(ctx context.Context, tx *sql.Tx, tenantID, storeID, groupID int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE product_option_groups pog JOIN attribute_groups ag ON ag.id=pog.attribute_group_id
		SET pog.name=ag.name,pog.selection_mode=ag.selection_mode,pog.min_select=ag.min_select,pog.max_select=ag.max_select,pog.status=ag.status
		WHERE pog.attribute_group_id=? AND pog.tenant_id=? AND pog.store_id=?`, groupID, tenantID, storeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE product_option_values pov
		JOIN product_option_groups pog ON pog.id=pov.group_id
		JOIN attribute_values av ON av.id=pov.attribute_value_id
		SET pov.name=av.name,pov.price_delta_cents=av.price_delta_cents,pov.is_default=av.is_default,pov.sort_order=av.sort_order,pov.status=av.status
		WHERE pog.attribute_group_id=? AND pog.tenant_id=? AND pog.store_id=? AND av.deleted_at IS NULL`, groupID, tenantID, storeID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO product_option_values(tenant_id,store_id,group_id,attribute_value_id,name,price_delta_cents,is_default,sort_order,status)
		SELECT pog.tenant_id,pog.store_id,pog.id,av.id,av.name,av.price_delta_cents,av.is_default,av.sort_order,av.status
		FROM product_option_groups pog JOIN attribute_values av ON av.group_id=pog.attribute_group_id AND av.deleted_at IS NULL
		LEFT JOIN product_option_values pov ON pov.group_id=pog.id AND pov.attribute_value_id=av.id
		WHERE pog.attribute_group_id=? AND pog.tenant_id=? AND pog.store_id=? AND pov.id IS NULL`, groupID, tenantID, storeID)
	return err
}

func (s *Server) deleteAttributeGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var productCount int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM product_option_groups WHERE attribute_group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, groupID, actor.TenantID, storeID).Scan(&productCount); err != nil {
		handleSQLError(w, err)
		return
	}
	if productCount > 0 {
		writeError(w, http.StatusConflict, "ATTRIBUTE_GROUP_IN_USE", "attribute group is still used by products")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(r.Context(), `UPDATE attribute_values SET status='DISABLED',deleted_at=NOW(3) WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, groupID, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE attribute_groups SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, groupID, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "attribute group not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "attribute_group.delete", "attribute_group", int64String(groupID), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) loadAttributeGroups(ctx context.Context, tenantID, storeID int64, activeOnly bool) ([]attributeGroupDTO, error) {
	query := `SELECT ag.id,ag.name,ag.selection_mode,ag.min_select,ag.max_select,ag.sort_order,ag.status,
		(SELECT COUNT(*) FROM product_option_groups pog WHERE pog.attribute_group_id=ag.id AND pog.tenant_id=ag.tenant_id AND pog.store_id=ag.store_id AND pog.deleted_at IS NULL)
		FROM attribute_groups ag WHERE ag.tenant_id=? AND ag.store_id=? AND ag.deleted_at IS NULL`
	if activeOnly {
		query += " AND ag.status='ACTIVE'"
	}
	query += " ORDER BY ag.sort_order,ag.id"
	rows, err := s.DB.QueryContext(ctx, query, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	items := []attributeGroupDTO{}
	for rows.Next() {
		var item attributeGroupDTO
		if err = rows.Scan(&item.ID, &item.Name, &item.SelectionMode, &item.MinSelect, &item.MaxSelect, &item.SortOrder, &item.Status, &item.ProductCount); err != nil {
			rows.Close()
			return nil, err
		}
		item.Values = []attributeValueDTO{}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range items {
		valueQuery := `SELECT id,name,price_delta_cents,is_default,sort_order,status FROM attribute_values WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`
		if activeOnly {
			valueQuery += " AND status='ACTIVE'"
		}
		valueQuery += " ORDER BY sort_order,id"
		valueRows, valueErr := s.DB.QueryContext(ctx, valueQuery, items[index].ID, tenantID, storeID)
		if valueErr != nil {
			return nil, valueErr
		}
		for valueRows.Next() {
			var value attributeValueDTO
			if valueErr = valueRows.Scan(&value.ID, &value.Name, &value.PriceDeltaCents, &value.IsDefault, &value.SortOrder, &value.Status); valueErr != nil {
				valueRows.Close()
				return nil, valueErr
			}
			items[index].Values = append(items[index].Values, value)
		}
		if valueErr = valueRows.Err(); valueErr != nil {
			valueRows.Close()
			return nil, valueErr
		}
		valueRows.Close()
	}
	return items, nil
}
