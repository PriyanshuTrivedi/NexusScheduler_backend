package store

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/booking/entity"
)

// tableBooking is the one table this store owns. Booking's audit trail is
// the row history itself (reschedule chains via parent_booking_id, status
// transitions on the row) plus the Kafka event each mutation already
// publishes via client — a separate booking_event table duplicated both
// with no consumer ever reading from it, so it was dropped.
const tableBooking = "booking"

var (
	ErrSlotAlreadyBooked = errors.New("store: resource already booked for this window")
	ErrNotFound          = errors.New("store: booking not found")
)

//go:generate mockgen -source=booking.go -destination=../../../gen/mocks/booking/store/booking_mock.go -package=mocks

type Store interface {
	CreateBooking(ctx context.Context, b entity.Booking) (*entity.Booking, error)
	CancelBooking(ctx context.Context, referenceCode string) (entity.BookingStatus, error)
	RescheduleBooking(ctx context.Context, referenceCode string, newStart, newEnd int64) (*entity.Booking, error)
	GetBooking(ctx context.Context, referenceCode string) (*entity.Booking, error)
	ListUserBookings(ctx context.Context, userID string, upcoming bool) ([]*entity.Booking, error)
	ListResourceBookings(ctx context.Context, resourceID string, upcoming bool) ([]*entity.Booking, error)
}

type pgStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

func genReferenceCode() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	code := strings.ToUpper(base32.StdEncoding.EncodeToString(b))[:8]
	return "NXS-" + code
}

func (s *pgStore) CreateBooking(ctx context.Context, b entity.Booking) (*entity.Booking, error) {
	refCode := genReferenceCode()
	var id string
	// Partial unique index on (resource_id, start_time) WHERE status IN
	// ('confirmed','waitlisted') is what actually prevents double
	// booking — this insert failing on conflict IS that guarantee. Now a
	// single statement (the booking_event insert that used to follow it
	// is gone), so no transaction wrapper is needed here anymore.
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (user_id, resource_id, start_time, end_time, title, subtitle, status, reference_code)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`, tableBooking), b.UserID, b.ResourceID, b.Start, b.End, b.Title, b.Subtitle, entity.StatusConfirmed.String(), refCode).Scan(&id)
	if err != nil {
		return nil, ErrSlotAlreadyBooked
	}

	b.ID, b.ReferenceCode, b.Status = id, refCode, entity.StatusConfirmed
	return &b, nil
}

func (s *pgStore) CancelBooking(ctx context.Context, referenceCode string) (entity.BookingStatus, error) {
	var status string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s SET status = $1, updated_at = now()
		WHERE reference_code = $2 AND status IN ($3, $4)
		RETURNING status
	`, tableBooking), entity.StatusCancelled.String(), referenceCode,
		entity.StatusConfirmed.String(), entity.StatusWaitlisted.String()).Scan(&status)

	if err == nil {
		return entity.StatusCancelled, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return entity.StatusUnspecified, err
	}

	// Zero rows updated is ambiguous — distinguish "already cancelled"
	// (idempotent success) from "never existed" (real NotFound).
	var existing string
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s WHERE reference_code = $1`, tableBooking), referenceCode).Scan(&existing)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.StatusUnspecified, ErrNotFound
	}
	if err != nil {
		return entity.StatusUnspecified, err
	}
	return entity.StatusCancelled, nil // already cancelled — idempotent no-op
}

func (s *pgStore) RescheduleBooking(ctx context.Context, referenceCode string, newStart, newEnd int64) (*entity.Booking, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var old entity.Booking
	var oldStatus string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, user_id, resource_id, title, subtitle, status
		FROM %s WHERE reference_code = $1
	`, tableBooking), referenceCode).Scan(&old.ID, &old.UserID, &old.ResourceID, &old.Title, &old.Subtitle, &oldStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	parsed := entity.ParseBookingStatus(oldStatus)
	if parsed != entity.StatusConfirmed && parsed != entity.StatusWaitlisted {
		return nil, ErrNotFound
	}

	// The old row flipping to 'rescheduled' and staying in place — rather
	// than being deleted — is what preserves reschedule lineage now that
	// there's no separate booking_event log: the chain of rows linked by
	// parent_booking_id is the history.
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET status = $1, updated_at = now() WHERE id = $2
	`, tableBooking), entity.StatusRescheduled.String(), old.ID); err != nil {
		return nil, err
	}

	var newID string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (user_id, resource_id, start_time, end_time, title, subtitle, status, reference_code, parent_booking_id)
		VALUES ($1, $2, to_timestamp($3), to_timestamp($4), $5, $6, $7, $8, $9)
		RETURNING id
	`, tableBooking), old.UserID, old.ResourceID, newStart, newEnd, old.Title, old.Subtitle,
		entity.StatusConfirmed.String(), referenceCode, old.ID).Scan(&newID)
	if err != nil {
		return nil, ErrSlotAlreadyBooked // the new slot is taken — same constraint, reused
	}

	return &entity.Booking{
		ID: newID, ReferenceCode: referenceCode, UserID: old.UserID, ResourceID: old.ResourceID,
		Title: old.Title, Subtitle: old.Subtitle, Status: entity.StatusConfirmed,
		ParentBookingID: old.ID,
	}, tx.Commit(ctx)
}

