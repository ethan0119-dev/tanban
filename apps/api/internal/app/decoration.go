package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	decorationSchemaVersion = 1
	decorationMaxBytes      = 256 * 1024
	decorationMaxModules    = 30
	decorationMaxBanners    = 8
)

var decorationHexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// DecorationConfig is an intentionally constrained mini-program appearance
// schema. It never accepts arbitrary markup, script, style sheets or routes.
// That keeps merchant-authored decoration safe to render in the customer app.
type DecorationConfig struct {
	SchemaVersion int                  `json:"schemaVersion"`
	TemplateKey   string               `json:"templateKey"`
	Theme         DecorationTheme      `json:"theme"`
	Home          DecorationHome       `json:"home"`
	Menu          DecorationMenu       `json:"menu"`
	Navigation    DecorationNavigation `json:"navigation"`
	Splash        DecorationSplash     `json:"splash"`
}

type DecorationTheme struct {
	PrimaryColor       string `json:"primaryColor"`
	AccentColor        string `json:"accentColor"`
	BackgroundColor    string `json:"backgroundColor"`
	SurfaceColor       string `json:"surfaceColor"`
	TextColor          string `json:"textColor"`
	MutedColor         string `json:"mutedColor"`
	NavBackgroundColor string `json:"navBackgroundColor"`
	NavTextColor       string `json:"navTextColor"`
	NavSelectedColor   string `json:"navSelectedColor"`
	Radius             string `json:"radius"`
}

type DecorationHome struct {
	Modules []DecorationModule `json:"modules"`
}

type DecorationModule struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Enabled   bool            `json:"enabled"`
	SortOrder int             `json:"sortOrder"`
	Config    json.RawMessage `json:"config"`
}

type DecorationMenu struct {
	CategoryLayout    string `json:"categoryLayout"`
	ProductLayout     string `json:"productLayout"`
	ShowDescription   bool   `json:"showDescription"`
	ShowStock         bool   `json:"showStock"`
	ShowSales         bool   `json:"showSales"`
	ShowSoldOut       bool   `json:"showSoldOut"`
	LoadMode          string `json:"loadMode"`
	ProductActionMode string `json:"productActionMode"`
	Density           string `json:"density"`
}

type DecorationNavigation struct {
	Items           []DecorationNavigationItem `json:"items"`
	BackgroundColor string                     `json:"backgroundColor"`
	TextColor       string                     `json:"textColor"`
	SelectedColor   string                     `json:"selectedColor"`
}

type DecorationNavigationItem struct {
	Key       string `json:"key"`
	Text      string `json:"text"`
	Visible   bool   `json:"visible"`
	SortOrder int    `json:"sortOrder"`
}

type DecorationSplash struct {
	Enabled          bool             `json:"enabled"`
	DisplayMode      string           `json:"displayMode"`
	ImageURL         string           `json:"imageUrl"`
	Title            string           `json:"title"`
	Subtitle         string           `json:"subtitle"`
	AutoCloseSeconds int              `json:"autoCloseSeconds"`
	Action           DecorationAction `json:"action"`
	Frequency        string           `json:"frequency"`
	ActiveFrom       string           `json:"activeFrom,omitempty"`
	ActiveTo         string           `json:"activeTo,omitempty"`
}

type DecorationAction struct {
	Type  string `json:"type"`
	Phone string `json:"phone,omitempty"`
}

type decorationHeroConfig struct {
	Items []struct {
		ImageURL string           `json:"imageUrl"`
		Title    string           `json:"title"`
		Subtitle string           `json:"subtitle"`
		Action   DecorationAction `json:"action"`
	} `json:"items"`
}

type decorationStoreHeaderConfig struct {
	ShowLogo    bool `json:"showLogo"`
	ShowStatus  bool `json:"showStatus"`
	ShowAddress bool `json:"showAddress"`
}

type decorationAnnouncementConfig struct {
	Prefix string `json:"prefix"`
}

type decorationQuickActionsConfig struct {
	Items []struct {
		Title    string           `json:"title"`
		Subtitle string           `json:"subtitle"`
		Action   DecorationAction `json:"action"`
	} `json:"items"`
}

type decorationImageConfig struct {
	ImageURL string           `json:"imageUrl"`
	Alt      string           `json:"alt"`
	Action   DecorationAction `json:"action"`
}

type decorationTextConfig struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Align string `json:"align"`
}

type decorationSpacerConfig struct {
	Height int `json:"height"`
}

type decorationDraftInput struct {
	ExpectedRevision int64            `json:"expectedRevision"`
	Config           DecorationConfig `json:"config"`
}

type decorationPublishInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Note             string `json:"note"`
}

type decorationRollbackInput struct {
	ExpectedRevision int64  `json:"expectedRevision"`
	Note             string `json:"note"`
}

