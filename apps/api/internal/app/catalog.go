package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type skuDTO struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Attributes  map[string]any `json:"attributes"`
	PriceCents  int64          `json:"price_cents"`
	Status      string         `json:"status"`
	Stock       int            `json:"stock"`
	AutoSoldOut bool           `json:"auto_sold_out"`
	AutoRefill  bool           `json:"auto_refill"`
	RefillStock int            `json:"refill_stock"`
}

type productDTO struct {
	ID          int64    `json:"id"`
	StoreID     int64    `json:"store_id"`
	CategoryID  int64    `json:"category_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ImageURL    string   `json:"image_url"`
	SortOrder   int      `json:"sort_order"`
	Status      string   `json:"status"`
	SKUs        []skuDTO `json:"skus"`
}

type skuInput struct {
	ID            int64          `json:"id"`
	Name          string         `json:"name"`
	Attributes    map[string]any `json:"attributes"`
	PriceCents    int64          `json:"price_cents"`
	Status        string         `json:"status"`
	Stock         int            `json:"stock"`
	ExpectedStock *int           `json:"expected_stock"`
	AutoSoldOut   *bool          `json:"auto_sold_out"`
	AutoRefill    bool           `json:"auto_refill"`
	RefillStock   int            `json:"refill_stock"`
}

type productInput struct {
	StoreID     int64      `json:"store_id"`
	CategoryID  int64      `json:"category_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	ImageURL    string     `json:"image_url"`
	SortOrder   int        `json:"sort_order"`
	Status      string     `json:"status"`
	SKUs        []skuInput `json:"skus"`
}

func (s *Server) listProducts(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	items, err := s.loadProducts(r.Context(), identity.TenantID, storeID, false, 0)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, items)
}

func (s *Server) createProduct(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	var input productInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name == "" || input.CategoryID == 0 || len(input.SKUs) == 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name, category_id and at least one sku are required")
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
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var categoryExists int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM categories WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", input.CategoryID, identity.TenantID, storeID).Scan(&categoryExists); err != nil {
		handleSQLError(w, err)
		return
	}
	if categoryExists == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_CATEGORY", "category does not belong to this store")
		return
	}
	result, err := tx.ExecContext(r.Context(), `INSERT INTO products(tenant_id,store_id,category_id,name,description,image_url,sort_order,status) VALUES(?,?,?,?,?,?,?,?)`, identity.TenantID, storeID, input.CategoryID, input.Name, input.Description, input.ImageURL, input.SortOrder, strings.ToUpper(input.Status))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	productID, _ := result.LastInsertId()
	for _, sku := range input.SKUs {
		if err = insertSKU(r.Context(), tx, identity.TenantID, storeID, productID, sku); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_SKU", err.Error())
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "product.create", "product", int64String(productID), map[string]any{"name": input.Name}, r)
	s.getProductByID(w, r, identity.TenantID, productID)
}

func insertSKU(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, input skuInput) error {
	if input.Name == "" || input.PriceCents < 0 || input.PriceCents > maxCatalogUnitPriceCents || input.Stock < 0 {
		return errors.New("sku name, stock and a price inside the allowed range are required")
	}
	if input.AutoRefill {
		return errors.New("daily inventory refill is reserved but not enabled yet")
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	autoSoldOut := true
	if input.AutoSoldOut != nil {
		autoSoldOut = *input.AutoSoldOut
	}
	attrs, _ := json.Marshal(input.Attributes)
	result, err := tx.ExecContext(ctx, `INSERT INTO skus(tenant_id,store_id,product_id,name,attributes_json,price_cents,status) VALUES(?,?,?,?,?,?,?)`, tenantID, storeID, productID, input.Name, string(attrs), input.PriceCents, strings.ToUpper(input.Status))
	if err != nil {
		return err
	}
	skuID, _ := result.LastInsertId()
	_, err = tx.ExecContext(ctx, "INSERT INTO inventory(sku_id,tenant_id,store_id,stock,auto_sold_out,auto_refill,refill_stock) VALUES(?,?,?,?,?,?,?)", skuID, tenantID, storeID, input.Stock, autoSoldOut, input.AutoRefill, input.RefillStock)
	return err
}

func (s *Server) getProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "productID")
	if ok {
		s.getProductByID(w, r, currentIdentity(r.Context()).TenantID, id)
	}
}

