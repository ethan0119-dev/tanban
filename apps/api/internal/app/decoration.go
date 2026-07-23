package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	decorationSchemaVersion = 1
	decorationMaxBytes      = 256 * 1024
	decorationMaxModules    = 30
	decorationMaxBanners    = 8
	decorationMaxHotspots   = 20
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
	FontScale          string `json:"fontScale"`
	SurfaceStyle       string `json:"surfaceStyle"`
	ButtonShape        string `json:"buttonShape"`
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
	TemplateKey     string                     `json:"templateKey"`
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

type decorationHotspotImageConfig struct {
	ImageURL string              `json:"imageUrl"`
	Alt      string              `json:"alt"`
	Hotspots []decorationHotspot `json:"hotspots"`
}

type decorationHotspot struct {
	ID     string           `json:"id"`
	X      float64          `json:"x"`
	Y      float64          `json:"y"`
	Width  float64          `json:"width"`
	Height float64          `json:"height"`
	Label  string           `json:"label"`
	Action DecorationAction `json:"action"`
}

type decorationTextConfig struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Align string `json:"align"`
}

type decorationCustomerServiceConfig struct {
	Title string `json:"title"`
	Body  string `json:"body"`
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
	GroupID    *int64 `json:"group_id"`
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
	GroupID    *int64 `json:"group_id"`
	GroupName  string `json:"group_name"`
}

