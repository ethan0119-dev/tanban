package app

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	websiteMediaMaxAssets = 500
	websiteMediaMaxBytes  = int64(512 * 1024 * 1024)
)

type websiteSettings struct {
	BrandName                string `json:"brandName"`
	BrandEnglishName         string `json:"brandEnglishName"`
	HeroEyebrow              string `json:"heroEyebrow"`
	HeroTitle                string `json:"heroTitle"`
	HeroHighlight            string `json:"heroHighlight"`
	HeroSubtitle             string `json:"heroSubtitle"`
	HeroImageURL             string `json:"heroImageUrl"`
	ScanOrderImageURL        string `json:"scanOrderImageUrl"`
	CashierImageURL          string `json:"cashierImageUrl"`
	KitchenImageURL          string `json:"kitchenImageUrl"`
	SceneBreakfastImageURL   string `json:"sceneBreakfastImageUrl"`
	SceneCoffeeTruckImageURL string `json:"sceneCoffeeTruckImageUrl"`
	SceneBakeryImageURL      string `json:"sceneBakeryImageUrl"`
	SceneNightMarketImageURL string `json:"sceneNightMarketImageUrl"`
	SceneCafeImageURL        string `json:"sceneCafeImageUrl"`
	SupportPhone             string `json:"supportPhone"`
	SupportEmail             string `json:"supportEmail"`
	ContactWechat            string `json:"contactWechat"`
	ContactQRURL             string `json:"contactQrUrl"`
	CompanyName              string `json:"companyName"`
	CompanyAddress           string `json:"companyAddress"`
	ICPNumber                string `json:"icpNumber"`
	FooterText               string `json:"footerText"`
	MerchantLoginURL         string `json:"merchantLoginUrl"`
	MetaTitle                string `json:"metaTitle"`
	MetaDescription          string `json:"metaDescription"`
}

