package app

import (
	"context"
	"math"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ethan0119-dev/tanban/apps/api/internal/cache"
)

func TestNormalizeMarketingCouponInputKeepsReservedDeliveryScopeButDoesNotActivateIt(t *testing.T) {
	t.Parallel()
	input := marketingCouponInput{
		Name:             "外卖满减预留",
		CouponType:       "full_reduction",
		ThresholdCents:   2_000,
		DiscountCents:    300,
		DistributionMode: "public_claim",
		TotalStock:       100,
		PerSubjectLimit:  1,
		ValidityMode:     "relative_days",
		ValidDays:        7,
		OrderTypes:       []string{"delivery", "dine_in", "delivery"},
	}
	raw, err := normalizeMarketingCouponInput(&input)
	if err != nil {
		t.Fatal(err)
	}
	if input.CouponType != "FULL_REDUCTION" || input.DistributionMode != "PUBLIC_CLAIM" {
		t.Fatalf("enum values were not normalized: %+v", input)
	}
	values := decodeMarketingOrderTypes(raw)
	if !reflect.DeepEqual(values, []string{"DELIVERY", "DINE_IN"}) {
		t.Fatalf("unexpected order types: %#v", values)
	}
	if !marketingHasDelivery(values) {
		t.Fatal("delivery must remain visible to activation validation")
	}
}

func TestMarketingBusinessDateUsesStoreTimezone(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 20, 16, 30, 0, 0, time.UTC)
	if got := marketingBusinessDate(now, "Asia/Shanghai"); got != "2026-07-21" {
		t.Fatalf("Shanghai business date = %s, want 2026-07-21", got)
	}
	if got := marketingBusinessDate(now, "America/Los_Angeles"); got != "2026-07-20" {
		t.Fatalf("Los Angeles business date = %s, want 2026-07-20", got)
	}
}

