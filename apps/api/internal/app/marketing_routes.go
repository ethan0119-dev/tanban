package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

const provisionalMarketingAssetWarning = "领取记录已保存到当前设备，是否可使用请以活动规则和订单结算页展示为准"

// registerMarketingMerchantRoutes deliberately lives outside merchant.go so the
// marketing module can be mounted without widening staff permissions. The
// caller should invoke it from the existing /merchant router.
func (s *Server) registerMarketingMerchantRoutes(r chi.Router) {
	r.Group(func(managers chi.Router) {
		managers.Use(requireRoles(RoleMerchantOwner, RoleMerchantManager))

		managers.Get("/marketing/apps", s.listMarketingApps)

		managers.Get("/marketing/coupons", s.listMarketingCoupons)
		managers.Post("/marketing/coupons", s.createMarketingCoupon)
		managers.Get("/marketing/coupons/{couponID}", s.getMarketingCoupon)
		managers.Put("/marketing/coupons/{couponID}", s.updateMarketingCoupon)
		managers.Delete("/marketing/coupons/{couponID}", s.deleteMarketingCoupon)
		managers.Post("/marketing/coupons/{couponID}/activate", s.activateMarketingCoupon)
		managers.Post("/marketing/coupons/{couponID}/pause", s.pauseMarketingCoupon)
		managers.Get("/marketing/coupon-records", s.listMarketingCouponRecords)
		managers.Get("/marketing/full-reductions", s.listFullReductions)
		managers.Post("/marketing/full-reductions", s.createFullReduction)
		managers.Put("/marketing/full-reductions/{campaignID}", s.updateFullReduction)
		managers.Post("/marketing/full-reductions/{campaignID}/activate", s.activateFullReduction)
		managers.Post("/marketing/full-reductions/{campaignID}/pause", s.pauseFullReduction)

		managers.Get("/marketing/placements", s.listMarketingPlacements)
		managers.Post("/marketing/placements", s.createMarketingPlacement)
		managers.Get("/marketing/placements/{placementID}", s.getMarketingPlacement)
		managers.Put("/marketing/placements/{placementID}", s.updateMarketingPlacement)
		managers.Delete("/marketing/placements/{placementID}", s.deleteMarketingPlacement)
		managers.Post("/marketing/placements/{placementID}/activate", s.activateMarketingPlacement)
		managers.Post("/marketing/placements/{placementID}/pause", s.pauseMarketingPlacement)

		managers.Get("/marketing/lotteries", s.listMarketingLotteries)
		managers.Post("/marketing/lotteries", s.createMarketingLottery)
		managers.Get("/marketing/lotteries/{lotteryID}", s.getMarketingLottery)
		managers.Put("/marketing/lotteries/{lotteryID}", s.updateMarketingLottery)
		managers.Delete("/marketing/lotteries/{lotteryID}", s.deleteMarketingLottery)
		managers.Post("/marketing/lotteries/{lotteryID}/activate", s.activateMarketingLottery)
		managers.Post("/marketing/lotteries/{lotteryID}/pause", s.pauseMarketingLottery)
		managers.Get("/marketing/lottery-draws", s.listMarketingLotteryDraws)
	})
}

// registerPublicMarketingRoutes exposes only public campaign discovery,
// provisional anonymous claims, popup events and free lottery draws. It never
// accepts a customer_id or OpenID supplied by the client.
func (s *Server) registerPublicMarketingRoutes(r chi.Router) {
	r.Get("/stores/{storeCode}/marketing/coupons", s.publicListMarketingCoupons)
	r.Post("/stores/{storeCode}/marketing/coupons/{couponID}/claim", s.publicClaimMarketingCoupon)
	r.Get("/stores/{storeCode}/marketing/full-reductions", s.publicListFullReductions)
	r.Get("/stores/{storeCode}/marketing/popup", s.publicMarketingPopup)
	r.Post("/stores/{storeCode}/marketing/events", s.publicRecordMarketingEvent)
	r.Get("/stores/{storeCode}/marketing/lotteries", s.publicListMarketingLotteries)
	r.Get("/stores/{storeCode}/marketing/lotteries/{lotteryID}", s.publicGetMarketingLottery)
	r.Post("/stores/{storeCode}/marketing/lotteries/{lotteryID}/draw", s.publicDrawMarketingLottery)
}

func marketingSubjectHash(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 12 || len(raw) > 128 {
		return "", errors.New("subject_key must contain between 12 and 128 characters")
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:]), nil
}

func marketingIdempotencyKey(r *http.Request, bodyValue string) (string, error) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		key = strings.TrimSpace(bodyValue)
	}
	if len(key) < 8 || len(key) > 128 {
		return "", errors.New("Idempotency-Key is required and must contain between 8 and 128 characters")
	}
	if strings.HasPrefix(strings.ToLower(key), "system:") {
		return "", errors.New("Idempotency-Key uses a reserved prefix")
	}
	return key, nil
}

func marketingTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := formatBeijingDateTime(value.Time)
	return &formatted
}

func normalizeMarketingOrderTypes(values []string) ([]string, string, error) {
	if len(values) == 0 {
		values = []string{"DINE_IN", "TAKEOUT"}
	}
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.ToUpper(strings.TrimSpace(raw))
		if !validStatus(value, "DINE_IN", "TAKEOUT", "DELIVERY") {
			return nil, "", errors.New("order_types may only contain DINE_IN, TAKEOUT or DELIVERY")
		}
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Strings(result)
	body, _ := json.Marshal(result)
	return result, string(body), nil
}