type websiteArticleInput struct {
	Slug       string `json:"slug"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	CoverURL   string `json:"coverUrl"`
	Content    string `json:"content"`
	IsFeatured bool   `json:"isFeatured"`
}

type websiteArticleView struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	CoverURL    string `json:"coverUrl"`
	Content     string `json:"content"`
	Status      string `json:"status"`
	IsFeatured  bool   `json:"isFeatured"`
	PublishedAt string `json:"publishedAt"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type websiteMediaView struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	AltText    string `json:"altText"`
	URL        string `json:"url"`
	StorageKey string `json:"storageKey"`
	MimeType   string `json:"mimeType"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SizeBytes  int64  `json:"sizeBytes"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

func defaultWebsiteSettings() websiteSettings {
	return websiteSettings{
		BrandName:                "摊伴",
		BrandEnglishName:         "TANBAN",
		HeroEyebrow:              "为小店而生的数字化经营工具",
		HeroTitle:                "小店，也值得拥有一套",
		HeroHighlight:            "好用的经营系统。",
		HeroSubtitle:             "从顾客扫码点单、会员营销，到门店接单与平台管理，摊伴把日常经营需要的能力放进一套简单、顺手的系统。",
		HeroImageURL:             "/website/hero-devices.png",
		ScanOrderImageURL:        "/website/scan-ordering.png",
		CashierImageURL:          "/website/cashier-counter.png",
		KitchenImageURL:          "/website/kitchen-printer.png",
		SceneBreakfastImageURL:   "/website/scene-breakfast.png",
		SceneCoffeeTruckImageURL: "/website/scene-coffee-truck.png",
		SceneBakeryImageURL:      "/website/scene-bakery.png",
		SceneNightMarketImageURL: "/website/scene-night-market.png",
		SceneCafeImageURL:        "/website/scene-cafe.png",
		SupportPhone:             "400-865-0906",
		SupportEmail:             "hello@tanban.cn",
		ContactWechat:            "TanbanService",
		CompanyName:              "摊伴科技",
		CompanyAddress:           "中国 · 杭州",
		FooterText:               "让小生意，也有从容经营的底气。",
		MerchantLoginURL:         "https://b.tanban.com.cn/",
		MetaTitle:                "摊伴 TANBAN｜让每一个小摊，经营成一个好品牌",
		MetaDescription:          "面向咖啡摊、夜市餐饮与小型门店的一体化经营系统。",
	}
}

func (s *Server) registerWebsitePlatformRoutes(r chi.Router) {
	r.Get("/website/settings", s.getWebsiteSettings)
	r.Get("/website/articles", s.listWebsiteArticles)
	r.Get("/website/articles/{articleID}", s.getWebsiteArticle)
	r.Get("/website/media", s.listWebsiteMedia)
	r.Group(func(admin chi.Router) {
		admin.Use(requireRoles(RolePlatformAdmin))
		admin.Put("/website/settings", s.updateWebsiteSettings)
		admin.Post("/website/articles", s.createWebsiteArticle)
		admin.Put("/website/articles/{articleID}", s.updateWebsiteArticle)
		admin.Delete("/website/articles/{articleID}", s.deleteWebsiteArticle)
		admin.Post("/website/articles/{articleID}/publish", s.publishWebsiteArticle)
		admin.Post("/website/articles/{articleID}/withdraw", s.withdrawWebsiteArticle)
		admin.Post("/website/media/upload", s.uploadWebsiteMedia)
		admin.Delete("/website/media/{mediaID}", s.deleteWebsiteMedia)
	})
}

func (s *Server) registerPublicWebsiteRoutes(r chi.Router) {
	r.Get("/website", s.publicWebsite)
	r.Get("/website/articles", s.publicWebsiteArticles)
	r.Get("/website/articles/{slug}", s.publicWebsiteArticle)
}

func (s *Server) currentWebsiteSettings(r *http.Request) (websiteSettings, error) {
	settings := defaultWebsiteSettings()
	err := s.loadSettingJSON(r, "website", &settings)
	if errors.Is(err, sql.ErrNoRows) {
		return settings, nil
	}
	return settings, err
}

func (s *Server) getWebsiteSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentWebsiteSettings(r)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, settings)
}

func normalizeWebsiteSettings(input *websiteSettings) {
	input.BrandName = strings.TrimSpace(input.BrandName)
	input.BrandEnglishName = strings.TrimSpace(input.BrandEnglishName)
	input.HeroEyebrow = strings.TrimSpace(input.HeroEyebrow)
	input.HeroTitle = strings.TrimSpace(input.HeroTitle)
	input.HeroHighlight = strings.TrimSpace(input.HeroHighlight)
	input.HeroSubtitle = strings.TrimSpace(input.HeroSubtitle)
	input.HeroImageURL = strings.TrimSpace(input.HeroImageURL)
	input.ScanOrderImageURL = strings.TrimSpace(input.ScanOrderImageURL)
	input.CashierImageURL = strings.TrimSpace(input.CashierImageURL)
	input.KitchenImageURL = strings.TrimSpace(input.KitchenImageURL)
	input.SceneBreakfastImageURL = strings.TrimSpace(input.SceneBreakfastImageURL)
	input.SceneCoffeeTruckImageURL = strings.TrimSpace(input.SceneCoffeeTruckImageURL)
	input.SceneBakeryImageURL = strings.TrimSpace(input.SceneBakeryImageURL)
	input.SceneNightMarketImageURL = strings.TrimSpace(input.SceneNightMarketImageURL)
	input.SceneCafeImageURL = strings.TrimSpace(input.SceneCafeImageURL)
	input.SupportPhone = strings.TrimSpace(input.SupportPhone)
	input.SupportEmail = strings.TrimSpace(input.SupportEmail)
	input.ContactWechat = strings.TrimSpace(input.ContactWechat)
	input.ContactQRURL = strings.TrimSpace(input.ContactQRURL)
	input.CompanyName = strings.TrimSpace(input.CompanyName)
	input.CompanyAddress = strings.TrimSpace(input.CompanyAddress)
	input.ICPNumber = strings.TrimSpace(input.ICPNumber)
	input.FooterText = strings.TrimSpace(input.FooterText)
	input.MerchantLoginURL = strings.TrimSpace(input.MerchantLoginURL)
	input.MetaTitle = strings.TrimSpace(input.MetaTitle)
	input.MetaDescription = strings.TrimSpace(input.MetaDescription)
}

