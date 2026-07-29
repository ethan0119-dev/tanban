package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	printOutboxBatchSize   = 20
	printOutboxMaxAttempts = 8
)

type printOutboxEvent struct {
	ID        int64
	TenantID  int64
	StoreID   int64
	OrderID   int64
	EventType string
	ActorID   int64
	Extra     string
	Status    string
	Attempts  int
	Available time.Time
}

type printOutboxEnqueuer func(context.Context, sqlQueryExecer, int64, int64, int64, string, bool, int64, string) error

// enqueuePrintOutboxWith records only the immutable business fact that may
// cause printing. Template loading, rendering and print-job creation happen
// after the surrounding order/payment/refund transaction has committed.
func enqueuePrintOutboxWith(ctx context.Context, executor sqlExecer, tenantID, storeID, orderID int64, eventType, dedupeKey string, actorID int64, extra string) error {
	eventType = strings.ToUpper(strings.TrimSpace(eventType))
	dedupeKey = strings.TrimSpace(dedupeKey)
	if tenantID <= 0 || storeID <= 0 || orderID <= 0 {
		return errors.New("print outbox tenant, store and order are required")
	}
	if !validStatus(eventType, "ORDER_CREATED", "PAYMENT_SUCCESS", "REFUND") {
		return fmt.Errorf("unsupported print outbox event %q", eventType)
	}
	if dedupeKey == "" || len(dedupeKey) > 160 {
		return errors.New("print outbox dedupe key is required and must not exceed 160 bytes")
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO print_outbox(tenant_id,store_id,order_id,event_type,dedupe_key,actor_id,extra_text,status,available_at)
		VALUES(?,?,?,?,?,?,?,'PENDING',NOW(3))
		ON DUPLICATE KEY UPDATE id=id`, tenantID, storeID, orderID, eventType, dedupeKey, actorID, extra)
	return err
}

func paymentPrintDedupeKey(paymentID int64) string {
	return fmt.Sprintf("PAYMENT:%d", paymentID)
}

func refundPrintDedupeKey(refundID int64) string {
	return fmt.Sprintf("REFUND:%d", refundID)
}

func orderCreatedPrintDedupeKey(orderID int64) string {
	return fmt.Sprintf("ORDER:%d:ORDER_CREATED", orderID)
}

// processPendingPrintOutbox is deliberately run before print-job delivery.
// Jobs created from committed facts can therefore be sent on the same tick.
func (s *Server) processPendingPrintOutbox(ctx context.Context) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM print_outbox
		WHERE status='PENDING' AND available_at<=NOW(3)
		ORDER BY available_at,id LIMIT ?`, printOutboxBatchSize)
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("list pending print outbox", "error", err)
		}
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if scanErr := rows.Scan(&id); scanErr != nil {
			err = scanErr
			break
		}
		ids = append(ids, id)
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if s.Logger != nil {
			s.Logger.Error("scan pending print outbox", "error", err)
		}
		return
	}
	for _, id := range ids {
		if eventErr := s.processPrintOutboxEvent(ctx, id); eventErr != nil && s.Logger != nil {
			s.Logger.Error("process print outbox", "outbox_id", id, "error", eventErr)
		}
	}
}

func (s *Server) processPrintOutboxEvent(ctx context.Context, id int64) error {
	return s.processPrintOutboxEventWith(ctx, id, func(ctx context.Context, executor sqlQueryExecer, tenantID, storeID, orderID int64, event string, reprint bool, actorID int64, extra string) error {
		return s.enqueueOrderPrintsWith(ctx, executor, tenantID, storeID, orderID, event, reprint, actorID, extra)
	})
}

// processPrintOutboxEventWith keeps the event lock, generated print_jobs and
// DONE transition in one transaction. A savepoint lets a failed generator
// discard every partial job while committing retry metadata for the event.
func (s *Server) processPrintOutboxEventWith(ctx context.Context, id int64, enqueue printOutboxEnqueuer) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var event printOutboxEvent
	err = tx.QueryRowContext(ctx, `SELECT id,tenant_id,store_id,order_id,event_type,actor_id,extra_text,status,attempts,available_at
		FROM print_outbox WHERE id=? FOR UPDATE`, id).
		Scan(&event.ID, &event.TenantID, &event.StoreID, &event.OrderID, &event.EventType, &event.ActorID, &event.Extra, &event.Status, &event.Attempts, &event.Available)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if event.Status != "PENDING" || event.Available.After(time.Now()) {
		return nil
	}

	attempt := event.Attempts + 1
	result, err := tx.ExecContext(ctx, "UPDATE print_outbox SET status='PROCESSING',attempts=? WHERE id=? AND status='PENDING'", attempt, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return nil
	}
	if _, err = tx.ExecContext(ctx, "SAVEPOINT print_outbox_enqueue"); err != nil {
		return err
	}

	enqueueErr := enqueue(ctx, tx, event.TenantID, event.StoreID, event.OrderID, event.EventType, false, event.ActorID, event.Extra)
	if enqueueErr == nil {
		if _, err = tx.ExecContext(ctx, `UPDATE print_outbox
			SET status='DONE',last_error='',processed_at=NOW(3)
			WHERE id=? AND status='PROCESSING'`, id); err != nil {
			return err
		}
		return tx.Commit()
	}

	if _, err = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT print_outbox_enqueue"); err != nil {
		return fmt.Errorf("rollback partial print jobs after %v: %w", enqueueErr, err)
	}
	errorText := truncateError(enqueueErr)
	if attempt >= printOutboxMaxAttempts {
		if _, err = tx.ExecContext(ctx, `UPDATE print_outbox
			SET status='DEAD',last_error=?,processed_at=NOW(3)
			WHERE id=? AND status='PROCESSING'`, errorText, id); err != nil {
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		if s.Logger != nil {
			s.Logger.Error("print outbox reached retry limit", "outbox_id", id, "order_id", event.OrderID, "event_type", event.EventType, "attempts", attempt, "error", enqueueErr)
		}
		return nil
	}

	nextAttempt := time.Now().Add(printOutboxBackoff(attempt))
	if _, err = tx.ExecContext(ctx, `UPDATE print_outbox
		SET status='PENDING',available_at=?,last_error=?,processed_at=NULL
		WHERE id=? AND status='PROCESSING'`, nextAttempt, errorText, id); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if s.Logger != nil {
		s.Logger.Warn("print outbox scheduled for retry", "outbox_id", id, "order_id", event.OrderID, "event_type", event.EventType, "attempts", attempt, "available_at", nextAttempt, "error", enqueueErr)
	}
	return nil
}

func printOutboxBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 9 {
		attempt = 9
	}
	return time.Duration(1<<attempt) * time.Second
}