func (s *pgStore) ListUserBookings(ctx context.Context, userID string, upcoming bool) ([]*entity.Booking, error) {
	condition := "start_time < now()"
	order := "DESC"
	if upcoming {
		condition = "start_time >= now() AND status IN ('confirmed', 'waitlisted')"
		order = "ASC"
	} else {
		condition = "start_time < now() OR status IN ('cancelled', 'failed', 'rescheduled')"
	}
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT id, reference_code, user_id, resource_id, start_time, end_time, title, subtitle, status FROM %s WHERE user_id = $1 AND %s ORDER BY start_time %s`, tableBooking, condition, order), userID)
	if err != nil {
		return nil, fmt.Errorf("store: list user bookings: %w", err)
	}
	defer rows.Close()
	bookings := make([]*entity.Booking, 0)
	for rows.Next() {
		var b entity.Booking
		var status string
		if err := rows.Scan(&b.ID, &b.ReferenceCode, &b.UserID, &b.ResourceID, &b.Start, &b.End, &b.Title, &b.Subtitle, &status); err != nil {
			return nil, fmt.Errorf("store: scan user booking: %w", err)
		}
		b.Status = entity.ParseBookingStatus(status)
		bookings = append(bookings, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list user bookings rows: %w", err)
	}
	return bookings, nil
}

func (s *pgStore) ListResourceBookings(ctx context.Context, resourceID string, upcoming bool) ([]*entity.Booking, error) {
	condition := "start_time < now()"
	order := "DESC"
	if upcoming {
		condition = "start_time >= now() AND status IN ('confirmed','waitlisted')"
		order = "ASC"
	} else {
		condition = "start_time < now() OR status IN ('cancelled','failed','rescheduled')"
	}
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT id,reference_code,user_id,resource_id,start_time,end_time,title,subtitle,status FROM %s WHERE resource_id=$1 AND %s ORDER BY start_time %s`, tableBooking, condition, order), resourceID)
	if err != nil {
		return nil, fmt.Errorf("store: list resource bookings: %w", err)
	}
	defer rows.Close()
	out := make([]*entity.Booking, 0)
	for rows.Next() {
		var b entity.Booking
		var st string
		if err := rows.Scan(&b.ID, &b.ReferenceCode, &b.UserID, &b.ResourceID, &b.Start, &b.End, &b.Title, &b.Subtitle, &st); err != nil {
			return nil, fmt.Errorf("store: scan resource booking: %w", err)
		}
		b.Status = entity.ParseBookingStatus(st)
		out = append(out, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list resource bookings rows: %w", err)
	}
	return out, nil
}

func (s *pgStore) GetBooking(ctx context.Context, referenceCode string) (*entity.Booking, error) {
	var b entity.Booking
	var status string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, reference_code, user_id, resource_id, start_time, end_time, title, subtitle, status
		FROM %s WHERE reference_code = $1
		ORDER BY created_at DESC LIMIT 1
	`, tableBooking), referenceCode).Scan(&b.ID, &b.ReferenceCode, &b.UserID, &b.ResourceID, &b.Start, &b.End, &b.Title, &b.Subtitle, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.Status = entity.ParseBookingStatus(status)
	return &b, nil
}