func defaultDecorationConfig(store storeDTO) DecorationConfig {
	theme := DecorationTheme{
		PrimaryColor: "#214d3f", AccentColor: "#dff06d",
		BackgroundColor: "#f6f5f0", SurfaceColor: "#fffefa",
		TextColor: "#17201b", MutedColor: "#747b75",
		NavBackgroundColor: "#fffefa", NavTextColor: "#7b807a",
		NavSelectedColor: "#214d3f", Radius: "LG",
		FontScale: "STANDARD", SurfaceStyle: "ELEVATED", ButtonShape: "ROUNDED",
	}
	hero := decorationHeroConfig{}
	if validDecorationURL(store.BannerURL) {
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
			TemplateKey:     "classic",
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
	fill(&config.Theme.FontScale, defaults.Theme.FontScale)
	fill(&config.Theme.SurfaceStyle, defaults.Theme.SurfaceStyle)
	fill(&config.Theme.ButtonShape, defaults.Theme.ButtonShape)
	fill(&config.Menu.CategoryLayout, defaults.Menu.CategoryLayout)
	fill(&config.Menu.ProductLayout, defaults.Menu.ProductLayout)
	fill(&config.Menu.LoadMode, defaults.Menu.LoadMode)
	fill(&config.Menu.ProductActionMode, defaults.Menu.ProductActionMode)
	fill(&config.Menu.Density, defaults.Menu.Density)
	fill(&config.Navigation.BackgroundColor, config.Theme.NavBackgroundColor)
	fill(&config.Navigation.TemplateKey, defaults.Navigation.TemplateKey)
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
	if !oneOf(config.Theme.FontScale, "COMPACT", "STANDARD", "LARGE") {
		return errors.New("theme.fontScale must be COMPACT, STANDARD or LARGE")
	}
	if !oneOf(config.Theme.SurfaceStyle, "FLAT", "BORDERED", "ELEVATED") {
		return errors.New("theme.surfaceStyle must be FLAT, BORDERED or ELEVATED")
	}
	if !oneOf(config.Theme.ButtonShape, "SQUARE", "ROUNDED", "PILL") {
		return errors.New("theme.buttonShape must be SQUARE, ROUNDED or PILL")
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
	if !oneOf(config.Navigation.TemplateKey, "classic", "soft", "warm", "dark") {
		return errors.New("navigation.templateKey must be classic, soft, warm or dark")
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
	if config.Splash.Enabled && !validDecorationURL(config.Splash.ImageURL) {
		return errors.New("splash.imageUrl must be secure when splash is enabled")
	}
	if config.Splash.ImageURL != "" && !validDecorationURL(config.Splash.ImageURL) {
		return errors.New("splash.imageUrl must be secure")
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
			if !validDecorationURL(item.ImageURL) {
				return errors.New("banner imageUrl must be HTTPS (loopback HTTP is allowed in local development)")
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
		if !validDecorationURL(config.ImageURL) || !validText(config.Alt, 80) {
			return errors.New("image module requires an HTTPS imageUrl (or local loopback HTTP) and short alt text")
		}
		return validateDecorationAction(config.Action)
	case "HOTSPOT_IMAGE":
		var config decorationHotspotImageConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validDecorationURL(config.ImageURL) || !validText(config.Alt, 80) {
			return errors.New("hotspot image requires an HTTPS imageUrl (or local loopback HTTP) and short alt text")
		}
		if len(config.Hotspots) > decorationMaxHotspots {
			return fmt.Errorf("hotspot image cannot contain more than %d hotspots", decorationMaxHotspots)
		}
		hotspotIDs := map[string]bool{}
		for index, hotspot := range config.Hotspots {
			if !validIdentifier(hotspot.ID, 64) || hotspotIDs[hotspot.ID] {
				return fmt.Errorf("hotspots[%d].id is invalid or duplicated", index)
			}
			hotspotIDs[hotspot.ID] = true
			if hotspot.X < 0 || hotspot.Y < 0 || hotspot.Width <= 0 || hotspot.Height <= 0 ||
				hotspot.X+hotspot.Width > 100 || hotspot.Y+hotspot.Height > 100 {
				return fmt.Errorf("hotspots[%d] must fit within 0-100 percent coordinates", index)
			}
			if !validRequiredText(hotspot.Label, 60) {
				return fmt.Errorf("hotspots[%d].label is required and limited to 60 characters", index)
			}
			if err := validateDecorationAction(hotspot.Action); err != nil {
				return fmt.Errorf("hotspots[%d].action: %w", index, err)
			}
		}
	case "TEXT":
		var config decorationTextConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validText(config.Title, 80) || !validText(config.Body, 500) || !oneOf(config.Align, "LEFT", "CENTER", "RIGHT") {
			return errors.New("text module content or alignment is invalid")
		}
	case "CUSTOMER_SERVICE":
		var config decorationCustomerServiceConfig
		if err := strictRawJSON(module.Config, &config); err != nil {
			return err
		}
		if !validText(config.Title, 80) || !validText(config.Body, 500) {
			return errors.New("customer service module text is too long")
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
	if !oneOf(action.Type, "NONE", "OPEN_MENU", "OPEN_DINE_IN", "OPEN_TAKEOUT", "OPEN_DELIVERY", "OPEN_ORDERS", "OPEN_PROFILE", "OPEN_RECHARGE", "OPEN_MY_COUPONS", "OPEN_COUPON_CENTER", "CALL_PHONE") {
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

func validDecorationURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

var errDecorationAssetUnavailable = errors.New("managed decoration image is unavailable")

func lockDecorationStore(ctx context.Context, tx *sql.Tx, tenantID, storeID int64) error {
	var lockedID int64
	return tx.QueryRowContext(ctx, `SELECT id FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL FOR UPDATE`, storeID, tenantID).Scan(&lockedID)
}

// validateManagedMediaURL verifies a server-managed image while the caller is
// holding the store row lock. The media delete path takes that same lock first,
// which makes "save a reference" and "delete the image" mutually exclusive.
// External HTTPS URLs are not managed by this service and are left untouched.
func (s *Server) validateManagedMediaURL(ctx context.Context, tx *sql.Tx, tenantID, storeID int64, rawURL string) error {
	base := strings.TrimRight(strings.TrimSpace(s.Config.MediaPublicBaseURL), "/")
	rawURL = strings.TrimSpace(rawURL)
	if base == "" || rawURL == "" || !strings.HasPrefix(rawURL, base+"/") {
		return nil
	}
	storageKey := strings.TrimPrefix(rawURL, base+"/")
	keyTenantID, keyStoreID, ok := parseLocalMediaStorageKey(storageKey)
	if !ok || keyTenantID != tenantID || keyStoreID != storeID {
		return fmt.Errorf("%w: image belongs to a different store", errDecorationAssetUnavailable)
	}
	var assetID int64
	err := tx.QueryRowContext(ctx, `SELECT id FROM media_assets WHERE tenant_id=? AND store_id=? AND storage_key=? AND url=? AND kind='IMAGE' AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, tenantID, storeID, storageKey, rawURL).Scan(&assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: image has been deleted or is unavailable", errDecorationAssetUnavailable)
	}
	return err
}

// validateManagedDecorationAssets locks every locally uploaded image referenced
// by a decoration while the caller is already holding the store row lock. The
// delete path takes the same locks in the same order, so a draft/publish cannot
// start referencing an image after deletion has passed its reference checks.
func (s *Server) validateManagedDecorationAssets(ctx context.Context, tx *sql.Tx, tenantID, storeID int64, config DecorationConfig) error {
	storageKeys, err := managedDecorationStorageKeys(config, s.Config.MediaPublicBaseURL)
	if err != nil {
		return err
	}
	for _, storageKey := range storageKeys {
		keyTenantID, keyStoreID, ok := parseLocalMediaStorageKey(storageKey)
		if !ok || keyTenantID != tenantID || keyStoreID != storeID {
			return fmt.Errorf("%w: image belongs to a different store", errDecorationAssetUnavailable)
		}
		var assetID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM media_assets WHERE tenant_id=? AND store_id=? AND storage_key=? AND kind='IMAGE' AND status='ACTIVE' AND deleted_at IS NULL FOR UPDATE`, tenantID, storeID, storageKey).Scan(&assetID)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", errDecorationAssetUnavailable, storageKey)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func managedDecorationStorageKeys(config DecorationConfig, publicBaseURL string) ([]string, error) {
	base := strings.TrimRight(strings.TrimSpace(publicBaseURL), "/")
	if base == "" {
		return nil, nil
	}
	body, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	var value any
	if err = json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	keys := map[string]struct{}{}
	collectManagedDecorationStorageKeys(value, base+"/", keys)
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
}

func collectManagedDecorationStorageKeys(value any, prefix string, keys map[string]struct{}) {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, prefix) {
			keys[strings.TrimPrefix(typed, prefix)] = struct{}{}
		}
	case []any:
		for _, item := range typed {
			collectManagedDecorationStorageKeys(item, prefix, keys)
		}
	case map[string]any:
		for _, item := range typed {
			collectManagedDecorationStorageKeys(item, prefix, keys)
		}
	}
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
	if parsed, err := parseBeijingDateTime(value); err == nil {
		return parsed, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.ParseInLocation(layout, value, beijingLocation); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, errors.New("must be a Beijing date-time such as 2026-07-21 14:00:00")
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
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, store.ID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = s.validateManagedDecorationAssets(r.Context(), tx, identity.TenantID, store.ID, input.Config); err != nil {
		if errors.Is(err, errDecorationAssetUnavailable) {
			writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
		} else {
			handleSQLError(w, err)
		}
		return
	}
	var newRevision int64
	if input.ExpectedRevision == 0 {
		result, insertErr := tx.ExecContext(r.Context(), `INSERT INTO store_decorations(tenant_id,store_id,schema_version,draft_json,draft_revision,created_by,updated_by) VALUES(?,?,?,?,1,?,?)`, identity.TenantID, store.ID, input.Config.SchemaVersion, string(body), identity.UserID, identity.UserID)
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
		result, updateErr := tx.ExecContext(r.Context(), `UPDATE store_decorations SET schema_version=?,draft_json=?,draft_revision=draft_revision+1,updated_by=? WHERE tenant_id=? AND store_id=? AND draft_revision=?`, input.Config.SchemaVersion, string(body), identity.UserID, identity.TenantID, store.ID, input.ExpectedRevision)
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
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
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
	if errors.Is(err, errDecorationAssetUnavailable) {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
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
	if err = lockDecorationStore(ctx, tx, actor.TenantID, storeID); err != nil {
		return decorationPublishedView{}, err
	}
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
	if err = s.validateManagedDecorationAssets(ctx, tx, actor.TenantID, storeID, config); err != nil {
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
	return decorationPublishedView{ID: versionID, VersionNo: versionNo, Config: config, Note: note, PublishedAt: formatBeijingDateTime(time.Now())}, nil
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
	rows, err := s.DB.QueryContext(r.Context(), `SELECT id,version_no,config_json,publish_note,DATE_FORMAT(published_at,'%Y-%m-%d %H:%i:%s') FROM store_decoration_versions WHERE tenant_id=? AND store_id=? ORDER BY version_no DESC LIMIT ? OFFSET ?`, identity.TenantID, store.ID, size, offset)
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
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
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
	item, err := scanDecorationVersion(s.DB.QueryRowContext(r.Context(), `SELECT id,version_no,config_json,publish_note,DATE_FORMAT(published_at,'%Y-%m-%d %H:%i:%s') FROM store_decoration_versions WHERE id=? AND tenant_id=? AND store_id=?`, versionID, identity.TenantID, store.ID))
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
	if errors.Is(err, errDecorationAssetUnavailable) {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_UNAVAILABLE", err.Error())
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
		FontScale: "STANDARD", SurfaceStyle: "ELEVATED", ButtonShape: "ROUNDED",
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
		FontScale: "STANDARD", SurfaceStyle: "BORDERED", ButtonShape: "ROUNDED",
	}
	clean.Navigation.BackgroundColor = clean.Theme.NavBackgroundColor
	clean.Navigation.TextColor = clean.Theme.NavTextColor
	clean.Navigation.SelectedColor = clean.Theme.NavSelectedColor
	writeData(w, http.StatusOK, []map[string]any{
		{"key": "coffee-light", "name": "咖啡暖调", "description": "自然深绿与浅奶油色，适合日常咖啡和移动摊位。", "scene": "咖啡 · 摊位", "highlights": []string{"标准首页", "舒适点单", "品牌主色"}, "tone": "linear-gradient(135deg,#214d3f,#dff06d)", "config": coffee},
		{"key": "night-market", "name": "夜市深色", "description": "深色表面与金色强调，适合夜间营业、烧烤和夜宵。", "scene": "夜市 · 夜宵", "highlights": []string{"深色沉浸", "高对比导航", "浮层卡片"}, "tone": "linear-gradient(135deg,#131b18,#f4c95d)", "config": night},
		{"key": "clean", "name": "简洁明亮", "description": "蓝白高对比和轻描边，适合快餐、轻食及通用新店。", "scene": "快餐 · 通用", "highlights": []string{"清晰层级", "描边卡片", "紧凑结构"}, "tone": "linear-gradient(135deg,#2563eb,#dbeafe)", "config": clean},
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
	err := s.DB.QueryRowContext(ctx, `SELECT d.draft_revision,d.draft_json,DATE_FORMAT(d.updated_at,'%Y-%m-%d %H:%i:%s'),v.id,v.version_no,v.config_json,v.publish_note,DATE_FORMAT(v.published_at,'%Y-%m-%d %H:%i:%s') FROM store_decorations d LEFT JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`, store.TenantID, store.ID).
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
	err = scanStore(s.DB.QueryRowContext(ctx, `SELECT id,tenant_id,code,name,logo_url,banner_url,address,phone,business_hours,notice,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM stores WHERE id=? AND tenant_id=? AND deleted_at IS NULL`, storeID, tenantID), &store)
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
	// Published snapshots are immutable, but the public view remains backward
	// compatible as constrained theme fields are added. Normalize only missing
	// fields in memory before validation; never expose a draft or rewrite history.
	normalizeDecorationConfig(&config)
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
	where := ` WHERE a.tenant_id=? AND a.store_id=? AND a.kind='IMAGE' AND a.deleted_at IS NULL`
	args := []any{identity.TenantID, storeID}
	if raw := strings.TrimSpace(r.URL.Query().Get("group_id")); raw != "" {
		groupID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || groupID < 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "group_id must be a non-negative integer")
			return
		}
		if groupID == 0 {
			where += " AND a.group_id IS NULL"
		} else {
			where += " AND a.group_id=?"
			args = append(args, groupID)
		}
	}
	if keyword := strings.TrimSpace(r.URL.Query().Get("q")); keyword != "" {
		if len(keyword) > 120 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "q is limited to 120 characters")
			return
		}
		where += " AND a.name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int
	if err = s.DB.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM media_assets a`+where, args...).Scan(&total); err != nil {
		handleSQLError(w, err)
		return
	}
	queryArgs := append(append([]any{}, args...), size, offset)
	rows, err := s.DB.QueryContext(r.Context(), `SELECT a.id,a.name,a.kind,a.url,a.storage_key,a.mime_type,a.width,a.height,a.size_bytes,a.status,DATE_FORMAT(a.created_at,'%Y-%m-%d %H:%i:%s'),a.group_id,COALESCE(g.name,'')
		FROM media_assets a LEFT JOIN media_asset_groups g ON g.id=a.group_id AND g.tenant_id=a.tenant_id AND g.store_id=a.store_id AND g.deleted_at IS NULL`+where+` ORDER BY a.id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer rows.Close()
	items := []mediaAssetView{}
	for rows.Next() {
		var item mediaAssetView
		var groupID sql.NullInt64
		if err = rows.Scan(&item.ID, &item.Name, &item.Kind, &item.URL, &item.StorageKey, &item.MimeType, &item.Width, &item.Height, &item.SizeBytes, &item.Status, &item.CreatedAt, &groupID, &item.GroupName); err != nil {
			handleSQLError(w, err)
			return
		}
		if groupID.Valid {
			item.GroupID = &groupID.Int64
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		handleSQLError(w, err)
		return
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
	var groupID int64
	if input.GroupID != nil {
		groupID = *input.GroupID
		if groupID < 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "group_id must be non-negative")
			return
		}
	}
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if err = validateMediaGroupID(r.Context(), tx, identity.TenantID, storeID, groupID); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_MEDIA_GROUP", err.Error())
		return
	}
	var result sql.Result
	if groupID > 0 {
		result, err = tx.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,group_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`, identity.TenantID, storeID, groupID, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, identity.UserID)
	} else {
		result, err = tx.ExecContext(r.Context(), `INSERT INTO media_assets(tenant_id,store_id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,created_by) VALUES(?,?,?,'IMAGE',?,?,?,?,?,?,'ACTIVE',?)`, identity.TenantID, storeID, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, identity.UserID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	id, _ := result.LastInsertId()
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "media_asset.create", "media_asset", int64String(id), map[string]any{"name": input.Name}, r)
	view := mediaAssetView{ID: id, Name: strings.TrimSpace(input.Name), Kind: "IMAGE", URL: strings.TrimSpace(input.URL), StorageKey: strings.TrimSpace(input.StorageKey), MimeType: strings.TrimSpace(input.MimeType), Width: input.Width, Height: input.Height, SizeBytes: input.SizeBytes, Status: "ACTIVE"}
	if groupID > 0 {
		view.GroupID = &groupID
	}
	writeData(w, http.StatusCreated, view)
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
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	if input.GroupID != nil {
		if *input.GroupID < 0 {
			writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "group_id must be non-negative")
			return
		}
		if err = validateMediaGroupID(r.Context(), tx, identity.TenantID, storeID, *input.GroupID); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_MEDIA_GROUP", err.Error())
			return
		}
	}
	var existing mediaAssetView
	err = tx.QueryRowContext(r.Context(), `SELECT id,name,kind,url,storage_key,mime_type,width,height,size_bytes,status,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') FROM media_assets WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL FOR UPDATE`, id, identity.TenantID, storeID).
		Scan(&existing.ID, &existing.Name, &existing.Kind, &existing.URL, &existing.StorageKey, &existing.MimeType, &existing.Width, &existing.Height, &existing.SizeBytes, &existing.Status, &existing.CreatedAt)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if isLocalMediaStorageKey(existing.StorageKey) {
		if err = validateLocalMediaRename(input, existing); err != nil {
			writeError(w, http.StatusBadRequest, "IMMUTABLE_MEDIA_METADATA", err.Error())
			return
		}
		name := strings.TrimSpace(input.Name)
		var result sql.Result
		var updateErr error
		if input.GroupID == nil {
			result, updateErr = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=? WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, name, id, identity.TenantID, storeID)
		} else if *input.GroupID == 0 {
			result, updateErr = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=?,group_id=NULL WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, name, id, identity.TenantID, storeID)
			existing.GroupID = nil
			existing.GroupName = ""
		} else {
			result, updateErr = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=?,group_id=? WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, name, *input.GroupID, id, identity.TenantID, storeID)
			existing.GroupID = input.GroupID
		}
		if updateErr != nil {
			handleSQLError(w, updateErr)
			return
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
			return
		}
		if err = tx.Commit(); err != nil {
			handleSQLError(w, err)
			return
		}
		existing.Name = name
		s.audit(r.Context(), identity, "media_asset.update", "media_asset", int64String(id), map[string]any{"name": name}, r)
		writeData(w, http.StatusOK, existing)
		return
	}
	if strings.TrimSpace(input.URL) == "" {
		input.URL = existing.URL
	}
	if strings.TrimSpace(input.StorageKey) == "" {
		input.StorageKey = existing.StorageKey
	}
	if strings.TrimSpace(input.MimeType) == "" {
		input.MimeType = existing.MimeType
	}
	if input.Width == 0 {
		input.Width = existing.Width
	}
	if input.Height == 0 {
		input.Height = existing.Height
	}
	if input.SizeBytes == 0 {
		input.SizeBytes = existing.SizeBytes
	}
	if err = validateMediaAssetInput(input); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	var result sql.Result
	if input.GroupID == nil {
		result, err = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=?,url=?,storage_key=?,mime_type=?,width=?,height=?,size_bytes=? WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, id, identity.TenantID, storeID)
	} else if *input.GroupID == 0 {
		result, err = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=?,url=?,storage_key=?,mime_type=?,width=?,height=?,size_bytes=?,group_id=NULL WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, id, identity.TenantID, storeID)
	} else {
		result, err = tx.ExecContext(r.Context(), `UPDATE media_assets SET name=?,url=?,storage_key=?,mime_type=?,width=?,height=?,size_bytes=?,group_id=? WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, strings.TrimSpace(input.Name), strings.TrimSpace(input.URL), strings.TrimSpace(input.StorageKey), strings.TrimSpace(input.MimeType), input.Width, input.Height, input.SizeBytes, *input.GroupID, id, identity.TenantID, storeID)
	}
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	s.audit(r.Context(), identity, "media_asset.update", "media_asset", int64String(id), map[string]any{"name": input.Name}, r)
	view := mediaAssetView{ID: id, Name: strings.TrimSpace(input.Name), Kind: "IMAGE", URL: strings.TrimSpace(input.URL), StorageKey: strings.TrimSpace(input.StorageKey), MimeType: strings.TrimSpace(input.MimeType), Width: input.Width, Height: input.Height, SizeBytes: input.SizeBytes, Status: "ACTIVE"}
	if input.GroupID != nil && *input.GroupID > 0 {
		view.GroupID = input.GroupID
	}
	writeData(w, http.StatusOK, view)
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
	tx, err := s.DB.BeginTx(r.Context(), nil)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	defer tx.Rollback()
	if err = lockDecorationStore(r.Context(), tx, identity.TenantID, storeID); err != nil {
		handleSQLError(w, err)
		return
	}
	var assetURL, storageKey string
	if err = tx.QueryRowContext(r.Context(), `SELECT url,storage_key FROM media_assets WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL FOR UPDATE`, id, identity.TenantID, storeID).Scan(&assetURL, &storageKey); err != nil {
		handleSQLError(w, err)
		return
	}
	var draftJSON, publishedJSON string
	err = tx.QueryRowContext(r.Context(), `SELECT d.draft_json,COALESCE(v.config_json,'') FROM store_decorations d LEFT JOIN store_decoration_versions v ON v.id=d.published_version_id AND v.tenant_id=d.tenant_id AND v.store_id=d.store_id WHERE d.tenant_id=? AND d.store_id=?`, identity.TenantID, storeID).Scan(&draftJSON, &publishedJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		handleSQLError(w, err)
		return
	}
	for _, body := range []string{draftJSON, publishedJSON} {
		referenced, referenceErr := decorationJSONReferencesURL(body, assetURL)
		if referenceErr != nil {
			s.Logger.Error("inspect media decoration reference", "error", referenceErr, "asset_id", id, "tenant_id", identity.TenantID, "store_id", storeID)
			writeError(w, http.StatusInternalServerError, "INVALID_DECORATION_DATA", "failed to verify whether the image is in use")
			return
		}
		if referenced {
			writeError(w, http.StatusConflict, "MEDIA_ASSET_IN_USE", "remove the image from the current draft and published decoration before deleting it")
			return
		}
	}
	versionRows, err := tx.QueryContext(r.Context(), `SELECT config_json FROM store_decoration_versions WHERE tenant_id=? AND store_id=? ORDER BY id`, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	for versionRows.Next() {
		var versionJSON string
		if err = versionRows.Scan(&versionJSON); err != nil {
			versionRows.Close()
			handleSQLError(w, err)
			return
		}
		referenced, referenceErr := decorationJSONReferencesURL(versionJSON, assetURL)
		if referenceErr != nil {
			versionRows.Close()
			s.Logger.Error("inspect historical media decoration reference", "error", referenceErr, "asset_id", id, "tenant_id", identity.TenantID, "store_id", storeID)
			writeError(w, http.StatusInternalServerError, "INVALID_DECORATION_DATA", "failed to verify whether the image is in use")
			return
		}
		if referenced {
			versionRows.Close()
			writeError(w, http.StatusConflict, "MEDIA_ASSET_IN_USE", "remove the image from all decoration versions before deleting it")
			return
		}
	}
	if err = versionRows.Err(); err != nil {
		versionRows.Close()
		handleSQLError(w, err)
		return
	}
	versionRows.Close()
	referenceKind, err := currentMediaAssetReference(r.Context(), tx, identity.TenantID, storeID, id, assetURL)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	if referenceKind != "" {
		writeError(w, http.StatusConflict, "MEDIA_ASSET_IN_USE", "remove the image from "+referenceKind+" before deleting it")
		return
	}
	result, err := tx.ExecContext(r.Context(), `UPDATE media_assets SET status='DELETED',deleted_at=NOW(3) WHERE id=? AND tenant_id=? AND store_id=? AND kind='IMAGE' AND deleted_at IS NULL`, id, identity.TenantID, storeID)
	if err != nil {
		handleSQLError(w, err)
		return
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "media asset not found")
		return
	}
	if err = tx.Commit(); err != nil {
		handleSQLError(w, err)
		return
	}
	if isLocalMediaStorageKey(storageKey) {
		if target, pathErr := localMediaPath(s.Config.MediaStorageDir, storageKey); pathErr != nil {
			s.Logger.Error("resolve deleted media path", "error", pathErr, "asset_id", id, "storage_key", storageKey)
		} else if removeErr := os.Remove(target); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			s.Logger.Error("remove deleted media file", "error", removeErr, "asset_id", id, "storage_key", storageKey)
		}
	}
	s.audit(r.Context(), identity, "media_asset.delete", "media_asset", int64String(id), nil, r)
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

func currentMediaAssetReference(ctx context.Context, queryer queryRower, tenantID, storeID, assetID int64, assetURL string) (string, error) {
	var referenceKind string
	err := queryer.QueryRowContext(ctx, `SELECT CASE
		WHEN EXISTS(SELECT 1 FROM product_images pi JOIN products p ON p.id=pi.product_id AND p.tenant_id=pi.tenant_id AND p.store_id=pi.store_id WHERE pi.tenant_id=? AND pi.store_id=? AND (pi.media_asset_id=? OR pi.url=?) AND pi.deleted_at IS NULL AND p.deleted_at IS NULL) THEN 'product images'
		WHEN EXISTS(SELECT 1 FROM products p WHERE p.tenant_id=? AND p.store_id=? AND p.image_url=? AND p.deleted_at IS NULL) THEN 'product images'
		WHEN EXISTS(SELECT 1 FROM marketing_placements p WHERE p.tenant_id=? AND p.store_id=? AND (p.image_asset_id=? OR p.image_url=?) AND p.deleted_at IS NULL) THEN 'marketing placements'
		WHEN EXISTS(SELECT 1 FROM store_operation_settings os WHERE os.tenant_id=? AND os.store_id=? AND os.customer_service_qr_url=?) THEN 'customer service QR code'
		WHEN EXISTS(SELECT 1 FROM store_profiles sp WHERE sp.tenant_id=? AND sp.store_id=? AND (JSON_CONTAINS(sp.environment_image_urls_json,JSON_QUOTE(?)) OR JSON_CONTAINS(sp.food_safety_image_urls_json,JSON_QUOTE(?)))) THEN 'store profile images'
		WHEN EXISTS(SELECT 1 FROM stores st WHERE st.tenant_id=? AND st.id=? AND (st.logo_url=? OR st.banner_url=?) AND st.deleted_at IS NULL) THEN 'store branding'
		WHEN EXISTS(SELECT 1 FROM membership_settings ms WHERE ms.tenant_id=? AND ms.card_image_url=?) THEN 'membership settings'
		WHEN EXISTS(SELECT 1 FROM modifier_items mi WHERE mi.tenant_id=? AND mi.store_id=? AND mi.image_url=? AND mi.deleted_at IS NULL) THEN 'modifier items'
		WHEN EXISTS(SELECT 1 FROM categories c WHERE c.tenant_id=? AND c.store_id=? AND c.icon_url=? AND c.deleted_at IS NULL) THEN 'categories'
		ELSE '' END`,
		tenantID, storeID, assetID, assetURL,
		tenantID, storeID, assetURL,
		tenantID, storeID, assetID, assetURL,
		tenantID, storeID, assetURL,
		tenantID, storeID, assetURL, assetURL,
		tenantID, storeID, assetURL, assetURL,
		tenantID, assetURL,
		tenantID, storeID, assetURL,
		tenantID, storeID, assetURL,
	).Scan(&referenceKind)
	return referenceKind, err
}

func validateMediaAssetInput(input mediaAssetInput) error {
	if !validRequiredText(input.Name, 120) {
		return errors.New("name is required and limited to 120 characters")
	}
	if !validDecorationURL(input.URL) || len(input.URL) > 1024 {
		return errors.New("url must be HTTPS (or loopback HTTP in local development) and no longer than 1024 characters")
	}
	if len(input.StorageKey) > 512 || len(input.MimeType) > 100 {
		return errors.New("storageKey or mimeType is too long")
	}
	if input.StorageKey != "" && isLocalMediaStorageKey(input.StorageKey) {
		return errors.New("storageKey uses a reserved server-managed prefix")
	}
	if input.Width < 0 || input.Height < 0 || input.Width > 20000 || input.Height > 20000 || input.SizeBytes < 0 || input.SizeBytes > 100*1024*1024 {
		return errors.New("image metadata is outside the allowed range")
	}
	if input.MimeType != "" && !strings.HasPrefix(strings.ToLower(input.MimeType), "image/") {
		return errors.New("mimeType must be an image type")
	}
	return nil
}

func validateLocalMediaRename(input mediaAssetInput, existing mediaAssetView) error {
	if !validRequiredText(input.Name, 120) {
		return errors.New("name is required and limited to 120 characters")
	}
	if value := strings.TrimSpace(input.URL); value != "" && value != existing.URL {
		return errors.New("the URL of an uploaded image is server-managed and cannot be changed")
	}
	if value := strings.TrimSpace(input.StorageKey); value != "" && value != existing.StorageKey {
		return errors.New("the storage key of an uploaded image is server-managed and cannot be changed")
	}
	if value := strings.TrimSpace(input.MimeType); value != "" && value != existing.MimeType {
		return errors.New("the MIME type of an uploaded image is server-managed and cannot be changed")
	}
	if input.Width != 0 && input.Width != existing.Width || input.Height != 0 && input.Height != existing.Height || input.SizeBytes != 0 && input.SizeBytes != existing.SizeBytes {
		return errors.New("the dimensions and size of an uploaded image are server-managed and cannot be changed")
	}
	return nil
}

func decorationJSONReferencesURL(body, targetURL string) (bool, error) {
	if strings.TrimSpace(body) == "" || strings.TrimSpace(targetURL) == "" {
		return false, nil
	}
	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		return false, err
	}
	return decorationValueReferencesURL(value, targetURL), nil
}

func decorationValueReferencesURL(value any, targetURL string) bool {
	switch typed := value.(type) {
	case string:
		return typed == targetURL
	case []any:
		for _, item := range typed {
			if decorationValueReferencesURL(item, targetURL) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if decorationValueReferencesURL(item, targetURL) {
				return true
			}
		}
	}
	return false
}