func validOptionalWebsiteURL(value string) bool {
	return value == "" || validDecorationURL(value)
}

func (s *Server) updateWebsiteSettings(w http.ResponseWriter, r *http.Request) {
	var input websiteSettings
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWebsiteSettings(&input)
	if !validRequiredText(input.BrandName, 80) ||
		!validRequiredText(input.BrandEnglishName, 40) ||
		!validRequiredText(input.HeroEyebrow, 100) ||
		!validRequiredText(input.HeroTitle, 120) ||
		!validRequiredText(input.HeroHighlight, 120) ||
		!validText(input.HeroSubtitle, 500) ||
		!validText(input.SupportPhone, 40) ||
		!validText(input.SupportEmail, 120) ||
		!validText(input.ContactWechat, 80) ||
		!validText(input.CompanyName, 120) ||
		!validText(input.CompanyAddress, 200) ||
		!validText(input.ICPNumber, 80) ||
		!validText(input.FooterText, 200) ||
		!validText(input.MetaTitle, 160) ||
		!validText(input.MetaDescription, 300) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "website setting text is missing or too long")
		return
	}
	for _, value := range []string{
		input.HeroImageURL,
		input.ScanOrderImageURL,
		input.CashierImageURL,
		input.KitchenImageURL,
		input.SceneBreakfastImageURL,
		input.SceneCoffeeTruckImageURL,
		input.SceneBakeryImageURL,
		input.SceneNightMarketImageURL,
		input.SceneCafeImageURL,
		input.ContactQRURL,
		input.MerchantLoginURL,
	} {
		if !validOptionalWebsiteURL(value) {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "website image and link URLs must be HTTP(S) URLs")
			return
		}
	}
	if err := s.saveSettingJSON(r, "website", input); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), currentIdentity(r.Context()), "website.settings.update", "website", "settings", map[string]any{"brandName": input.BrandName}, r)
	writeData(w, http.StatusOK, input)
}

func validWebsiteSlug(value string) bool {
	if value == "" || len(value) > 180 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-') {
			return false
		}
	}
	return !strings.HasPrefix(value, "-") && !strings.HasSuffix(value, "-") && !strings.Contains(value, "--")
}

func normalizeWebsiteArticle(input *websiteArticleInput) {
	input.Slug = strings.ToLower(strings.TrimSpace(input.Slug))
	input.Title = strings.TrimSpace(input.Title)
	input.Summary = strings.TrimSpace(input.Summary)
	input.CoverURL = strings.TrimSpace(input.CoverURL)
	input.Content = strings.TrimSpace(input.Content)
}

func validateWebsiteArticle(input websiteArticleInput) string {
	if !validWebsiteSlug(input.Slug) {
		return "slug must contain only lowercase letters, numbers and single hyphens"
	}
	if !validRequiredText(input.Title, 180) || !validText(input.Summary, 400) || !validRequiredText(input.Content, 50000) {
		return "title and content are required and article text is too long"
	}
	if input.CoverURL != "" && !validDecorationURL(input.CoverURL) {
		return "coverUrl must be an HTTP(S) image URL"
	}
	return ""
}