type decorationPublishedView struct {
	ID          int64            `json:"id"`
	VersionNo   int              `json:"versionNo"`
	Config      DecorationConfig `json:"config"`
	Note        string           `json:"note"`
	PublishedAt string           `json:"publishedAt"`
}

type decorationDraftView struct {
	Revision  int64            `json:"revision"`
	Config    DecorationConfig `json:"config"`
	UpdatedAt string           `json:"updatedAt,omitempty"`
}

type decorationView struct {
	StoreName string                   `json:"storeName"`
	Draft     decorationDraftView      `json:"draft"`
	Published *decorationPublishedView `json:"published"`
}

type mediaAssetInput struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	StorageKey string `json:"storageKey"`
	MimeType   string `json:"mimeType"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SizeBytes  int64  `json:"sizeBytes"`
}

type mediaAssetView struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	URL        string `json:"url"`
	StorageKey string `json:"storageKey"`
	MimeType   string `json:"mimeType"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	SizeBytes  int64  `json:"sizeBytes"`
	Status     string `json:"status"`
	CreatedAt  string `json:"createdAt"`
}

func defaultDecorationConfig(store storeDTO) DecorationConfig {
	theme := DecorationTheme{
		PrimaryColor: "#214d3f", AccentColor: "#dff06d",
		BackgroundColor: "#f6f5f0", SurfaceColor: "#fffefa",
		TextColor: "#17201b", MutedColor: "#747b75",
		NavBackgroundColor: "#fffefa", NavTextColor: "#7b807a",
		NavSelectedColor: "#214d3f", Radius: "LG",
	}
	hero := decorationHeroConfig{}
	if validHTTPSURL(store.BannerURL) {
		hero.Items = append(hero.Items, struct {
			ImageURL string           `json:"imageUrl"`
			Title    string           `json:"title"`
			Subtitle string           `json:"subtitle"`
			Action   DecorationAction `json:"action"`
		}{ImageURL: store.BannerURL, Action: DecorationAction{Type: "OPEN_MENU"}})
	}
	quick := decorationQuickActionsConfig{}
	quick.Items = append(quick.Items,
		struct {
			Title    string           `json:"title"`
			Subtitle string           `json:"subtitle"`
			Action   DecorationAction `json:"action"`
		}{Title: "堂食 / 自提点单", Subtitle: "选好口味，在线下单", Action: DecorationAction{Type: "OPEN_MENU"}},
		struct {
			Title    string           `json:"title"`
			Subtitle string           `json:"subtitle"`
			Action   DecorationAction `json:"action"`
		}{Title: "查看我的订单", Subtitle: "支付与制作进度", Action: DecorationAction{Type: "OPEN_ORDERS"}},
	)
	return DecorationConfig{
		SchemaVersion: decorationSchemaVersion,
		TemplateKey:   "coffee-light",
		Theme:         theme,
		Home: DecorationHome{Modules: []DecorationModule{
			decorationModule("hero", "HERO_BANNER", 10, hero),
			decorationModule("store", "STORE_HEADER", 20, decorationStoreHeaderConfig{ShowLogo: true, ShowStatus: true, ShowAddress: true}),
			decorationModule("notice", "ANNOUNCEMENT", 30, decorationAnnouncementConfig{Prefix: "公告"}),
			decorationModule("quick-actions", "QUICK_ACTIONS", 40, quick),
		}},
		Menu: DecorationMenu{
			CategoryLayout: "LEFT", ProductLayout: "LIST", ShowDescription: true,
			ShowSoldOut: true, LoadMode: "BY_CATEGORY", ProductActionMode: "SKU_SHEET", Density: "COMFORTABLE",
		},
		Navigation: DecorationNavigation{
			BackgroundColor: theme.NavBackgroundColor, TextColor: theme.NavTextColor, SelectedColor: theme.NavSelectedColor,
			Items: []DecorationNavigationItem{
				{Key: "home", Text: "首页", Visible: true, SortOrder: 10},
				{Key: "menu", Text: "点单", Visible: true, SortOrder: 20},
				{Key: "orders", Text: "订单", Visible: true, SortOrder: 30},
				{Key: "profile", Text: "我的", Visible: true, SortOrder: 40},
			},
		},
		Splash: DecorationSplash{
			DisplayMode: "POPUP", AutoCloseSeconds: 5,
			Action: DecorationAction{Type: "NONE"}, Frequency: "ONCE_PER_VERSION",
		},
	}
}

func decorationModule(id, moduleType string, sortOrder int, config any) DecorationModule {
	body, _ := json.Marshal(config)
	return DecorationModule{ID: id, Type: moduleType, Enabled: true, SortOrder: sortOrder, Config: body}
}

