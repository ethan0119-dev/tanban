package app

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type fastFoodPlateDTO struct {
	ID          int64  `json:"id"`
	StoreID     int64  `json:"storeId"`
	Name        string `json:"plateName"`
	PlateCode   string `json:"plateCode"`
	PublicID    string `json:"publicId"`
	Remark      string `json:"remark"`
	SortOrder   int    `json:"sortOrder"`
	Status      string `json:"status"`
	QRScene     string `json:"qrScene"`
	MiniappPath string `json:"miniappPath"`
}

type fastFoodPlateInput struct {
	StoreID       int64  `json:"storeId"`
	LegacyStoreID int64  `json:"store_id"`
	Name          string `json:"plateName"`
	LegacyName    string `json:"name"`
	PlateCode     string `json:"plateCode"`
	LegacyCode    string `json:"plate_code"`
	Remark        string `json:"remark"`
	SortOrder     int    `json:"sortOrder"`
	Status        string `json:"status"`
}

type fastFoodPlateReference struct {
	ID        int64
	PublicID  string
	Name      string
	PlateCode string
}

func normalizeFastFoodPlateInput(input *fastFoodPlateInput) error {
	if input.StoreID == 0 {
		input.StoreID = input.LegacyStoreID
	}
	if input.Name == "" {
		input.Name = input.LegacyName
	}
	if input.PlateCode == "" {
		input.PlateCode = input.LegacyCode
	}
	input.Name = strings.TrimSpace(input.Name)
	input.PlateCode = strings.TrimSpace(input.PlateCode)
	input.Remark = strings.TrimSpace(input.Remark)
	input.Status = strings.ToUpper(strings.TrimSpace(input.Status))
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if input.Name == "" || len([]rune(input.Name)) > 80 {
		return errors.New("plateName is required and must not exceed 80 characters")
	}
	if input.PlateCode == "" || len(input.PlateCode) > 64 || strings.ContainsAny(input.PlateCode, " \t\r\n") {
		return errors.New("plateCode is required, must not exceed 64 characters, and cannot contain spaces")
	}
	if len([]rune(input.Remark)) > 255 {
		return errors.New("remark must not exceed 255 characters")
	}
	if !validStatus(input.Status, "ACTIVE", "DISABLED") {
		return errors.New("status must be ACTIVE or DISABLED")
	}
	return nil
}

func scanFastFoodPlate(row scanner, item *fastFoodPlateDTO) error {
	if err := row.Scan(&item.ID, &item.StoreID, &item.Name, &item.PlateCode, &item.PublicID, &item.Remark, &item.SortOrder, &item.Status); err != nil {
		return err
	}
	item.QRScene = "fp=" + item.PublicID
	item.MiniappPath = "pages/menu/index"
	return nil
}

