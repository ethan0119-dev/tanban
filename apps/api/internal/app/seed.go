package app

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) SeedDemo(ctx context.Context) error {
	if !s.Config.SeedDemo {
		return nil
	}
	if s.Config.DemoMerchantUser == "" || len(s.Config.DemoMerchantPass) < 8 {
		return fmt.Errorf("TB_SEED_DEMO requires TB_DEMO_MERCHANT_USERNAME and TB_DEMO_MERCHANT_PASSWORD of at least 8 characters")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var tenantID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM tenants WHERE code='manong-coffee' AND deleted_at IS NULL").Scan(&tenantID)
	if err != nil {
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO tenants(code,name,contact_name,status,payment_provider) VALUES('manong-coffee','码农咖啡','店主','ACTIVE','mock')`)
		if insertErr != nil {
			return insertErr
		}
		tenantID, _ = result.LastInsertId()
	}
	var storeID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM stores WHERE code='manong-coffee' AND deleted_at IS NULL").Scan(&storeID)
	if err != nil {
		result, insertErr := tx.ExecContext(ctx, `INSERT INTO stores(tenant_id,code,name,business_hours,notice,status) VALUES(?,'manong-coffee','码农咖啡','18:00-24:00','SUV 后座咖啡摊，欢迎扫码点单','ACTIVE')`, tenantID)
		if insertErr != nil {
			return insertErr
		}
		storeID, _ = result.LastInsertId()
	}
	var categoryID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM categories WHERE tenant_id=? AND store_id=? AND name='咖啡' AND deleted_at IS NULL", tenantID, storeID).Scan(&categoryID)
	if err != nil {
		result, insertErr := tx.ExecContext(ctx, "INSERT INTO categories(tenant_id,store_id,name,sort_order,status) VALUES(?,?, '咖啡',10,'ACTIVE')", tenantID, storeID)
		if insertErr != nil {
			return insertErr
		}
		categoryID, _ = result.LastInsertId()
	}
	var productCount int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM products WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL", tenantID, storeID).Scan(&productCount); err != nil {
		return err
	}
	if productCount == 0 {
		products := []struct {
			name, description string
			price             int64
		}{
			{"美式咖啡", "清爽醇厚，适合夜市散步", 1200},
			{"拿铁", "浓缩咖啡与牛奶的平衡", 1600},
			{"生椰拿铁", "椰香与咖啡香气融合", 1800},
		}
		for index, product := range products {
			result, insertErr := tx.ExecContext(ctx, `INSERT INTO products(tenant_id,store_id,category_id,name,description,sort_order,status) VALUES(?,?,?,?,?,?,'ACTIVE')`, tenantID, storeID, categoryID, product.name, product.description, (index+1)*10)
			if insertErr != nil {
				return insertErr
			}
			productID, _ := result.LastInsertId()
			result, insertErr = tx.ExecContext(ctx, `INSERT INTO skus(tenant_id,store_id,product_id,name,attributes_json,price_cents,status) VALUES(?,?,?,'标准杯','{}',?,'ACTIVE')`, tenantID, storeID, productID, product.price)
			if insertErr != nil {
				return insertErr
			}
			skuID, _ := result.LastInsertId()
			if _, insertErr = tx.ExecContext(ctx, "INSERT INTO inventory(sku_id,tenant_id,store_id,stock,auto_sold_out) VALUES(?,?,?,99,1)", skuID, tenantID, storeID); insertErr != nil {
				return insertErr
			}
		}
	}
	var userCount int
	if err = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username=? AND deleted_at IS NULL", s.Config.DemoMerchantUser).Scan(&userCount); err != nil {
		return err
	}
	if userCount == 0 {
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(s.Config.DemoMerchantPass), bcrypt.DefaultCost)
		if hashErr != nil {
			return hashErr
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO users(tenant_id,username,password_hash,display_name,role,status) VALUES(?,?,?,'码农咖啡店主','MERCHANT_OWNER','ACTIVE')`, tenantID, s.Config.DemoMerchantUser, string(hash)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