func normalizeDecorationConfig(config *DecorationConfig) {
	defaults := defaultDecorationConfig(storeDTO{})
	if config.SchemaVersion == 0 {
		config.SchemaVersion = decorationSchemaVersion
	}
	if strings.TrimSpace(config.TemplateKey) == "" {
		config.TemplateKey = defaults.TemplateKey
	}
	fill := func(value *string, fallback string) {
		if strings.TrimSpace(*value) == "" {
			*value = fallback
		}
	}
	fill(&config.Theme.PrimaryColor, defaults.Theme.PrimaryColor)
	fill(&config.Theme.AccentColor, defaults.Theme.AccentColor)
	fill(&config.Theme.BackgroundColor, defaults.Theme.BackgroundColor)
	fill(&config.Theme.SurfaceColor, defaults.Theme.SurfaceColor)
	fill(&config.Theme.TextColor, defaults.Theme.TextColor)
	fill(&config.Theme.MutedColor, defaults.Theme.MutedColor)
	fill(&config.Theme.NavBackgroundColor, defaults.Theme.NavBackgroundColor)
	fill(&config.Theme.NavTextColor, defaults.Theme.NavTextColor)
	fill(&config.Theme.NavSelectedColor, defaults.Theme.NavSelectedColor)
	fill(&config.Theme.Radius, defaults.Theme.Radius)
	fill(&config.Menu.CategoryLayout, defaults.Menu.CategoryLayout)
	fill(&config.Menu.ProductLayout, defaults.Menu.ProductLayout)
	fill(&config.Menu.LoadMode, defaults.Menu.LoadMode)
	fill(&config.Menu.ProductActionMode, defaults.Menu.ProductActionMode)
	fill(&config.Menu.Density, defaults.Menu.Density)
	fill(&config.Navigation.BackgroundColor, config.Theme.NavBackgroundColor)
	fill(&config.Navigation.TextColor, config.Theme.NavTextColor)
	fill(&config.Navigation.SelectedColor, config.Theme.NavSelectedColor)
	if len(config.Navigation.Items) == 0 {
		config.Navigation.Items = defaults.Navigation.Items
	}
	fill(&config.Splash.DisplayMode, defaults.Splash.DisplayMode)
	fill(&config.Splash.Frequency, defaults.Splash.Frequency)
	if config.Splash.Action.Type == "" {
		config.Splash.Action.Type = "NONE"
	}
	sort.SliceStable(config.Home.Modules, func(i, j int) bool { return config.Home.Modules[i].SortOrder < config.Home.Modules[j].SortOrder })
	sort.SliceStable(config.Navigation.Items, func(i, j int) bool {
		return config.Navigation.Items[i].SortOrder < config.Navigation.Items[j].SortOrder
	})
}