func decodeMarketingOrderTypes(raw string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil {
		return []string{}
	}
	return values
}

func marketingHasDelivery(values []string) bool {
	for _, value := range values {
		if strings.EqualFold(value, "DELIVERY") {
			return true
		}
	}
	return false
}

func marketingChannel(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		value = "ALL"
	}
	if !validStatus(value, "ALL", "DINE_IN", "TAKEOUT", "DELIVERY") {
		return "", errors.New("channel_scope must be ALL, DINE_IN, TAKEOUT or DELIVERY")
	}
	return value, nil
}

func publicMarketingChannel(r *http.Request) (string, error) {
	value := strings.TrimSpace(r.URL.Query().Get("channel_scope"))
	if value == "" {
		value = strings.TrimSpace(r.URL.Query().Get("channel"))
	}
	value = strings.ToUpper(value)
	if value == "" || value == "ALL" {
		return "", nil
	}
	if !validStatus(value, "DINE_IN", "TAKEOUT") {
		return "", errors.New("public marketing channel must be DINE_IN or TAKEOUT")
	}
	return value, nil
}

func marketingOrderTypesContain(values []string, wanted string) bool {
	if wanted == "" {
		return true
	}
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func marketingBusinessDate(now time.Time, timezone string) string {
	return now.In(beijingLocation).Format("2006-01-02")
}

func marketingSubjectMask(subjectHash string) string {
	if len(subjectHash) <= 8 {
		return subjectHash
	}
	return subjectHash[:8]
}

func marketingProvisionalFields() map[string]any {
	return map[string]any{
		"identity_verified": false,
		"asset_status":      "PROVISIONAL",
		"warning":           provisionalMarketingAssetWarning,
	}
}

// allowPublicMarketingMutation is a single-node abuse guard for anonymous
// value-bearing marketing actions. The verified wx.login customer session is
// still the formal identity boundary; this limiter only prevents a client from
// cheaply rotating subject_key values fast enough to drain campaign inventory.
func (s *Server) allowPublicMarketingMutation(ctx context.Context, r *http.Request, storeCode, action string) bool {
	remoteHash := publicClientHash(r)
	now := time.Now().UTC()
	return s.consumePublicMarketingBucket(ctx, storeCode, action, remoteHash, now, time.Minute, 30) &&
		s.consumePublicMarketingBucket(ctx, storeCode, action, remoteHash, now, time.Hour, 180)
}

// allowPublicMarketingEvent bounds anonymous analytics writes twice: first for
// all events from one client, then for a particular placement and event type.
// Dropping analytics is preferable to allowing forged metrics or an unbounded
// marketing_events table.
func (s *Server) allowPublicMarketingEvent(ctx context.Context, r *http.Request, storeCode string, placementID int64, eventType string) bool {
	remoteHash := publicClientHash(r)
	now := time.Now().UTC()
	if !s.consumePublicMarketingBucket(ctx, storeCode, "event-all", remoteHash, now, time.Minute, 60) ||
		!s.consumePublicMarketingBucket(ctx, storeCode, "event-all", remoteHash, now, time.Hour, 600) {
		return false
	}
	action := "event-" + strconv.FormatInt(placementID, 10) + "-" + strings.ToLower(eventType)
	return s.consumePublicMarketingBucket(ctx, storeCode, action, remoteHash, now, time.Minute, 10) &&
		s.consumePublicMarketingBucket(ctx, storeCode, action, remoteHash, now, time.Hour, 100)
}

// allowPublicMarketingEventSubject collapses noisy retries from the same
// anonymous subject into one record per placement, event type and minute. The
// outer IP limits remain authoritative because subject_key is client supplied.
func (s *Server) allowPublicMarketingEventSubject(ctx context.Context, r *http.Request, storeCode string, placementID int64, eventType, subjectHash string) bool {
	identity := strings.TrimSpace(subjectHash)
	if identity == "" {
		identity = publicClientHash(r)
	}
	action := "event-subject-" + strconv.FormatInt(placementID, 10) + "-" + strings.ToLower(eventType) + "-" + identity
	return s.consumePublicMarketingBucket(ctx, storeCode, action, "dedupe", time.Now().UTC(), time.Minute, 1)
}

func publicClientHash(r *http.Request) string {
	remoteHash := sha256.Sum256([]byte(publicClientHost(r)))
	return hex.EncodeToString(remoteHash[:8])
}

func publicClientHost(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if remote == "" {
		remote = "unknown"
	}
	return remote
}

func (s *Server) consumePublicMarketingBucket(ctx context.Context, storeCode, action, remoteHash string, now time.Time, window time.Duration, limit int) bool {
	bucket := now.Truncate(window).Unix()
	key := "public-marketing:" + strings.ToLower(strings.TrimSpace(storeCode)) + ":" + action + ":" + remoteHash + ":" + strconv.FormatInt(int64(window/time.Second), 10) + ":" + strconv.FormatInt(bucket, 10)
	s.publicRateMu.Lock()
	defer s.publicRateMu.Unlock()
	attempts := 0
	if raw, err := s.Cache.Get(ctx, key); err == nil {
		attempts, _ = strconv.Atoi(string(raw))
	}
	if attempts >= limit {
		return false
	}
	_ = s.Cache.Set(ctx, key, []byte(strconv.Itoa(attempts+1)), window+time.Minute)
	return true
}
