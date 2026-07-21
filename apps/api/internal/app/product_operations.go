package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxProductImages = 4

type resolvedProductImage struct {
	MediaAssetID *int64
	URL          string
	IsPrimary    bool
	SortOrder    int
}

func resolveProductImages(ctx context.Context, queryer queryRower, tenantID, storeID int64, images []productImageInput, legacyURL string) ([]resolvedProductImage, string, error) {
	if images == nil {
		legacyURL = strings.TrimSpace(legacyURL)
		if legacyURL == "" {
			return []resolvedProductImage{}, "", nil
		}
		if len(legacyURL) > 1024 || !validDecorationURL(legacyURL) {
			return nil, "", errors.New("image_url must be a valid HTTPS URL no longer than 1024 characters")
		}
		return []resolvedProductImage{{URL: legacyURL, IsPrimary: true}}, legacyURL, nil
	}
	if len(images) > maxProductImages {
		return nil, "", fmt.Errorf("a product supports one primary image and at most three gallery images")
	}
	if len(images) == 0 {
		return []resolvedProductImage{}, "", nil
	}
	primaryCount := 0
	seen := make(map[string]bool, len(images))
	resolved := make([]resolvedProductImage, 0, len(images))
	mainURL := ""
	for _, input := range images {
		if input.IsPrimary {
			primaryCount++
		}
		url := strings.TrimSpace(input.URL)
		var assetID *int64
		if input.MediaAssetID > 0 {
			var assetURL string
			if err := queryer.QueryRowContext(ctx, `SELECT url FROM media_assets WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, input.MediaAssetID, tenantID, storeID).Scan(&assetURL); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return nil, "", fmt.Errorf("media asset %d does not belong to this store", input.MediaAssetID)
				}
				return nil, "", err
			}
			url = assetURL
			value := input.MediaAssetID
			assetID = &value
		} else if url == "" || len(url) > 1024 || !validDecorationURL(url) {
			return nil, "", errors.New("every product image requires a store media asset or a valid HTTPS URL")
		}
		if seen[url] {
			return nil, "", errors.New("the same image cannot be selected more than once")
		}
		seen[url] = true
		item := resolvedProductImage{MediaAssetID: assetID, URL: url, IsPrimary: input.IsPrimary, SortOrder: input.SortOrder}
		if item.IsPrimary {
			mainURL = url
		}
		resolved = append(resolved, item)
	}
	if primaryCount != 1 {
		return nil, "", errors.New("exactly one product image must be marked as primary")
	}
	return resolved, mainURL, nil
}

func insertResolvedProductImages(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, images []resolvedProductImage) error {
	for _, image := range images {
		var assetID any
		if image.MediaAssetID != nil {
			assetID = *image.MediaAssetID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO product_images(tenant_id,store_id,product_id,media_asset_id,url,is_primary,sort_order) VALUES(?,?,?,?,?,?,?)`, tenantID, storeID, productID, assetID, image.URL, image.IsPrimary, image.SortOrder); err != nil {
			return err
		}
	}
	return nil
}

func replaceProductImages(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, images []resolvedProductImage) error {
	if _, err := tx.ExecContext(ctx, `UPDATE product_images SET deleted_at=NOW(3) WHERE tenant_id=? AND store_id=? AND product_id=? AND deleted_at IS NULL`, tenantID, storeID, productID); err != nil {
		return err
	}
	return insertResolvedProductImages(ctx, tx, tenantID, storeID, productID, images)
}

func syncLegacyProductPrimary(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, imageURL string) error {
	imageURL = strings.TrimSpace(imageURL)
	var imageID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM product_images WHERE tenant_id=? AND store_id=? AND product_id=? AND is_primary=1 AND deleted_at IS NULL ORDER BY id LIMIT 1 FOR UPDATE`, tenantID, storeID, productID).Scan(&imageID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if imageURL == "" {
		if imageID == 0 {
			return nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE product_images SET deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND store_id=?`, imageID, tenantID, storeID)
		return err
	}
	if imageID > 0 {
		_, err = tx.ExecContext(ctx, `UPDATE product_images SET media_asset_id=NULL,url=?,sort_order=0 WHERE id=? AND tenant_id=? AND store_id=?`, imageURL, imageID, tenantID, storeID)
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO product_images(tenant_id,store_id,product_id,url,is_primary,sort_order) VALUES(?,?,?,?,1,0)`, tenantID, storeID, productID, imageURL)
	return err
}

type productActionInput struct {
	Action string `json:"action"`
	Stock  *int   `json:"stock"`
}

func (s *Server) performProductAction(w http.ResponseWriter, r *http.Request) {
	productID, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var input productActionInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Action = strings.ToUpper(strings.TrimSpace(input.Action))
	if input.Stock != nil && (*input.Stock < 0 || *input.Stock > 1_000_000_000) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "stock must be between 0 and 1000000000")
		return
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	var storeID int64
	var name string
	if err = tx.QueryRowContext(r.Context(), `SELECT store_id,name FROM products WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE`, productID, actor.TenantID).Scan(&storeID, &name); err != nil {
		handleSQLError(w, err)
		return
	}
	responseProductID := productID
	switch input.Action {
	case "ACTIVATE":
		_, err = tx.ExecContext(r.Context(), `UPDATE products SET status='ACTIVE' WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, productID, actor.TenantID, storeID)
	case "DEACTIVATE":
		_, err = tx.ExecContext(r.Context(), `UPDATE products SET status='DISABLED' WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, productID, actor.TenantID, storeID)
	case "RECOMMEND":
		_, err = tx.ExecContext(r.Context(), `UPDATE products SET recommended=1 WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, productID, actor.TenantID, storeID)
	case "UNRECOMMEND":
		_, err = tx.ExecContext(r.Context(), `UPDATE products SET recommended=0 WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, productID, actor.TenantID, storeID)
	case "SOLD_OUT":
		_, err = tx.ExecContext(r.Context(), `UPDATE inventory i JOIN skus s ON s.id=i.sku_id AND s.tenant_id=i.tenant_id AND s.store_id=i.store_id
			SET i.stock=0 WHERE s.product_id=? AND s.tenant_id=? AND s.store_id=? AND s.deleted_at IS NULL`, productID, actor.TenantID, storeID)
	case "RESTOCK_FULL":
		if input.Stock != nil {
			_, err = tx.ExecContext(r.Context(), `UPDATE inventory i JOIN skus s ON s.id=i.sku_id AND s.tenant_id=i.tenant_id AND s.store_id=i.store_id
				SET i.stock=? WHERE s.product_id=? AND s.tenant_id=? AND s.store_id=? AND s.deleted_at IS NULL`, *input.Stock, productID, actor.TenantID, storeID)
		} else {
			_, err = tx.ExecContext(r.Context(), `UPDATE inventory i JOIN skus s ON s.id=i.sku_id AND s.tenant_id=i.tenant_id AND s.store_id=i.store_id
				SET i.stock=CASE WHEN i.refill_stock>0 THEN i.refill_stock ELSE i.stock END
				WHERE s.product_id=? AND s.tenant_id=? AND s.store_id=? AND s.deleted_at IS NULL`, productID, actor.TenantID, storeID)
		}
	case "COPY":
		responseProductID, err = copyProduct(r.Context(), tx, actor.TenantID, storeID, productID, name)
	default:
		writeError(w, http.StatusBadRequest, "INVALID_PRODUCT_ACTION", "action must be ACTIVATE, DEACTIVATE, RECOMMEND, UNRECOMMEND, COPY, SOLD_OUT or RESTOCK_FULL")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), actor, "product.action."+strings.ToLower(input.Action), "product", int64String(responseProductID), map[string]any{"source_product_id": productID, "stock": input.Stock}, r)
	s.getProductByID(w, r, actor.TenantID, responseProductID)
}

type skuClone struct {
	Name           string
	AttributesJSON string
	PriceCents     int64
	Status         string
	Stock          int
	AutoSoldOut    bool
	AutoRefill     bool
	RefillStock    int
}

func copyProduct(ctx context.Context, tx *sql.Tx, tenantID, storeID, productID int64, name string) (int64, error) {
	copyName := strings.TrimSpace(name) + "（副本）"
	if len([]rune(copyName)) > 120 {
		copyName = string([]rune(copyName)[:120])
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO products(tenant_id,store_id,category_id,product_type,unit_resource_id,name,description,image_url,sort_order,recommended,in_store_enabled,delivery_enabled,max_per_order,cashier_only,channels_json,sale_periods_json,print_label_resource_id,status)
		SELECT tenant_id,store_id,category_id,product_type,unit_resource_id,?,description,image_url,sort_order+1,0,in_store_enabled,0,max_per_order,cashier_only,channels_json,sale_periods_json,print_label_resource_id,'DISABLED'
		FROM products WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, copyName, productID, tenantID, storeID)
	if err != nil {
		return 0, err
	}
	copyID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.name,s.attributes_json,s.price_cents,s.status,i.stock,i.auto_sold_out,i.auto_refill,i.refill_stock
		FROM skus s JOIN inventory i ON i.sku_id=s.id AND i.tenant_id=s.tenant_id AND i.store_id=s.store_id
		WHERE s.product_id=? AND s.tenant_id=? AND s.store_id=? AND s.deleted_at IS NULL ORDER BY s.id`, productID, tenantID, storeID)
	if err != nil {
		return 0, err
	}
	clones := []skuClone{}
	for rows.Next() {
		var clone skuClone
		if err = rows.Scan(&clone.Name, &clone.AttributesJSON, &clone.PriceCents, &clone.Status, &clone.Stock, &clone.AutoSoldOut, &clone.AutoRefill, &clone.RefillStock); err != nil {
			rows.Close()
			return 0, err
		}
		clones = append(clones, clone)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	for _, clone := range clones {
		result, err = tx.ExecContext(ctx, `INSERT INTO skus(tenant_id,store_id,product_id,name,attributes_json,price_cents,status) VALUES(?,?,?,?,?,?,?)`, tenantID, storeID, copyID, clone.Name, clone.AttributesJSON, clone.PriceCents, clone.Status)
		if err != nil {
			return 0, err
		}
		skuID, lastErr := result.LastInsertId()
		if lastErr != nil {
			return 0, lastErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO inventory(sku_id,tenant_id,store_id,stock,auto_sold_out,auto_refill,refill_stock) VALUES(?,?,?,?,?,?,?)`, skuID, tenantID, storeID, clone.Stock, clone.AutoSoldOut, clone.AutoRefill, clone.RefillStock); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO product_images(tenant_id,store_id,product_id,media_asset_id,url,is_primary,sort_order)
		SELECT tenant_id,store_id,?,media_asset_id,url,is_primary,sort_order FROM product_images
		WHERE product_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, copyID, productID, tenantID, storeID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO product_resource_bindings(tenant_id,store_id,product_id,resource_id,binding_type,sort_order)
		SELECT tenant_id,store_id,?,resource_id,binding_type,sort_order FROM product_resource_bindings WHERE product_id=? AND tenant_id=? AND store_id=?`, copyID, productID, tenantID, storeID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO product_modifier_groups(tenant_id,store_id,product_id,modifier_group_id,sort_order)
		SELECT tenant_id,store_id,?,modifier_group_id,sort_order FROM product_modifier_groups WHERE product_id=? AND tenant_id=? AND store_id=?`, copyID, productID, tenantID, storeID); err != nil {
		return 0, err
	}
	if err = copyProductOptionGroups(ctx, tx, tenantID, storeID, productID, copyID); err != nil {
		return 0, err
	}
	return copyID, nil
}

type optionGroupClone struct {
	ID               int64
	AttributeGroupID sql.NullInt64
	Name             string
	Kind             string
	SelectionMode    string
	MinSelect        int
	MaxSelect        int
	SortOrder        int
	Status           string
}

type optionValueClone struct {
	AttributeValueID sql.NullInt64
	Name             string
	PriceDeltaCents  int64
	IsDefault        bool
	SortOrder        int
	Status           string
}

func copyProductOptionGroups(ctx context.Context, tx *sql.Tx, tenantID, storeID, sourceProductID, copyProductID int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,attribute_group_id,name,kind,selection_mode,min_select,max_select,sort_order,status FROM product_option_groups
		WHERE product_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, sourceProductID, tenantID, storeID)
	if err != nil {
		return err
	}
	groups := []optionGroupClone{}
	for rows.Next() {
		var group optionGroupClone
		if err = rows.Scan(&group.ID, &group.AttributeGroupID, &group.Name, &group.Kind, &group.SelectionMode, &group.MinSelect, &group.MaxSelect, &group.SortOrder, &group.Status); err != nil {
			rows.Close()
			return err
		}
		groups = append(groups, group)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, group := range groups {
		valueRows, queryErr := tx.QueryContext(ctx, `SELECT attribute_value_id,name,price_delta_cents,is_default,sort_order,status FROM product_option_values
			WHERE group_id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id`, group.ID, tenantID, storeID)
		if queryErr != nil {
			return queryErr
		}
		values := []optionValueClone{}
		for valueRows.Next() {
			var value optionValueClone
			if queryErr = valueRows.Scan(&value.AttributeValueID, &value.Name, &value.PriceDeltaCents, &value.IsDefault, &value.SortOrder, &value.Status); queryErr != nil {
				valueRows.Close()
				return queryErr
			}
			values = append(values, value)
		}
		if queryErr = valueRows.Err(); queryErr != nil {
			valueRows.Close()
			return queryErr
		}
		valueRows.Close()
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO product_option_groups(tenant_id,store_id,product_id,attribute_group_id,name,kind,selection_mode,min_select,max_select,sort_order,status) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, tenantID, storeID, copyProductID, group.AttributeGroupID, group.Name, group.Kind, group.SelectionMode, group.MinSelect, group.MaxSelect, group.SortOrder, group.Status)
		if insertErr != nil {
			return insertErr
		}
		newGroupID, insertErr := result.LastInsertId()
		if insertErr != nil {
			return insertErr
		}
		for _, value := range values {
			if _, insertErr = tx.ExecContext(ctx, `INSERT INTO product_option_values(tenant_id,store_id,group_id,attribute_value_id,name,price_delta_cents,is_default,sort_order,status) VALUES(?,?,?,?,?,?,?,?,?)`, tenantID, storeID, newGroupID, value.AttributeValueID, value.Name, value.PriceDeltaCents, value.IsDefault, value.SortOrder, value.Status); insertErr != nil {
				return insertErr
			}
		}
	}
	return nil
}

func (s *Server) getProductStatistics(w http.ResponseWriter, r *http.Request) {
	productID, ok := pathID(w, r, "productID")
	if !ok {
		return
	}
	actor := currentIdentity(r.Context())
	var from, to *time.Time
	for key, target := range map[string]**time.Time{"from": &from, "to": &to} {
		raw := strings.TrimSpace(r.URL.Query().Get(key))
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", key+" must be an RFC3339 timestamp")
			return
		}
		parsed = parsed.UTC()
		*target = &parsed
	}
	if from != nil && to != nil && !from.Before(*to) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "from must be before to")
		return
	}
	var storeID int64
	if err := s.DB.QueryRowContext(r.Context(), `SELECT store_id FROM products WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, productID, actor.TenantID).Scan(&storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	query := `SELECT COUNT(DISTINCT o.id),COALESCE(SUM(oi.quantity),0),COALESCE(SUM(oi.subtotal_cents),0)
		FROM order_items oi JOIN orders o ON o.id=oi.order_id AND o.tenant_id=oi.tenant_id
		WHERE oi.tenant_id=? AND oi.product_id=? AND o.store_id=? AND o.payment_status IN ('PAID','PARTIALLY_REFUNDED','REFUNDED')`
	args := []any{actor.TenantID, productID, storeID}
	if from != nil {
		query += " AND o.paid_at>=?"
		args = append(args, *from)
	}
	if to != nil {
		query += " AND o.paid_at<?"
		args = append(args, *to)
	}
	var paidOrders, salesCount int
	var grossSales int64
	if err := s.DB.QueryRowContext(r.Context(), query, args...).Scan(&paidOrders, &salesCount, &grossSales); err != nil {
		handleSQLError(w, err)
		return
	}
	fromText, toText := "", ""
	if from != nil {
		fromText = from.Format(time.RFC3339)
	}
	if to != nil {
		toText = to.Format(time.RFC3339)
	}
	writeData(w, http.StatusOK, map[string]any{
		"product_id": productID, "paid_order_count": paidOrders, "sales_count": salesCount,
		"gross_sales_cents": grossSales, "from": fromText, "to": toText,
		"metric_scope": "PAID_ORDER_GROSS_BEFORE_REFUNDS",
	})
}