func scanWebsiteArticle(scanner interface{ Scan(...any) error }) (websiteArticleView, error) {
	var item websiteArticleView
	var publishedAt sql.NullTime
	var createdAt, updatedAt time.Time
	err := scanner.Scan(&item.ID, &item.Slug, &item.Title, &item.Summary, &item.CoverURL, &item.Content, &item.Status, &item.IsFeatured, &publishedAt, &createdAt, &updatedAt)
	if err != nil {
		return item, err
	}
	if publishedAt.Valid {
		item.PublishedAt = formatBeijingDateTime(publishedAt.Time)
	}
	item.CreatedAt = formatBeijingDateTime(createdAt)
	item.UpdatedAt = formatBeijingDateTime(updatedAt)
	return item, nil
}

const websiteArticleSelect = `SELECT id,slug,title,summary,cover_url,content_text,status,is_featured,published_at,created_at,updated_at FROM website_articles`

func (s *Server) listWebsiteArticles(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	search := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && !validStatus(status, "DRAFT", "PUBLISHED", "WITHDRAWN") {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid article status")
		return
	}
	var total int
	err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM website_articles WHERE deleted_at IS NULL AND (title LIKE ? OR summary LIKE ?) AND (?='' OR status=?)`, search, search, status, status).Scan(&total)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), websiteArticleSelect+` WHERE deleted_at IS NULL AND (title LIKE ? OR summary LIKE ?) AND (?='' OR status=?) ORDER BY created_at DESC LIMIT ? OFFSET ?`, search, search, status, status, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []websiteArticleView{}
	for rows.Next() {
		item, scanErr := scanWebsiteArticle(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func websiteArticleID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "articleID"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid article id")
	}
	return id, nil
}

func (s *Server) websiteArticleByID(r *http.Request, id int64) (websiteArticleView, error) {
	return scanWebsiteArticle(s.DB.QueryRowContext(r.Context(), websiteArticleSelect+` WHERE id=? AND deleted_at IS NULL`, id))
}

func (s *Server) getWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	id, err := websiteArticleID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	item, err := s.websiteArticleByID(r, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) createWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	var input websiteArticleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWebsiteArticle(&input)
	if message := validateWebsiteArticle(input); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO website_articles(slug,title,summary,cover_url,content_text,status,is_featured,created_by,updated_by) VALUES(?,?,?,?,?,'DRAFT',?,?,?)`, input.Slug, input.Title, input.Summary, input.CoverURL, input.Content, input.IsFeatured, identity.UserID, identity.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "website.article.create", "website_article", int64String(id), map[string]any{"title": input.Title, "slug": input.Slug}, r)
	item, err := s.websiteArticleByID(r, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusCreated, item)
}

func (s *Server) updateWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	id, err := websiteArticleID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var currentStatus string
	if err = s.DB.QueryRowContext(r.Context(), `SELECT status FROM website_articles WHERE id=? AND deleted_at IS NULL`, id).Scan(&currentStatus); err != nil {
		handleSQLError(w, err)
		return
	}
	if currentStatus == "PUBLISHED" {
		writeError(w, http.StatusConflict, "ARTICLE_PUBLISHED", "withdraw the article before editing")
		return
	}
	var input websiteArticleInput
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeWebsiteArticle(&input)
	if message := validateWebsiteArticle(input); message != "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
		return
	}
	identity := currentIdentity(r.Context())
	_, err = s.DB.ExecContext(r.Context(), `UPDATE website_articles SET slug=?,title=?,summary=?,cover_url=?,content_text=?,is_featured=?,status='DRAFT',updated_by=? WHERE id=? AND deleted_at IS NULL`, input.Slug, input.Title, input.Summary, input.CoverURL, input.Content, input.IsFeatured, identity.UserID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "website.article.update", "website_article", int64String(id), map[string]any{"title": input.Title}, r)
	item, err := s.websiteArticleByID(r, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) changeWebsiteArticleStatus(w http.ResponseWriter, r *http.Request, status string) {
	id, err := websiteArticleID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	var result sql.Result
	if status == "PUBLISHED" {
		result, err = s.DB.ExecContext(r.Context(), `UPDATE website_articles SET status='PUBLISHED',published_at=COALESCE(published_at,NOW(3)),updated_by=? WHERE id=? AND deleted_at IS NULL`, identity.UserID, id)
	} else {
		result, err = s.DB.ExecContext(r.Context(), `UPDATE website_articles SET status='WITHDRAWN',updated_by=? WHERE id=? AND deleted_at IS NULL`, identity.UserID, id)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "website article not found")
		return
	}
	s.audit(r.Context(), identity, "website.article."+strings.ToLower(status), "website_article", int64String(id), nil, r)
	item, err := s.websiteArticleByID(r, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) publishWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	s.changeWebsiteArticleStatus(w, r, "PUBLISHED")
}