func validateDecorationConfig(config DecorationConfig) error {
	if config.SchemaVersion != decorationSchemaVersion {
		return fmt.Errorf("schemaVersion must be %d", decorationSchemaVersion)
	}
	if !validText(config.TemplateKey, 64) {
		return errors.New("templateKey must contain at most 64 characters")
	}
	for name, color := range map[string]string{
		"primaryColor": config.Theme.PrimaryColor, "accentColor": config.Theme.AccentColor,
		"backgroundColor": config.Theme.BackgroundColor, "surfaceColor": config.Theme.SurfaceColor,
		"textColor": config.Theme.TextColor, "mutedColor": config.Theme.MutedColor,
		"navBackgroundColor": config.Theme.NavBackgroundColor, "navTextColor": config.Theme.NavTextColor,
		"navSelectedColor": config.Theme.NavSelectedColor,
	} {
		if !decorationHexColor.MatchString(color) {
			return fmt.Errorf("theme.%s must be a #RRGGBB color", name)
		}
	}
	if !oneOf(config.Theme.Radius, "SM", "MD", "LG") {
		return errors.New("theme.radius must be SM, MD or LG")
	}
	if len(config.Home.Modules) > decorationMaxModules {
		return fmt.Errorf("home modules cannot exceed %d", decorationMaxModules)
	}
	moduleIDs := map[string]bool{}
	for index, module := range config.Home.Modules {
		if !validIdentifier(module.ID, 64) || moduleIDs[module.ID] {
			return fmt.Errorf("home.modules[%d].id is invalid or duplicated", index)
		}
		moduleIDs[module.ID] = true
		if err := validateDecorationModule(module); err != nil {
			return fmt.Errorf("home.modules[%d]: %w", index, err)
		}
	}
	if !oneOf(config.Menu.CategoryLayout, "LEFT", "TOP") {
		return errors.New("menu.categoryLayout must be LEFT or TOP")
	}
	if !oneOf(config.Menu.ProductLayout, "LIST", "GRID") {
		return errors.New("menu.productLayout must be LIST or GRID")
	}
	if !oneOf(config.Menu.LoadMode, "BY_CATEGORY", "ALL") {
		return errors.New("menu.loadMode must be BY_CATEGORY or ALL")
	}
	if !oneOf(config.Menu.ProductActionMode, "SKU_SHEET", "DIRECT_ADD") {
		return errors.New("menu.productActionMode must be SKU_SHEET or DIRECT_ADD")
	}
	if !oneOf(config.Menu.Density, "COMPACT", "COMFORTABLE") {
		return errors.New("menu.density must be COMPACT or COMFORTABLE")
	}
	if len(config.Navigation.Items) < 2 || len(config.Navigation.Items) > 4 {
		return errors.New("navigation must contain between 2 and 4 items")
	}
	for name, color := range map[string]string{"backgroundColor": config.Navigation.BackgroundColor, "textColor": config.Navigation.TextColor, "selectedColor": config.Navigation.SelectedColor} {
		if !decorationHexColor.MatchString(color) {
			return fmt.Errorf("navigation.%s must be a #RRGGBB color", name)
		}
	}
	navKeys := map[string]bool{}
	for index, item := range config.Navigation.Items {
		if !oneOf(item.Key, "home", "menu", "orders", "profile") || navKeys[item.Key] {
			return fmt.Errorf("navigation.items[%d].key is invalid or duplicated", index)
		}
		if !validRequiredText(item.Text, 8) {
			return fmt.Errorf("navigation.items[%d].text is required and limited to 8 characters", index)
		}
		navKeys[item.Key] = true
	}
	if !navKeys["home"] || !navKeys["menu"] {
		return errors.New("navigation must contain home and menu")
	}
	if !oneOf(config.Splash.DisplayMode, "FULLSCREEN", "POPUP") {
		return errors.New("splash.displayMode must be FULLSCREEN or POPUP")
	}
	if !oneOf(config.Splash.Frequency, "EVERY_VISIT", "DAILY", "ONCE_PER_VERSION") {
		return errors.New("splash.frequency is invalid")
	}
	if config.Splash.AutoCloseSeconds < 0 || config.Splash.AutoCloseSeconds > 30 {
		return errors.New("splash.autoCloseSeconds must be between 0 and 30")
	}
	if config.Splash.Enabled && !validHTTPSURL(config.Splash.ImageURL) {
		return errors.New("splash.imageUrl must be an HTTPS URL when splash is enabled")
	}
	if config.Splash.ImageURL != "" && !validHTTPSURL(config.Splash.ImageURL) {
		return errors.New("splash.imageUrl must be an HTTPS URL")
	}
	if !validText(config.Splash.Title, 60) || !validText(config.Splash.Subtitle, 160) {
		return errors.New("splash text is too long")
	}
	if err := validateDecorationAction(config.Splash.Action); err != nil {
		return fmt.Errorf("splash.action: %w", err)
	}
	from, err := parseDecorationTime(config.Splash.ActiveFrom)
	if err != nil {
		return fmt.Errorf("splash.activeFrom: %w", err)
	}
	to, err := parseDecorationTime(config.Splash.ActiveTo)
	if err != nil {
		return fmt.Errorf("splash.activeTo: %w", err)
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return errors.New("splash.activeFrom cannot be after activeTo")
	}
	body, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if len(body) > decorationMaxBytes {
		return fmt.Errorf("decoration config cannot exceed %d bytes", decorationMaxBytes)
	}
	return nil
}

func validateDecorationModule(module DecorationModule) error {
	if len(module.Config) == 0 || string(module.Config) == "null" {
		return errors.New("config is required")
	}
	switch module.Type {
	case "HERO_BANNER":
		var config decorationHeroConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if len(config.Items) > decorationMaxBanners {
			return fmt.Errorf("banner items cannot exceed %d", decorationMaxBanners)
		}
		for _, item := range config.Items {
			if !validHTTPSURL(item.ImageURL) {
				return errors.New("banner imageUrl must be an HTTPS URL")
			}
			if !validText(item.Title, 60) || !validText(item.Subtitle, 160) {
				return errors.New("banner text is too long")
			}
			if err := validateDecorationAction(item.Action); err != nil {
				return err
			}
		}
	case "STORE_HEADER":
		var config decorationStoreHeaderConfig
		return strictRawJSON(module.Config, &config)
	case "ANNOUNCEMENT":
		var config decorationAnnouncementConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validText(config.Prefix, 16) {
			return errors.New("announcement prefix is too long")
		}
	case "QUICK_ACTIONS":
		var config decorationQuickActionsConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if len(config.Items) < 1 || len(config.Items) > 4 {
			return errors.New("quick actions must contain between 1 and 4 items")
		}
		for _, item := range config.Items {
			if !validRequiredText(item.Title, 30) || !validText(item.Subtitle, 80) {
				return errors.New("quick action text is invalid")
			}
			if err := validateDecorationAction(item.Action); err != nil {
				return err
			}
		}
	case "IMAGE":
		var config decorationImageConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validHTTPSURL(config.ImageURL) || !validText(config.Alt, 80) {
			return errors.New("image module requires a valid HTTPS imageUrl and short alt text")
		}
		return validateDecorationAction(config.Action)
	case "TEXT":
		var config decorationTextConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validText(config.Title, 80) || !validText(config.Body, 500) || !oneOf(config.Align, "LEFT", "CENTER", "RIGHT") {
			return errors.New("text module content or alignment is invalid")
		}
	case "SPACER":
		var config decorationSpacerConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if config.Height < 4 || config.Height > 160 {
			return errors.New("spacer height must be between 4 and 160")
		}
	default:
		return errors.New("unsupported module type")
	}
	return nil
}

