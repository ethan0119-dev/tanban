package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"
)

type mediaGroupInput struct {
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

type mediaGroupView struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	SortOrder  int    `json:"sort_order"`
	AssetCount int    `json:"asset_count"`
	CreatedAt  string `json:"created_at"`
}

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Server) listMediaGroups(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT g.id,g.name,g.sort_order,COUNT(a.id),DATE_FORMAT(g.created_at,'%Y-%m-%d %H:%i:%s')
		FROM media_asset_groups g
		LEFT JOIN media_assets a ON a.group_id=g.id AND a.tenant_id=g.tenant_id AND a.store_id=g.store_id AND a.deleted_at IS NULL
		WHERE g.tenant_id=? AND g.store_id=? AND g.deleted_at IS NULL
		GROUP BY g.id,g.name,g.sort_order,g.created_at ORDER BY g.sort_order,g.id`, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []mediaGroupView{}
	for rows.Next() {
		var item mediaGroupView
		if err = rows.Scan(&item.ID, &item.Name, &item.SortOrder, &item.AssetCount, &item.CreatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createMediaGroup(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input mediaGroupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validRequiredText(input.Name, 80) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required and limited to 80 characters")
		return
	}
	var duplicate int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM media_asset_groups WHERE tenant_id=? AND store_id=? AND name=? AND deleted_at IS NULL`, actor.TenantID, storeID, input.Name).Scan(&duplicate); err != nil {
		handleSQLError(w, err)
		return
	}
	if duplicate > 0 {
		writeError(w, http.StatusConflict, "DUPLICATE_MEDIA_GROUP", "a media group with this name already exists")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO media_asset_groups(tenant_id,store_id,name,sort_order,created_by) VALUES(?,?,?,?,?)`, actor.TenantID, storeID, input.Name, input.SortOrder, actor.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), actor, "media_group.create", "media_group", int64String(id), input, r)
	writeData(w, http.StatusCreated, mediaGroupView{ID: id, Name: input.Name, SortOrder: input.SortOrder})
}

func (s *Server) updateMediaGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input mediaGroupInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !validRequiredText(input.Name, 80) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name is required and limited to 80 characters")
		return
	}
	var duplicate int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM media_asset_groups WHERE tenant_id=? AND store_id=? AND name=? AND id<>? AND deleted_at IS NULL`, actor.TenantID, storeID, input.Name, id).Scan(&duplicate); err != nil {
		handleSQLError(w, err)
		return
	}
	if duplicate > 0 {
		writeError(w, http.StatusConflict, "DUPLICATE_MEDIA_GROUP", "a media group with this name already exists")
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE media_asset_groups SET name=?,sort_order=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, input.Name, input.SortOrder, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media group not found")
		return
	}
	s.audit(r.Context(), actor, "media_group.update", "media_group", int64String(id), input, r)
	writeData(w, http.StatusOK, mediaGroupView{ID: id, Name: input.Name, SortOrder: input.SortOrder})
}

func (s *Server) deleteMediaGroup(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "groupID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
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
	if err = lockDecorationStore(r.Context(), tx, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE media_asset_groups SET deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, id, actor.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media group not found")
		return
	}
	if _, err = tx.ExecContext(r.Context(), `UPDATE media_assets SET group_id=NULL WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, id, actor.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "media_group.delete", "media_group", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func validateMediaGroupID(ctx context.Context, queryer queryRower, tenantID, storeID, groupID int64) error {
	if groupID == 0 {
		return nil
	}
	var found int64
	err := queryer.QueryRowContext(ctx, `SELECT id FROM media_asset_groups WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL FOR UPDATE`, groupID, tenantID, storeID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("media group does not belong to this store")
	}
	return err
}