func (s *Server) getProductByID(w http.ResponseWriter, r *http.Request, tenantID, id int64) {
	items, err := s.loadProducts(r.Context(), tenantID, 0, false, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if len(items) == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	writeData(w, http.StatusOK, items[0])
}

func (s *Server) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	var input productInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Status == "" {
		input.Status = "ACTIVE"
	}
	if strings.TrimSpace(input.Name) == "" || input.CategoryID <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "name and category_id are required")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID int64
	if err = tx.QueryRowContext(r.Context(), "SELECT store_id FROM products WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE", id, identity.TenantID).Scan(&storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	var categoryExists int
	if err = tx.QueryRowContext(r.Context(), "SELECT COUNT(*) FROM categories WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL", input.CategoryID, identity.TenantID, storeID).Scan(&categoryExists); err != nil {
		handleSQLError(w, err)
		return
	}
	if categoryExists == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_CATEGORY", "category does not belong to this store")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE products SET category_id=?,name=?,description=?,image_url=?,sort_order=?,status=? WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, input.CategoryID, input.Name, input.Description, input.ImageURL, input.SortOrder, strings.ToUpper(input.Status), id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if len(input.SKUs) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_SKU", "at least one sku is required")
		return
	}
	retainedSKU := make(map[int64]bool, len(input.SKUs))
	existingSKUSet := make(map[int64]bool)
	existingRows, err := tx.QueryContext(r.Context(), "SELECT id FROM skus WHERE product_id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var existingSKU []int64
	for existingRows.Next() {
		var skuID int64
		if err = existingRows.Scan(&skuID); err != nil {
			existingRows.Close()
			handleSQLError(w, err)
			return
		}
		existingSKU = append(existingSKU, skuID)
		existingSKUSet[skuID] = true
	}
	if err = existingRows.Err(); err != nil {
		existingRows.Close()
		handleSQLError(w, err)
		return
	}
	existingRows.Close()
	for _, sku := range input.SKUs {
		if strings.TrimSpace(sku.Name) == "" || sku.PriceCents < 0 || sku.PriceCents > maxCatalogUnitPriceCents || sku.Stock < 0 || sku.RefillStock < 0 {
			writeError(w, http.StatusBadRequest, "INVALID_SKU", "sku name and non-negative price_cents/stock/refill_stock are required")
			return
		}
		if sku.ID == 0 {
			if err = insertSKU(r.Context(), tx, identity.TenantID, storeID, id, sku); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_SKU", err.Error())
				return
			}
			continue
		}
		if sku.ExpectedStock == nil || *sku.ExpectedStock < 0 {
			writeError(w, http.StatusBadRequest, "EXPECTED_STOCK_REQUIRED", "expected_stock is required for every existing sku")
			return
		}
		if !existingSKUSet[sku.ID] {
			writeError(w, http.StatusBadRequest, "INVALID_SKU", "sku does not belong to this product")
			return
		}
		if sku.AutoRefill {
			writeError(w, http.StatusConflict, "FEATURE_NOT_READY", "daily inventory refill is reserved but not enabled yet")
			return
		}
		retainedSKU[sku.ID] = true
		if sku.Status == "" {
			sku.Status = "ACTIVE"
		}
		attrs, _ := json.Marshal(sku.Attributes)
		if _, err = tx.ExecContext(r.Context(), `UPDATE skus SET name=?,attributes_json=?,price_cents=?,status=? WHERE id=? AND product_id=? AND tenant_id=? AND deleted_at IS NULL`, sku.Name, string(attrs), sku.PriceCents, strings.ToUpper(sku.Status), sku.ID, id, identity.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
		autoSoldOut := true
		if sku.AutoSoldOut != nil {
			autoSoldOut = *sku.AutoSoldOut
		}
		if err = updateInventoryOptimistic(r.Context(), tx, identity.TenantID, sku.ID, *sku.ExpectedStock, sku.Stock, autoSoldOut, sku.AutoRefill, sku.RefillStock); errors.Is(err, errStockConflict) {
			writeError(w, http.StatusConflict, "STOCK_CONFLICT", "inventory changed while this product was being edited; refresh and retry")
			return
		} else if err != nil {
			handleSQLError(w, err)
			return
		}
	}
	var removedSKU []int64
	for _, skuID := range existingSKU {
		if !retainedSKU[skuID] {
			removedSKU = append(removedSKU, skuID)
		}
	}
	for _, skuID := range removedSKU {
		if _, err = tx.ExecContext(r.Context(), "UPDATE skus SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND product_id=? AND tenant_id=? AND deleted_at IS NULL", skuID, id, identity.TenantID); err != nil {
			handleSQLError(w, err)
			return
		}
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "product.update", "product", int64String(id), map[string]any{"name": input.Name}, r)
	s.getProductByID(w, r, identity.TenantID, id)
}