func validateDecorationAction(action DecorationAction) error {
	if !oneOf(action.Type, "NONE", "OPEN_MENU", "OPEN_ORDERS", "OPEN_PROFILE", "CALL_PHONE") {
		return errors.New("unsupported action type")
	}
	if action.Type == "CALL_PHONE" {
		phone := strings.NewReplacer(" ", "", "-", "").Replace(action.Phone)
		if len(phone) < 5 || len(phone) > 20 {
			return errors.New("CALL_PHONE requires a valid phone")
		}
	} else if action.Phone != "" {
		return errors.New("phone is only allowed for CALL_PHONE")
	}
	return nil
}

func strictRawJSON(body json.RawMessage, target any) error {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid module config: %w", err)
	}
	return nil
}

func validIdentifier(value string, limit int) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

func validRequiredText(value string, limit int) bool {
	return strings.TrimSpace(value) != "" && validText(value, limit)
}

func validText(value string, limit int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= limit
}

func validHTTPSURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func parseDecorationTime(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("must be an RFC3339 timestamp or YYYY-MM-DD")
}

func (s *Server) getDecoration(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	view, err := s.loadDecorationView(r.Context(), store)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, view)
}

func (s *Server) saveDecorationDraft(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input decorationDraftInput
	if !decodeJSON(w, r, &input) {
		return
	}
	normalizeDecorationConfig(&input.Config)
	if err := validateDecorationConfig(input.Config); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_DECORATION", err.Error())
		return
	}
	body, _ := json.Marshal(input.Config)
	var newRevision int64
	if input.ExpectedRevision == 0 {
		result, insertErr := s.DB.ExecContext(r.Context(), `INSERT INTO store_decorations(tenant_id,store_id,schema_version,draft_json,draft_revision,created_by,updated_by) VALUES(?,?,?,?,1,?,?)`, identity.TenantID, store.ID, input.Config.SchemaVersion, string(body), identity.UserID, identity.UserID)
		if insertErr != nil {
			if strings.Contains(insertErr.Error(), "1062") {
				writeError(w, http.StatusConflict, "DRAFT_CONFLICT", "decoration draft changed; reload before saving")
				return
			}
			handleSQLError(w, insertErr)
			return
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			writeError(w, http.StatusConflict, "DRAFT_CONFLICT", "decoration draft changed; reload before saving")
			return
		}
		newRevision = 1
	} else {
		result, updateErr := s.DB.ExecContext(r.Context(), `UPDATE store_decorations SET schema_version=?,draft_json=?,draft_revision=draft_revision+1,updated_by=? WHERE tenant_id=? AND store_id=? AND draft_revision=?`, input.Config.SchemaVersion, string(body), identity.UserID, identity.TenantID, store.ID, input.ExpectedRevision)
		if updateErr != nil {
			handleSQLError(w, updateErr)
			return
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			writeError(w, http.StatusConflict, "DRAFT_CONFLICT", "decoration draft changed; reload before saving")
			return
		}
		newRevision = input.ExpectedRevision + 1
	}
	s.audit(r.Context(), identity, "decoration.draft.save", "store_decoration", int64String(store.ID), map[string]any{"revision": newRevision}, r)
	writeData(w, http.StatusOK, decorationDraftView{Revision: newRevision, Config: input.Config})
}

func (s *Server) publishDecoration(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input decorationPublishInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if !validText(input.Note, 255) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "publish note is too long")
		return
	}
	published, err := s.publishDecorationVersion(r.Context(), identity, store.ID, input.ExpectedRevision, input.Note, 0, "")
	if errors.Is(err, errDecorationConflict) {
		writeError(w, http.StatusConflict, "DRAFT_CONFLICT", "decoration draft changed; reload before publishing")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "decoration.publish", "store_decoration_version", int64String(published.ID), map[string]any{"versionNo": published.VersionNo}, r)
	writeData(w, http.StatusCreated, published)
}

var errDecorationConflict = errors.New("decoration draft conflict")