func (s *Server) listFastFoodPlates(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	var total int
	if err = s.DB.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM fast_food_plates WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL", actor.TenantID, storeID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,store_id,name,plate_code,public_scene,remark,sort_order,status FROM fast_food_plates
		WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY sort_order,id LIMIT ? OFFSET ?`, actor.TenantID, storeID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []fastFoodPlateDTO{}
	for rows.Next() {
		var item fastFoodPlateDTO
		if err = scanFastFoodPlate(rows, &item); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) createFastFoodPlate(w http.ResponseWriter, r *http.Request) {
	actor := currentIdentity(r.Context())
	var input fastFoodPlateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeFastFoodPlateInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	if input.StoreID == 0 {
		var err error
		input.StoreID, err = s.tenantStoreID(r, actor.TenantID)
		if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	var owned int64
	if err := s.DB.QueryRowContext(r.Context(), "SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL", input.StoreID, actor.TenantID).Scan(&owned); err != nil {
		handleSQLError(w, err)
		return
	}
	publicID, err := newTablePublicID()
	if err != nil {
		handleSQLError(w, err)
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO fast_food_plates(tenant_id,store_id,name,plate_code,public_scene,remark,sort_order,status) VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, input.StoreID, input.Name, input.PlateCode, publicID, input.Remark, input.SortOrder, input.Status)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			writeError(w, http.StatusConflict, "PLATE_CODE_EXISTS", "plateCode already exists in this store")
			return
		}
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), actor, "fast_food_plate.create", "fast_food_plate", int64String(id), input, r)
	s.getFastFoodPlateByID(w, r, actor.TenantID, id)
}

func (s *Server) getFastFoodPlate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "plateID")
	if !ok {
		return
	}
	s.getFastFoodPlateByID(w, r, currentIdentity(r.Context()).TenantID, id)
}

func (s *Server) getFastFoodPlateByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	var item fastFoodPlateDTO
	if err := scanFastFoodPlate(s.DB.QueryRowContext(r.Context(), `SELECT id,store_id,name,plate_code,public_scene,remark,sort_order,status FROM fast_food_plates WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, id, tenantID), &item); err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) updateFastFoodPlate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "plateID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input fastFoodPlateInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := normalizeFastFoodPlateInput(&input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE fast_food_plates SET name=?,plate_code=?,remark=?,sort_order=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.Name, input.PlateCode, input.Remark, input.SortOrder, input.Status, id, actor.TenantID)
	if err != nil {
		if strings.Contains(err.Error(), "1062") {
			writeError(w, http.StatusConflict, "PLATE_CODE_EXISTS", "plateCode already exists in this store")
			return
		}
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "fast-food plate not found")
		return
	}
	s.audit(r.Context(), actor, "fast_food_plate.update", "fast_food_plate", int64String(id), input, r)
	s.getFastFoodPlateByID(w, r, actor.TenantID, id)
}

func (s *Server) deleteFastFoodPlate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "plateID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), "UPDATE fast_food_plates SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, actor.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "fast-food plate not found")
		return
	}
	s.audit(r.Context(), actor, "fast_food_plate.delete", "fast_food_plate", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func resolveOrderFastFoodPlate(ctx context.Context, queryer sqlQueryer, tenantID, storeID int64, publicID string) (fastFoodPlateReference, error) {
	var item fastFoodPlateReference
	err := queryer.QueryRowContext(ctx, `SELECT id,public_scene,name,plate_code FROM fast_food_plates
		WHERE public_scene=? AND tenant_id=? AND store_id=? AND status='ACTIVE' AND deleted_at IS NULL`, publicID, tenantID, storeID).
		Scan(&item.ID, &item.PublicID, &item.Name, &item.PlateCode)
	return item, err
}

func (s *Server) publicResolveFastFoodPlate(w http.ResponseWriter, r *http.Request) {
	publicID := strings.TrimSpace(strings.TrimPrefix(chi.URLParam(r, "code"), "fp="))
	var item fastFoodPlateDTO
	var storeCode, storeName string
	err := s.DB.QueryRowContext(r.Context(), `SELECT p.id,p.store_id,p.name,p.plate_code,p.public_scene,p.remark,p.sort_order,p.status,s.code,s.name
		FROM fast_food_plates p JOIN stores s ON s.id=p.store_id AND s.tenant_id=p.tenant_id
		JOIN tenants t ON t.id=p.tenant_id
		WHERE p.public_scene=? AND p.status='ACTIVE' AND p.deleted_at IS NULL
		AND s.status='ACTIVE' AND s.deleted_at IS NULL AND t.status='ACTIVE'
		AND (t.service_expires_at IS NULL OR t.service_expires_at >= CURRENT_DATE) AND t.deleted_at IS NULL`, publicID).
		Scan(&item.ID, &item.StoreID, &item.Name, &item.PlateCode, &item.PublicID, &item.Remark, &item.SortOrder, &item.Status, &storeCode, &storeName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "FAST_FOOD_PLATE_INVALID", "fast-food plate is disabled or invalid")
			return
		}
		handleSQLError(w, err)
		return
	}
	item.QRScene, item.MiniappPath = "fp="+item.PublicID, "pages/menu/index"
	writeData(w, http.StatusOK, map[string]any{"publicId": item.PublicID, "storeCode": storeCode, "storeName": storeName, "plateCode": item.PlateCode, "plateName": item.Name, "status": item.Status, "qrScene": item.QRScene, "miniappPath": item.MiniappPath})
}