func (s *Server) withdrawWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	s.changeWebsiteArticleStatus(w, r, "WITHDRAWN")
}

func (s *Server) deleteWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	id, err := websiteArticleID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	identity := currentIdentity(r.Context())
	result, err := s.DB.ExecContext(r.Context(), `UPDATE website_articles SET deleted_at=NOW(3),updated_by=? WHERE id=? AND deleted_at IS NULL AND status<>'PUBLISHED'`, identity.UserID, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusConflict, "DELETE_NOT_ALLOWED", "published articles must be withdrawn before deletion")
		return
	}
	s.audit(r.Context(), identity, "website.article.delete", "website_article", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) publicWebsite(w http.ResponseWriter, r *http.Request) {
	settings, err := s.currentWebsiteSettings(r)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), websiteArticleSelect+` WHERE status='PUBLISHED' AND deleted_at IS NULL ORDER BY is_featured DESC,published_at DESC LIMIT 6`)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	articles := []websiteArticleView{}
	for rows.Next() {
		item, scanErr := scanWebsiteArticle(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		articles = append(articles, item)
	}
	writeData(w, http.StatusOK, map[string]any{"settings": settings, "articles": articles})
}

func (s *Server) publicWebsiteArticles(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM website_articles WHERE status='PUBLISHED' AND deleted_at IS NULL`).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), websiteArticleSelect+` WHERE status='PUBLISHED' AND deleted_at IS NULL ORDER BY is_featured DESC,published_at DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []websiteArticleView{}
	for rows.Next() {
		item, scanErr := scanWebsiteArticle(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) publicWebsiteArticle(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(strings.TrimSpace(chi.URLParam(r, "slug")))
	if !validWebsiteSlug(slug) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "website article not found")
		return
	}
	item, err := scanWebsiteArticle(s.DB.QueryRowContext(r.Context(), websiteArticleSelect+` WHERE slug=? AND status='PUBLISHED' AND deleted_at IS NULL`, slug))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) listWebsiteMedia(w http.ResponseWriter, r *http.Request) {
	page, size, offset := pagination(r)
	var total int
	if err := s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM website_media_assets WHERE status='ACTIVE' AND deleted_at IS NULL`).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,alt_text,url,storage_key,mime_type,width,height,size_bytes,status,created_at FROM website_media_assets WHERE status='ACTIVE' AND deleted_at IS NULL ORDER BY id DESC LIMIT ? OFFSET ?`, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []websiteMediaView{}
	for rows.Next() {
		var item websiteMediaView
		var createdAt time.Time
		if err = rows.Scan(&item.ID, &item.Name, &item.AltText, &item.URL, &item.StorageKey, &item.MimeType, &item.Width, &item.Height, &item.SizeBytes, &item.Status, &createdAt); err != nil {
			handleSQLError(w, err)
			return
		}
		item.CreatedAt = formatBeijingDateTime(createdAt)
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) uploadWebsiteMedia(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, mediaMaxMultipartBytes)
	if err := r.ParseMultipartForm(mediaMaxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MULTIPART", "a multipart form with an image file is required")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "FILE_REQUIRED", "multipart field file is required")
		return
	}
	defer file.Close()
	imageFile, err := inspectUploadedImage(file)
	if err != nil {
		status := http.StatusUnsupportedMediaType
		if errors.Is(err, errMediaUploadTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, "INVALID_IMAGE", err.Error())
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strings.TrimSpace(filepath.Base(header.Filename))
	}
	if name == "" {
		name = "官网图片"
	}
	altText := strings.TrimSpace(r.FormValue("alt_text"))
	if !validRequiredText(name, 120) || !validText(altText, 200) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "image name or alt text is too long")
		return
	}
	var count int
	var totalBytes int64
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*),COALESCE(SUM(size_bytes),0) FROM website_media_assets WHERE status='ACTIVE' AND deleted_at IS NULL`).Scan(&count, &totalBytes); err != nil {
		handleSQLError(w, err)
		return
	}
	if count >= websiteMediaMaxAssets || totalBytes+int64(len(imageFile.Data)) > websiteMediaMaxBytes {
		writeError(w, http.StatusInsufficientStorage, "MEDIA_QUOTA_EXCEEDED", "website image library quota has been reached")
		return
	}
	storageKey, err := newWebsiteMediaStorageKey(imageFile.Extension, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to allocate image storage")
		return
	}
	publicURL, err := mediaPublicURL(s.Config.MediaPublicBaseURL, storageKey)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "MEDIA_STORAGE_NOT_CONFIGURED", "media public URL is not configured")
		return
	}
	storedPath, err := persistUploadedImage(s.Config.MediaStorageDir, storageKey, imageFile.Data)
	if err != nil {
		s.Logger.Error("persist website media", "error", err)
		writeError(w, http.StatusInternalServerError, "MEDIA_STORAGE_ERROR", "failed to persist image")
		return
	}
	removeStoredFile := true
	defer func() {
		if removeStoredFile {
			_ = os.Remove(storedPath)
		}
	}()
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO website_media_assets(name,alt_text,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,?,?,?,?,?,'ACTIVE',?)`, name, altText, publicURL, storageKey, imageFile.MimeType, imageFile.Width, imageFile.Height, len(imageFile.Data), identity.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	removeStoredFile = false
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "website.media.upload", "website_media", int64String(id), map[string]any{"name": name, "storage_key": storageKey}, r)
	writeData(w, http.StatusCreated, websiteMediaView{ID: id, Name: name, AltText: altText, URL: publicURL, StorageKey: storageKey, MimeType: imageFile.MimeType, Width: imageFile.Width, Height: imageFile.Height, SizeBytes: int64(len(imageFile.Data)), Status: "ACTIVE", CreatedAt: formatBeijingDateTime(time.Now())})
}