func (s *Server) publishDecorationVersion(ctx context.Context, actor identity, storeID, expectedRevision int64, note string, sourceVersionID int64, sourceJSON string) (decorationPublishedView, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return decorationPublishedView{}, err
	}
	defer tx.Rollback()
	var draftJSON string
	var revision int64
	if err = tx.QueryRowContext(ctx, `SELECT draft_json,draft_revision FROM store_decorations WHERE tenant_id=? AND store_id=? FOR UPDATE`, actor.TenantID, storeID).Scan(&draftJSON, &revision); err != nil {
		return decorationPublishedView{}, err
	}
	if revision != expectedRevision {
		return decorationPublishedView{}, errDecorationConflict
	}
	if sourceJSON != "" {
		draftJSON = sourceJSON
	}
	var config DecorationConfig
	if err = json.Unmarshal([]byte(draftJSON), &config); err != nil {
		return decorationPublishedView{}, err
	}
	normalizeDecorationConfig(&config)
	if err = validateDecorationConfig(config); err != nil {
		return decorationPublishedView{}, err
	}
	canonical, _ := json.Marshal(config)
	var versionNo int
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM store_decoration_versions WHERE tenant_id=? AND store_id=?`, actor.TenantID, storeID).Scan(&versionNo); err != nil {
		return decorationPublishedView{}, err
	}
	var source any
	if sourceVersionID > 0 {
		source = sourceVersionID
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO store_decoration_versions(tenant_id,store_id,version_no,schema_version,config_json,publish_note,source_version_id,published_by) VALUES(?,?,?,?,?,?,?,?)`, actor.TenantID, storeID, versionNo, config.SchemaVersion, string(canonical), note, source, actor.UserID)
	if err != nil {
		return decorationPublishedView{}, err
	}
	versionID, err := result.LastInsertId()
	if err != nil {
		return decorationPublishedView{}, err
	}
	if sourceJSON == "" {
		_, err = tx.ExecContext(ctx, `UPDATE store_decorations SET published_version_id=?,updated_by=? WHERE tenant_id=? AND store_id=? AND draft_revision=?`, versionID, actor.UserID, actor.TenantID, storeID, revision)
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE store_decorations SET schema_version=?,draft_json=?,draft_revision=draft_revision+1,published_version_id=?,updated_by=? WHERE tenant_id=? AND store_id=? AND draft_revision=?`, config.SchemaVersion, string(canonical), versionID, actor.UserID, actor.TenantID, storeID, revision)
	}
	if err != nil {
		return decorationPublishedView{}, err
	}
	if err = tx.Commit(); err != nil {
		return decorationPublishedView{}, err
	}
	return decorationPublishedView{ID: versionID, VersionNo: versionNo, Config: config, Note: note, PublishedAt: time.Now().UTC().Format(time.RFC3339)}, nil
}

func (s *Server) listDecorationVersions(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	var total int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM store_decoration_versions WHERE tenant_id=? AND store_id=?`, identity.TenantID, store.ID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,version_no,config_json,publish_note,DATE_FORMAT(published_at,'%Y-%m-%dT%H:%i:%sZ') FROM store_decoration_versions WHERE tenant_id=? AND store_id=? ORDER BY version_no DESC LIMIT ? OFFSET ?`, identity.TenantID, store.ID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []decorationPublishedView{}
	for rows.Next() {
		item, scanErr := scanDecorationVersion(rows)
		if scanErr != nil {
			handleSQLError(w, scanErr)
			return
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) getDecorationVersion(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	versionID, ok := pathID(w, r, "versionID")
	if !ok {
		return
	}
	item, err := scanDecorationVersion(s.DB.QueryRowContext(r.Context(), `SELECT id,version_no,config_json,publish_note,DATE_FORMAT(published_at,'%Y-%m-%dT%H:%i:%sZ') FROM store_decoration_versions WHERE id=? AND tenant_id=? AND store_id=?`, versionID, identity.TenantID, store.ID))
	if err != nil {
		handleSQLError(w, err)
		return
	}
	writeData(w, http.StatusOK, item)
}

func (s *Server) rollbackDecoration(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	store, err := s.decorationStore(r.Context(), r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	versionID, ok := pathID(w, r, "versionID")
	if !ok {
		return
	}
	var input decorationRollbackInput
	if !decodeJSON(w, r, &input) {
		return
	}
	input.Note = strings.TrimSpace(input.Note)
	if !validText(input.Note, 255) {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "rollback note is too long")
		return
	}
	var sourceJSON string
	if err = s.DB.QueryRowContext(r.Context(), `SELECT config_json FROM store_decoration_versions WHERE id=? AND tenant_id=? AND store_id=?`, versionID, identity.TenantID, store.ID).Scan(&sourceJSON); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.Note == "" {
		input.Note = fmt.Sprintf("回滚至版本 %d", versionID)
	}
	published, err := s.publishDecorationVersion(r.Context(), identity, store.ID, input.ExpectedRevision, input.Note, versionID, sourceJSON)
	if errors.Is(err, errDecorationConflict) {
		writeError(w, http.StatusConflict, "DRAFT_CONFLICT", "decoration draft changed; reload before rolling back")
		return
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "decoration.rollback", "store_decoration_version", int64String(published.ID), map[string]any{"sourceVersionId": versionID, "versionNo": published.VersionNo}, r)
	writeData(w, http.StatusCreated, published)
}

func (s *Server) getDecorationTemplates(w http.ResponseWriter, _ *http.Request) {
	coffee := defaultDecorationConfig(storeDTO{})
	night := coffee
	night.TemplateKey = "night-market"
	night.Theme = DecorationTheme{
		PrimaryColor: "#1f4036", AccentColor: "#f4c95d", BackgroundColor: "#131b18",
		SurfaceColor: "#1f2925", TextColor: "#f8f4e8", MutedColor: "#aab5ad",
		NavBackgroundColor: "#17201d", NavTextColor: "#aab5ad", NavSelectedColor: "#f4c95d", Radius: "MD",
	}
	night.Navigation.BackgroundColor = night.Theme.NavBackgroundColor
	night.Navigation.TextColor = night.Theme.NavTextColor
	night.Navigation.SelectedColor = night.Theme.NavSelectedColor
	clean := coffee
	clean.TemplateKey = "clean"
	clean.Theme = DecorationTheme{
		PrimaryColor: "#2563eb", AccentColor: "#dbeafe", BackgroundColor: "#f8fafc",
		SurfaceColor: "#ffffff", TextColor: "#0f172a", MutedColor: "#64748b",
		NavBackgroundColor: "#ffffff", NavTextColor: "#64748b", NavSelectedColor: "#2563eb", Radius: "SM",
	}
	clean.Navigation.BackgroundColor = clean.Theme.NavBackgroundColor
	clean.Navigation.TextColor = clean.Theme.NavTextColor
	clean.Navigation.SelectedColor = clean.Theme.NavSelectedColor
	writeData(w, http.StatusOK, []map[string]any{
		{"key": "coffee-light", "name": "咖啡暖调", "config": coffee},
		{"key": "night-market", "name": "夜市深色", "config": night},
		{"key": "clean", "name": "简洁明亮", "config": clean},
	})
}

func scanDecorationVersion(scanner interface{ Scan(...any) error }) (decorationPublishedView, error) {
	var item decorationPublishedView
	var body string
	if err := scanner.Scan(&item.ID, &item.VersionNo, &body, &item.Note, &item.PublishedAt); err != nil {
		return item, err
	}
	if err := json.Unmarshal([]byte(body), &item.Config); err != nil {
		return item, err
	}
	return item, nil
}

func (s *Server) loadDecorationView(ctx context.Context, store storeDTO) (decorationView, error) {
	var revision int64
	var draftJSON, updatedAt string
	var publishedID sql.NullInt64
	var publishedVersion sql.NullInt64
	var publishedJSON, publishedNote, publishedAt sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT d.draft_revision,d.draft_json,DATE_FORMAT(d.updated_at,'%Y-%m-%dT%H:%i:%sZ'),v.id,v.version_no,v.config_json,v.publish_note,DATE_FORMAT(v.published_at,'%Y-%m-%dT%H:%i:%sZ') FROM store_decorations d LEFT JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`, store.TenantID, store.ID).
		Scan(&revision, &draftJSON, &updatedAt, &publishedID, &publishedVersion, &publishedJSON, &publishedNote, &publishedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return decorationView{StoreName: store.Name, Draft: decorationDraftView{Revision: 0, Config: defaultDecorationConfig(store)}}, nil
	}
	if err != nil {
		return decorationView{}, err
	}
	var draft DecorationConfig
	if err = json.Unmarshal([]byte(draftJSON), &draft); err != nil {
		return decorationView{}, err
	}
	view := decorationView{StoreName: store.Name, Draft: decorationDraftView{Revision: revision, Config: draft, UpdatedAt: updatedAt}}
	if publishedID.Valid && publishedJSON.Valid {
		var config DecorationConfig
		if err = json.Unmarshal([]byte(publishedJSON.String), &config); err != nil {
			return decorationView{}, err
		}
		view.Published = &decorationPublishedView{ID: publishedID.Int64, VersionNo: int(publishedVersion.Int64), Config: config, Note: publishedNote.String, PublishedAt: publishedAt.String}
	}
	return view, nil
}