func TestNormalizeMarketingCouponInputRejectsInvalidMoneyRules(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		input marketingCouponInput
	}{
		{"zero discount", marketingCouponInput{Name: "bad", CouponType: "CASH", DiscountCents: 0, TotalStock: 1, ValidityMode: "RELATIVE_DAYS", ValidDays: 1}},
		{"reduction over threshold", marketingCouponInput{Name: "bad", CouponType: "FULL_REDUCTION", ThresholdCents: 100, DiscountCents: 101, TotalStock: 1, ValidityMode: "RELATIVE_DAYS", ValidDays: 1}},
		{"limit over stock", marketingCouponInput{Name: "bad", CouponType: "CASH", DiscountCents: 1, TotalStock: 1, PerSubjectLimit: 2, ValidityMode: "RELATIVE_DAYS", ValidDays: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := normalizeMarketingCouponInput(&test.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNormalizeMarketingPlacementRequiresSafeTargetAndURL(t *testing.T) {
	t.Parallel()
	target := int64(7)
	valid := marketingPlacementInput{Name: "领券弹窗", ImageURL: "https://cdn.example.com/coupon.png", ActionType: "claim_coupon", ActionTargetID: &target, Frequency: "daily"}
	if err := normalizeMarketingPlacementInput(&valid); err != nil {
		t.Fatal(err)
	}
	if valid.PlacementCode != "HOME_POPUP" || valid.ChannelScope != "ALL" || valid.ActionType != "CLAIM_COUPON" {
		t.Fatalf("defaults were not normalized: %+v", valid)
	}
	menu := marketingPlacementInput{Name: "点单页弹窗", PlacementCode: "menu_popup", ImageURL: "https://cdn.example.com/menu.png"}
	if err := normalizeMarketingPlacementInput(&menu); err != nil || menu.PlacementCode != "MENU_POPUP" {
		t.Fatalf("menu placement should be accepted and normalized: %+v, err=%v", menu, err)
	}
	unknown := marketingPlacementInput{Name: "未知位置", PlacementCode: "UNKNOWN", ImageURL: "https://cdn.example.com/bad.png"}
	if err := normalizeMarketingPlacementInput(&unknown); err == nil {
		t.Fatal("unknown placement code should be rejected")
	}
	unsafe := marketingPlacementInput{Name: "bad", ImageURL: "javascript:alert(1)", ActionType: "NONE"}
	if err := normalizeMarketingPlacementInput(&unsafe); err == nil {
		t.Fatal("unsafe URL must be rejected")
	}
	missingTarget := marketingPlacementInput{Name: "bad", ImageURL: "https://cdn.example.com/a.png", ActionType: "OPEN_LOTTERY"}
	if err := normalizeMarketingPlacementInput(&missingTarget); err == nil {
		t.Fatal("targeted action without target must be rejected")
	}
}

func TestChooseWeightedMarketingPrizeUsesStableBoundaries(t *testing.T) {
	t.Parallel()
	prizes := []marketingLotteryPrizeRow{
		{ID: 1, Name: "谢谢参与", PrizeType: "THANKS", Weight: 2, Status: "ACTIVE"},
		{ID: 2, Name: "咖啡券", PrizeType: "COUPON", Weight: 3, Status: "ACTIVE"},
		{ID: 3, Name: "disabled", PrizeType: "COUPON", Weight: 100, Status: "DISABLED"},
	}
	for ticket, want := range map[int64]int64{0: 1, 1: 1, 2: 2, 4: 2} {
		got, total, err := chooseWeightedMarketingPrize(prizes, ticket)
		if err != nil {
			t.Fatalf("ticket %d: %v", ticket, err)
		}
		if total != 5 || got.ID != want {
			t.Fatalf("ticket %d selected %d with total %d; want %d,5", ticket, got.ID, total, want)
		}
	}
	if _, _, err := chooseWeightedMarketingPrize(prizes, 5); err == nil {
		t.Fatal("ticket equal to total weight must be rejected")
	}
	if _, _, err := chooseWeightedMarketingPrize([]marketingLotteryPrizeRow{{Weight: math.MaxInt64, Status: "ACTIVE"}, {Weight: 1, Status: "ACTIVE"}}, 0); err == nil {
		t.Fatal("weight overflow must be rejected")
	}
}

func TestNormalizeMarketingLotteryOnlyAllowsFreeCouponOrThanksPrizes(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	couponID := int64(9)
	input := marketingLotteryInput{
		Name: "开业抽奖", ActiveFrom: now, ActiveTo: now.Add(time.Hour), DailyLimit: 1, TotalLimit: 3, Terms: "免费参与，不要求购买。",
		Prizes: []marketingLotteryPrizeInput{
			{Name: "谢谢参与", PrizeType: "thanks", Weight: 90},
			{Name: "5元券", PrizeType: "coupon", CouponCampaignID: &couponID, Weight: 10, TotalStock: 5},
		},
	}
	if err := normalizeMarketingLotteryInput(&input); err != nil {
		t.Fatal(err)
	}
	if input.ChannelScope != "ALL" || input.Prizes[0].PrizeType != "THANKS" || input.Prizes[1].PrizeType != "COUPON" {
		t.Fatalf("lottery defaults were not normalized: %+v", input)
	}
	bad := input
	bad.Prizes = []marketingLotteryPrizeInput{{Name: "现金", PrizeType: "CASH", Weight: 1}}
	if err := normalizeMarketingLotteryInput(&bad); err == nil {
		t.Fatal("cash prize must be rejected")
	}
}

func TestMarketingAnonymousIdentityIsHashedAndAlwaysProvisional(t *testing.T) {
	t.Parallel()
	first, err := marketingSubjectHash("guest_abcdefghijkl")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := marketingSubjectHash("guest_abcdefghijkl")
	if first != second || len(first) != 64 || first == "guest_abcdefghijkl" {
		t.Fatalf("subject hash is not stable and opaque: %q", first)
	}
	if _, err = marketingSubjectHash("short"); err == nil {
		t.Fatal("short subject key must be rejected")
	}
	view := marketingProvisionalFields()
	if view["identity_verified"] != false || view["asset_status"] != "PROVISIONAL" || view["warning"] == "" {
		t.Fatalf("provisional warning missing: %#v", view)
	}
}

func TestSecureMarketingRandomBelowStaysInsideRange(t *testing.T) {
	t.Parallel()
	for range 32 {
		value, err := secureMarketingRandomBelow(7)
		if err != nil {
			t.Fatal(err)
		}
		if value < 0 || value >= 7 {
			t.Fatalf("random value %d outside [0,7)", value)
		}
	}
	if _, err := secureMarketingRandomBelow(0); err == nil {
		t.Fatal("non-positive limit must be rejected")
	}
}

func TestMarketingIdempotencyKeyRejectsReservedSystemPrefix(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequest("POST", "/", nil)
	request.Header.Set("Idempotency-Key", "system:lottery_coupon:forged")
	if _, err := marketingIdempotencyKey(request, ""); err == nil {
		t.Fatal("public callers must not be able to use the internal idempotency namespace")
	}
	request.Header.Set("Idempotency-Key", "customer-request-123")
	if key, err := marketingIdempotencyKey(request, ""); err != nil || key != "customer-request-123" {
		t.Fatalf("valid customer key was rejected: key=%q err=%v", key, err)
	}
}

func TestMarketingAppStatusSummaryIsStableAndCompact(t *testing.T) {
	t.Parallel()
	if got := (marketingAppStatusCounts{}).summary(); got != "NOT_CONFIGURED" {
		t.Fatalf("unexpected empty summary: %q", got)
	}
	counts := marketingAppStatusCounts{Draft: 2, Active: 1, Paused: 3, Ended: 4}
	if got := counts.summary(); got != "ACTIVE:1 / PAUSED:3 / DRAFT:2 / ENDED:4" {
		t.Fatalf("unexpected status summary: %q", got)
	}
}

func TestMarketingSubjectMaskNeverPanicsOnLegacyData(t *testing.T) {
	t.Parallel()
	if got := marketingSubjectMask("short"); got != "short" {
		t.Fatalf("unexpected short mask: %q", got)
	}
	if got := marketingSubjectMask("0123456789abcdef"); got != "01234567" {
		t.Fatalf("unexpected opaque mask: %q", got)
	}
}

func TestLoadMarketingDrawByKeyReplaysTheSameProvisionalCoupon(t *testing.T) {
	t.Parallel()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 7, 20, 4, 30, 0, 0, time.UTC)
	validTo := createdAt.Add(7 * 24 * time.Hour)
	prizeSnapshot := `{"id":11,"name":"5元券","prize_type":"COUPON","coupon_campaign_id":8,"weight":10,"total_stock":20,"awarded_count":3,"sort_order":2,"status":"ACTIVE"}`
	mock.ExpectQuery(`SELECT d\.id,d\.draw_no`).WithArgs(int64(1), "draw-request-123").WillReturnRows(sqlmock.NewRows([]string{
		"id", "draw_no", "campaign_id", "campaign_name", "prize_id", "result_type", "result_reason", "prize_snapshot_json", "request_fingerprint", "created_at",
		"customer_coupon_id", "coupon_no", "coupon_campaign_id", "coupon_campaign_name", "source", "valid_from", "valid_to",
	}).AddRow(101, "LD202607200001", 7, "开业抽奖", 11, "COUPON", "AWARDED", prizeSnapshot, "fingerprint", createdAt,
		201, "CP202607200001", 8, "5元代金券", "LOTTERY", createdAt, validTo))

	view, fingerprint, found, err := loadMarketingDrawByKeyWithQueryer(context.Background(), db, 1, "draw-request-123")
	if err != nil {
		t.Fatal(err)
	}
	if !found || fingerprint != "fingerprint" {
		t.Fatalf("draw replay was not found: found=%v fingerprint=%q", found, fingerprint)
	}
	if view["draw_id"] != int64(101) || view["prize_type"] != "COUPON" || view["prize_name"] != "5元券" {
		t.Fatalf("draw response contract was not preserved: %#v", view)
	}
	coupon, ok := view["coupon"].(map[string]any)
	if !ok || coupon["id"] != int64(201) || coupon["asset_status"] != "PROVISIONAL" || coupon["identity_verified"] != false {
		t.Fatalf("idempotent replay lost its provisional coupon: %#v", view["coupon"])
	}
	prize, ok := view["prize"].(map[string]any)
	if !ok || prize["coupon_campaign_id"] != int64(8) || prize["status"] != "ACTIVE" {
		t.Fatalf("idempotent replay lost its prize snapshot: %#v", view["prize"])
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMarketingDrawViewDoesNotReportUnavailableCouponAsAWin(t *testing.T) {
	t.Parallel()
	view := marketingDrawView(1, "LD1", 2, "抽奖", marketingLotteryPrizeRow{Name: "已售罄咖啡券", PrizeType: "COUPON"}, "THANKS", "PRIZE_UNAVAILABLE", nil, time.Now())
	if view["prize_type"] != "THANKS" || view["prize_name"] != "谢谢参与" {
		t.Fatalf("unavailable coupon must be rendered as a non-winning draw: %#v", view)
	}
}

func TestPublicMarketingRateLimitCannotBeBypassedByChangingSourcePort(t *testing.T) {
	t.Parallel()
	server := &Server{Cache: cache.NewMemory()}
	for index := 0; index < 30; index++ {
		request := httptest.NewRequest("POST", "/", nil)
		request.RemoteAddr = "203.0.113.8:" + strconv.Itoa(40000+index)
		if !server.allowPublicMarketingMutation(request.Context(), request, "coffee-a", "coupon-claim") {
			t.Fatalf("request %d was rate limited too early", index+1)
		}
	}
	request := httptest.NewRequest("POST", "/", nil)
	request.RemoteAddr = "203.0.113.8:49999"
	if server.allowPublicMarketingMutation(request.Context(), request, "coffee-a", "coupon-claim") {
		t.Fatal("changing the TCP source port must not bypass the per-IP limit")
	}
	other := httptest.NewRequest("POST", "/", nil)
	other.RemoteAddr = "203.0.113.9:40000"
	if !server.allowPublicMarketingMutation(other.Context(), other, "coffee-a", "coupon-claim") {
		t.Fatal("an unrelated IP must have an independent bucket")
	}
}

func TestPublicMarketingEventRateLimitAndSubjectDedupe(t *testing.T) {
	t.Parallel()
	server := &Server{Cache: cache.NewMemory()}
	request := httptest.NewRequest("POST", "/", nil)
	request.RemoteAddr = "203.0.113.8:40000"
	for index := 0; index < 10; index++ {
		if !server.allowPublicMarketingEvent(request.Context(), request, "coffee-a", 12, "IMPRESSION") {
			t.Fatalf("event %d was rate limited too early", index+1)
		}
	}
	if server.allowPublicMarketingEvent(request.Context(), request, "coffee-a", 12, "IMPRESSION") {
		t.Fatal("the same placement and event type must be bounded")
	}
	if !server.allowPublicMarketingEvent(request.Context(), request, "coffee-a", 13, "IMPRESSION") {
		t.Fatal("an unrelated placement should retain its own short-window allowance")
	}

	other := httptest.NewRequest("POST", "/", nil)
	other.RemoteAddr = "203.0.113.9:40000"
	if !server.allowPublicMarketingEventSubject(other.Context(), other, "coffee-a", 12, "CLICK", "subject-hash") {
		t.Fatal("the first subject event should be recorded")
	}
	if server.allowPublicMarketingEventSubject(other.Context(), other, "coffee-a", 12, "CLICK", "subject-hash") {
		t.Fatal("a repeated subject event in the same minute should be deduplicated")
	}
	if !server.allowPublicMarketingEventSubject(other.Context(), other, "coffee-a", 12, "CLOSE", "subject-hash") {
		t.Fatal("a different event type should retain its own dedupe key")
	}
}
