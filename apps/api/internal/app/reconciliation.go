package app

import (
	"context"
	"errors"
	"time"

	"github.com/ethan0119-dev/tanban/apps/api/internal/provider"
)

// StartPaymentReconciler provides the compensation path for lost or delayed
// payment callbacks. Provider Query must be implemented before a real payment
// adapter is enabled; the mock adapter already supports this contract.
func (s *Server) StartPaymentReconciler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reconcilePayments(ctx)
			}
		}
	}()
}

func (s *Server) reconcilePayments(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id,p.tenant_id,p.order_id,p.provider_order_no,p.status,o.payment_status,p.provider_request_no,p.amount_cents,p.merchant_no
		FROM payment_transactions p
		JOIN orders o ON o.id=p.order_id AND o.tenant_id=p.tenant_id
		WHERE p.provider=? AND (p.status IN ('CREATING','PENDING') OR (p.status='SUCCESS' AND o.payment_status='UNPAID'))
		ORDER BY p.updated_at,p.id LIMIT 100`, s.Payment.Name())
	if err != nil {
		s.Logger.Error("list payments for reconciliation", "error", err)
		return
	}
	type pendingPayment struct {
		id, tenantID, orderID, amount                                         int64
		providerNo, status, orderPaymentStatus, providerRequestNo, merchantNo string
	}
	var pending []pendingPayment
	for rows.Next() {
		var item pendingPayment
		if rows.Scan(&item.id, &item.tenantID, &item.orderID, &item.providerNo, &item.status, &item.orderPaymentStatus, &item.providerRequestNo, &item.amount, &item.merchantNo) == nil {
			pending = append(pending, item)
		}
	}
	rows.Close()
	for _, item := range pending {
		if item.status == string(provider.PaymentSuccess) && item.orderPaymentStatus == "UNPAID" {
			if err = s.markPaymentPaid(ctx, s.Payment.Name(), item.providerNo, time.Now()); err != nil {
				_, _ = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=?", item.id)
				s.Logger.Error("finish locally successful payment", "payment_id", item.id, "error", err)
			}
			continue
		}
		if item.status == paymentStatusCreating {
			conn, release, lockErr := s.acquirePaymentOrderLock(ctx, item.tenantID, item.orderID)
			if lockErr != nil {
				s.Logger.Warn("lock creating payment during reconciliation", "payment_id", item.id, "error", lockErr)
				continue
			}
			var currentStatus, orderStatus string
			lockErr = conn.QueryRowContext(ctx, `SELECT p.status,o.status FROM payment_transactions p JOIN orders o ON o.id=p.order_id
				WHERE p.id=? AND p.tenant_id=?`, item.id, item.tenantID).Scan(&currentStatus, &orderStatus)
			if lockErr == nil && currentStatus == paymentStatusCreating && orderStatus == "PENDING_PAYMENT" {
				intent, loadErr := s.loadPaymentCreationIntent(ctx, conn, item.id)
				if loadErr == nil {
					_, loadErr = s.submitPaymentIntent(ctx, conn, intent)
				}
				if loadErr != nil {
					_, _ = conn.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=? AND status='CREATING'", item.id)
					s.Logger.Warn("resume creating payment", "payment_id", item.id, "error", loadErr)
				}
			} else if lockErr == nil && currentStatus == paymentStatusCreating {
				// A process may have reached the provider but crashed before storing
				// its response. Re-submit the stable provider idempotency key before
				// closing; blindly marking the local row CLOSED could otherwise hide
				// a payment that actually succeeded at the provider.
				lockErr = s.closePendingPaymentLocked(ctx, conn, item.tenantID, item.orderID)
				if errors.Is(lockErr, errPaymentAlreadyPaid) {
					lockErr = nil
				}
			}
			release()
			if lockErr != nil {
				s.Logger.Error("inspect creating payment", "payment_id", item.id, "error", lockErr)
			}
			continue
		}
		queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		result, queryErr := s.Payment.Query(queryCtx, item.providerNo)
		cancel()
		if queryErr != nil {
			_, _ = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=? AND status='PENDING'", item.id)
			s.Logger.Warn("query payment during reconciliation", "provider_order_no", item.providerNo, "error", queryErr)
			continue
		}
		if result.ProviderOrderNo != item.providerNo || result.OrderNo != item.providerRequestNo || result.Amount != item.amount || result.MerchantNo != item.merchantNo {
			_, _ = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=? AND status='PENDING'", item.id)
			s.Logger.Error("provider payment identity mismatch", "payment_id", item.id, "provider_order_no", item.providerNo)
			continue
		}
		switch result.Status {
		case provider.PaymentSuccess:
			paidAt := time.Now()
			if result.PaidAt != nil {
				paidAt = *result.PaidAt
			}
			if err = s.markPaymentPaid(ctx, s.Payment.Name(), item.providerNo, paidAt); err != nil {
				_, _ = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=?", item.id)
				s.Logger.Error("finalize reconciled payment", "provider_order_no", item.providerNo, "error", err)
			}
		case provider.PaymentFailed, provider.PaymentClosed:
			_, err = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET status=? WHERE id=? AND provider=? AND status='PENDING'", string(result.Status), item.id, s.Payment.Name())
			if err != nil {
				s.Logger.Error("record reconciled payment status", "provider_order_no", item.providerNo, "error", err)
			}
		default:
			_, _ = s.DB.ExecContext(ctx, "UPDATE payment_transactions SET updated_at=NOW(3) WHERE id=? AND status='PENDING'", item.id)
		}
	}
}

// StartRefundReconciler completes local accounting after an ambiguous refund
// timeout or a successful provider response followed by a local DB failure.
func (s *Server) StartRefundReconciler(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.reconcileRefunds(ctx)
			}
		}
	}()
}

func (s *Server) reconcileRefunds(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id,r.refund_no,r.provider_refund_no,p.provider_order_no,r.amount_cents,p.merchant_no FROM refunds r
		JOIN payment_transactions p ON p.id=r.payment_id
		WHERE p.provider=? AND r.status='PENDING' ORDER BY r.updated_at,r.id LIMIT 100`, s.Payment.Name())
	if err != nil {
		s.Logger.Error("list refunds for reconciliation", "error", err)
		return
	}
	type pendingRefund struct {
		id               int64
		amount           int64
		refundNo         string
		providerRefundNo string
		providerOrderNo  string
		merchantNo       string
	}
	var pending []pendingRefund
	for rows.Next() {
		var item pendingRefund
		if rows.Scan(&item.id, &item.refundNo, &item.providerRefundNo, &item.providerOrderNo, &item.amount, &item.merchantNo) == nil {
			pending = append(pending, item)
		}
	}
	rows.Close()
	for _, item := range pending {
		if item.providerRefundNo != "" {
			if err = s.finalizeRefund(ctx, item.id, item.providerRefundNo); err != nil {
				_, _ = s.DB.ExecContext(ctx, "UPDATE refunds SET last_error=?,updated_at=NOW(3) WHERE id=? AND status='PENDING'", truncateError(err), item.id)
			}
			continue
		}
		queryCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		result, queryErr := s.Payment.QueryRefund(queryCtx, item.refundNo)
		cancel()
		if queryErr != nil {
			// RefundNo is a mandatory provider-side idempotency key. Re-submit the
			// same durable intent after an ambiguous/not-found query so a crash
			// between local commit and the first provider call cannot strand it.
			refundCtx, refundCancel := context.WithTimeout(ctx, 12*time.Second)
			resubmitted, refundErr := s.Payment.Refund(refundCtx, provider.RefundRequest{MerchantNo: item.merchantNo, ProviderOrderNo: item.providerOrderNo, RefundNo: item.refundNo, Amount: item.amount})
			refundCancel()
			if refundErr != nil {
				_, _ = s.DB.ExecContext(ctx, "UPDATE refunds SET last_error=?,updated_at=NOW(3) WHERE id=? AND status='PENDING'", truncateError(refundErr), item.id)
				continue
			}
			result = provider.QueryRefundResult{RefundNo: item.refundNo, ProviderRefundNo: resubmitted.ProviderRefundNo, ProviderOrderNo: item.providerOrderNo, MerchantNo: item.merchantNo, Amount: item.amount, Status: resubmitted.Status}
		}
		identityMismatch := result.RefundNo != item.refundNo || result.ProviderOrderNo != item.providerOrderNo || result.MerchantNo != item.merchantNo || result.Amount != item.amount
		successWithoutProviderNo := (result.Status == provider.PaymentSuccess || result.Status == provider.PaymentRefunded) && result.ProviderRefundNo == ""
		if identityMismatch || successWithoutProviderNo {
			_, _ = s.DB.ExecContext(ctx, "UPDATE refunds SET last_error='provider refund identity mismatch',updated_at=NOW(3) WHERE id=? AND status='PENDING'", item.id)
			s.Logger.Error("provider refund identity mismatch", "refund_id", item.id, "refund_no", item.refundNo)
			continue
		}
		switch result.Status {
		case provider.PaymentSuccess, provider.PaymentRefunded:
			if err = s.finalizeRefund(ctx, item.id, result.ProviderRefundNo); err != nil {
				_, _ = s.DB.ExecContext(ctx, "UPDATE refunds SET provider_refund_no=?,last_error=?,updated_at=NOW(3) WHERE id=? AND status='PENDING'", result.ProviderRefundNo, truncateError(err), item.id)
				s.Logger.Error("finalize reconciled refund", "refund_id", item.id, "error", err)
			}
		case provider.PaymentFailed, provider.PaymentClosed:
			_, err = s.DB.ExecContext(ctx, "UPDATE refunds SET status='FAILED',provider_refund_no=?,last_error='provider rejected refund' WHERE id=? AND status='PENDING'", result.ProviderRefundNo, item.id)
			if err != nil {
				s.Logger.Error("record reconciled refund status", "refund_id", item.id, "error", err)
			}
		default:
			_, _ = s.DB.ExecContext(ctx, "UPDATE refunds SET last_error='',updated_at=NOW(3) WHERE id=? AND status='PENDING'", item.id)
		}
	}
}