func (s *Server) deleteWebsiteMedia(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "mediaID"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid media id")
		return
	}
	var imageURL string
	if err = s.DB.QueryRowContext(r.Context(), `SELECT url FROM website_media_assets WHERE id=? AND status='ACTIVE' AND deleted_at IS NULL`, id).Scan(&imageURL); err != nil {
		handleSQLError(w, err)
		return
	}
	var articleReferences int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM website_articles WHERE deleted_at IS NULL AND cover_url=?`, imageURL).Scan(&articleReferences); err != nil {
		handleSQLError(w, err)
		return
	}
	var settingReferences int
	pattern := "%" + strings.ReplaceAll(imageURL, "%", "\\%") + "%"
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM platform_settings WHERE setting_key='website' AND value_text LIKE ?`, pattern).Scan(&settingReferences); err != nil {
		handleSQLError(w, err)
		return
	}
	if articleReferences+settingReferences > 0 {
		writeError(w, http.StatusConflict, "MEDIA_IN_USE", "the image is currently used by the website")
		return
	}
	identity := currentIdentity(r.Context())
	_, err = s.DB.ExecContext(r.Context(), `UPDATE website_media_assets SET status='DELETED',deleted_at=NOW(3) WHERE id=? AND deleted_at IS NULL`, id)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "website.media.delete", "website_media", int64String(id), nil, r)
	w.WriteHeader(http.StatusNoContent)
}