func (s *Server) decorationStore(ctx context.Context, r *http.Request, tenantID int64) (storeDTO, error) {
	storeID, err := s.tenantStoreID(r, tenantID)
	if err != nil {
		return storeDTO{}, err
	}
	var store storeDTO
	err = scanStore(s.DB.QueryRowContext(ctx, `SELECT id,tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ') FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, storeID, tenantID), &store)
	return store, err
}

// publicDecorationConfig returns only an immutable published snapshot. Drafts
// are never exposed to customer routes. Corrupt or absent snapshots degrade to
// the legacy store fields so a bad decoration cannot take ordering offline.
func (s *Server) publicDecorationConfig(ctx context.Context, store storeDTO) (DecorationConfig, int) {
	var version int
	var body string
	err := s.DB.QueryRowContext(ctx, `SELECT v.version_no,v.config_json FROM store_decorations d JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`, store.TenantID, store.ID).Scan(&version, &body)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.Logger.Error("load public decoration", "error", err, "tenant_id", store.TenantID, "store_id", store.ID)
		}
		return defaultDecorationConfig(store), 0
	}
	var config DecorationConfig
	if err = json.Unmarshal([]byte(body), &config); err != nil {
		s.Logger.Error("decode public decoration", "error", err, "tenant_id", store.TenantID, "store_id", store.ID, "version", version)
		return defaultDecorationConfig(store), 0
	}
	if validationErr := validateDecorationConfig(config); validationErr != nil {
		s.Logger.Error("validate public decoration", "error", validationErr, "tenant_id", store.TenantID, "store_id", store.ID, "version", version)
		return defaultDecorationConfig(store), 0
	}
	return config, version
}

func (s *Server) listMediaAssets(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	page, size, offset := pagination(r)
	var total int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM media_assets WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL`, identity.TenantID, storeID).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,DATE_FORMAT(created_at,'%Y-%m-%dT%H:%i:%sZ') FROM media_assets WHERE tenant_id=? AND store_id=? AND deleted_at IS NULL ORDER BY id DESC LIMIT ? OFFSET ?`, identity.TenantID, storeID, size, offset)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []mediaAssetView{}
	for rows.Next() {
		var item mediaAssetView
		if err = rows.Scan(&item.ID, &item.Name, &item.Kind, &item.URL, &item.StorageKey, &item.MimeType, &item.Width, &item.Height, &item.SizeBytes, &item.Status, &item.CreatedAt); err != nil {
			handleSQLError(w, err)
			return
		}
		items = append(items, item)
	}
	writeList(w, http.StatusOK, items, total, page, size)
}

func (s *Server) createMediaAsset(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	var input mediaAssetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = validateMediaAssetInput(input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`, identity.TenantID, storeID, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, identity.UserID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	s.audit(r.Context(), identity, "media_asset.create", "media_asset", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusCreated, mediaAssetView{ID: id, Name: strings.TrimSpace(input.Name), Kind: "IMAGE", URL: strings.TrimSpace(input.URL), StorageKey: strings.TrimSpace(input.StorageKey), MimeType: strings.TrimSpace(input.MimeType), Width: input.Width, Height: input.Height, SizeBytes: input.SizeBytes, Status: "ACTIVE"})
}

func (s *Server) updateMediaAsset(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "assetID")
	if !ok {
		return
	}
	var input mediaAssetInput
	if !decodeJSON(w, r, &input) {
		return
	}
	if err = validateMediaAssetInput(input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE media_assets SET name=?,url=?,storage_key=?,mime_type=?,width=?,height=?,size_bytes=? WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	s.audit(r.Context(), identity, "media_asset.update", "media_asset", int64String(id), map[string]any{"name": input.Name}, r)
	writeData(w, http.StatusOK, mediaAssetView{ID: id, Name: strings.TrimSpace(input.Name), Kind: "IMAGE", URL: strings.TrimSpace(input.URL), StorageKey: strings.TrimSpace(input.StorageKey), MimeType: strings.TrimSpace(input.MimeType), Width: input.Width, Height: input.Height, SizeBytes: input.SizeBytes, Status: "ACTIVE"})
}

func (s *Server) deleteMediaAsset(w http.ResponseWriter, r *http.Request) {
	identity := currentIdentity(r.Context())
	storeID, err := s.tenantStoreID(r, identity.TenantID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, ok := pathID(w, r, "assetID")
	if !ok {
		return
	}
	result, err := s.DB.ExecContext(r.Context(), `UPDATE media_assets SET status='DELETED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND store_id=? AND deleted_at IS NULL`, id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	s.audit(r.Context(), identity, "media_asset.delete", "media_asset", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func validateMediaAssetInput(input mediaAssetInput) error {
	if !validRequiredText(input.Name, 120) {
		return errors.New("name is required and limited to 120 characters")
	}
	if !validHTTPSURL(input.URL) || len(input.URL) > 1024 {
		return errors.New("url must be an HTTPS URL no longer than 1024 characters")
	}
	if len(input.StorageKey) > 512 || len(input.MimeType) > 100 {
		return errors.New("storageKey or mimeType is too long")
	}
	if input.Width < 0 || input.Height < 0 || input.Width > 20000 || input.Height > 20000 || input.SizeBytes < 0 || input.SizeBytes > 100*1024*1024 {
		return errors.New("image metadata is outside the allowed range")
	}
	if input.MimeType != "" && !strings.HasPrefix(strings.ToLower(input.MimeType), "image/") {
		return errors.New("mimeType must be an image type")
	}
	return nil
}