func (s *Server) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	identity := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(r.Context(), "UPDATE products SET status='DISABLED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	if err == nil {
		_, err = tx.ExecContext(r.Context(), "UPDATE skus SET status='DISABLED',deleted_at=NOW(3) WHERE product_id=? AND tenant_id=? AND deleted_at IS NULL", id, identity.TenantID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if n, _ := result.RowsAffected(); n == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "product not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "product.delete", "product", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

type inventoryInput struct {
	Stock         int  `json:"stock"`
	ExpectedStock *int `json:"expected_stock"`
	AutoSoldOut   bool `json:"auto_sold_out"`
	AutoRefill    bool `json:"auto_refill"`
	RefillStock   int  `json:"refill_stock"`
}

func (s *Server) updateInventory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "skuID")
	if !ok {
		return
	}
	var input inventoryInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Stock < 0 || input.RefillStock < 0 || input.ExpectedStock == nil || *input.ExpectedStock < 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "stock/refill_stock must be non-negative and expected_stock is required")
		return
	}
	if input.AutoRefill {
		writeError(w, http.StatusConflict, "FEATURE_NOT_READY", "daily inventory refill is reserved but not enabled yet")
		return
	}
	identity := currentIdentity(r.Context())
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = updateInventoryOptimistic(r.Context(), tx, identity.TenantID, id, *input.ExpectedStock, input.Stock, input.AutoSoldOut, input.AutoRefill, input.RefillStock); errors.Is(err, errStockConflict) {
		writeError(w, http.StatusConflict, "STOCK_CONFLICT", "inventory changed; refresh and retry")
		return
	} else if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "inventory.update", "sku", int64String(id), input, r)
	writeData(w, http.StatusOK, map[string]any{"sku_id": id, "stock": input.Stock, "auto_sold_out": input.AutoSoldOut, "auto_refill": input.AutoRefill, "refill_stock": input.RefillStock})
}

var errStockConflict = errors.New("inventory stock changed")

func updateInventoryOptimistic(ctx context.Context, tx *sql.Tx, tenantID, skuID int64, expectedStock, stock int, autoSoldOut, autoRefill bool, refillStock int) error {
	var currentStock int
	if err := tx.QueryRowContext(ctx, `SELECT stock FROM inventory WHERE sku_id=? AND tenant_id=? FOR UPDATE`, skuID, tenantID).Scan(&currentStock); err != nil {
		return err
	}
	if currentStock != expectedStock {
		return errStockConflict
	}
	_, err := tx.ExecContext(ctx, `UPDATE inventory SET stock=?,auto_sold_out=?,auto_refill=?,refill_stock=? WHERE sku_id=? AND tenant_id=?`, stock, autoSoldOut, autoRefill, refillStock, skuID, tenantID)
	return err
}

func (s *Server) loadProducts(ctx context.Context, tenantID, storeID int64, publicOnly bool, productID int64) ([]productDTO, error) {
	query := `SELECT id,store_id,category_id,name,description,image_url,sort_order,status FROM products WHERE tenant_id=? AND deleted_at IS NULL`
	args := []any{tenantID}
	if storeID > 0 {
		query += " AND store_id=?"
		args = append(args, storeID)
	}
	if productID > 0 {
		query += " AND id=?"
		args = append(args, productID)
	}
	if publicOnly {
		query += " AND status='ACTIVE' AND EXISTS(SELECT 1 FROM categories c WHERE c.id=products.category_id AND c.tenant_id=products.tenant_id AND c.store_id=products.store_id AND c.status='ACTIVE' AND c.deleted_at IS NULL)"
	}
	query += " ORDER BY sort_order,id"
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []productDTO{}
	for rows.Next() {
		var item productDTO
		if err := rows.Scan(&item.ID, &item.StoreID, &item.CategoryID, &item.Name, &item.Description, &item.ImageURL, &item.SortOrder, &item.Status); err != nil {
			return nil, err
		}
		item.SKUs, err = s.loadSKUs(ctx, tenantID, item.ID, publicOnly)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Server) loadSKUs(ctx context.Context, tenantID, productID int64, publicOnly bool) ([]skuDTO, error) {
	query := `SELECT s.id,s.name,s.attributes_json,s.price_cents,s.status,i.stock,i.auto_sold_out,i.auto_refill,i.refill_stock
		FROM skus s JOIN inventory i ON i.sku_id=s.id WHERE s.tenant_id=? AND s.product_id=? AND s.deleted_at IS NULL`
	if publicOnly {
		query += " AND s.status='ACTIVE' AND (i.auto_sold_out=0 OR i.stock>0)"
	}
	query += " ORDER BY s.id"
	rows, err := s.DB.QueryContext(ctx, query, tenantID, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []skuDTO{}
	for rows.Next() {
		var item skuDTO
		var attrs string
		if err := rows.Scan(&item.ID, &item.Name, &attrs, &item.PriceCents, &item.Status, &item.Stock, &item.AutoSoldOut, &item.AutoRefill, &item.RefillStock); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(attrs), &item.Attributes)
		if item.Attributes == nil {
			item.Attributes = map[string]any{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

var _ = sql.ErrNoRows
