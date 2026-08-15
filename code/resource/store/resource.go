package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
)

const tableResource, tableResourceType, tableRecurrenceRule, tableSlot = "resource", "resource_type", "recurrence_rule", "slot"

// Store owns resource_type, resource, recurrence_rule, and slot exclusively.
var (
	ErrResourceNotFound           = errors.New("store: resource not found")
	ErrResourceTypeNotFound       = errors.New("store: resource type not found")
	ErrResourceTypeInUse          = errors.New("store: resource type is still used by resources")
	ErrResourceTypeMustBeInactive = errors.New("store: resource type must be inactive before deletion")
	ErrResourceMustBeInactive     = errors.New("store: resource must be inactive before deletion")
	ErrResourceUnavailable        = errors.New("store: resource is inactive")
	ErrResourceTypeExists         = errors.New("store: resource type already exists")
	ErrSlotNotFound               = errors.New("store: slot not found")
	ErrSlotAlreadyExists          = errors.New("store: a slot already exists at that resource and start time")
)

//go:generate mockgen -source=resource.go -destination=../../../gen/mocks/resource/store/resource_mock.go -package=mocks

type Store interface {
	ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error)
	CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error)
	SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error)
	DeleteResourceType(ctx context.Context, resourceTypeID string) error
	CreateResource(ctx context.Context, r entity.Resource, slots []entity.Slot) (string, error)
	SetResourceStatus(ctx context.Context, resourceID string, isActive bool) (*string, error)
	DeleteResource(ctx context.Context, resourceID string) (*string, error)
	ReplaceRecurrence(ctx context.Context, resourceID string, rules []entity.RecurrenceRule, slots []entity.Slot, regenerateFrom time.Time) (int, error)
	AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error)
	BlockSlot(ctx context.Context, slotID, reason string) (entity.SlotStatus, error)
	SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (int, error)
	SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error)
	GetSlot(ctx context.Context, slotID string) (entity.Slot, error)
	GetResourceOrgID(ctx context.Context, resourceID string) (*string, error)
}

type pgStore struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) Store {
	return &pgStore{pool: pool}
}

func (s *pgStore) ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, name, is_active
		FROM %s
		WHERE is_active = TRUE
		ORDER BY lower(name), id
	`, tableResourceType))
	if err != nil {
		return nil, fmt.Errorf("store: list resource types: %w", err)
	}
	defer rows.Close()

	types := make([]entity.ResourceType, 0)
	for rows.Next() {
		var rt entity.ResourceType
		if err := rows.Scan(&rt.ID, &rt.Name, &rt.IsActive); err != nil {
			return nil, fmt.Errorf("store: scan resource type: %w", err)
		}
		types = append(types, rt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list resource types rows: %w", err)
	}
	return types, nil
}

func (s *pgStore) CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(
		`SELECT EXISTS (SELECT 1 FROM %s WHERE lower(name) = lower($1))`,
		tableResourceType,
	), name).Scan(&exists); err != nil {
		return entity.ResourceType{}, fmt.Errorf("store: check resource type: %w", err)
	}
	if exists {
		return entity.ResourceType{}, ErrResourceTypeExists
	}

	var rt entity.ResourceType
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (name)
		VALUES ($1)
		RETURNING id, name, is_active
	`, tableResourceType), name).Scan(&rt.ID, &rt.Name, &rt.IsActive)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.ResourceType{}, ErrResourceTypeExists
		}
		return entity.ResourceType{}, fmt.Errorf("store: create resource type: %w", err)
	}
	return rt, nil
}

func (s *pgStore) SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error) {
	var rt entity.ResourceType
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s SET is_active = $2, updated_at = now()
		WHERE id = $1
		RETURNING id, name, is_active
	`, tableResourceType), resourceTypeID, isActive).Scan(&rt.ID, &rt.Name, &rt.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.ResourceType{}, ErrResourceTypeNotFound
	}
	if err != nil {
		return entity.ResourceType{}, fmt.Errorf("store: set resource type status: %w", err)
	}
	return rt, nil
}

func (s *pgStore) DeleteResourceType(ctx context.Context, resourceTypeID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var typeID string
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, is_active FROM %s WHERE id = $1 FOR UPDATE
	`, tableResourceType), resourceTypeID).Scan(&typeID, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrResourceTypeNotFound
		}
		return fmt.Errorf("store: check resource type: %w", err)
	}

	var resourceCount int
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s WHERE resource_type_id = $1
	`, tableResource), resourceTypeID).Scan(&resourceCount); err != nil {
		return fmt.Errorf("store: count resources for type: %w", err)
	}
	if active {
		return ErrResourceTypeMustBeInactive
	}
	if resourceCount > 0 {
		return ErrResourceTypeInUse
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, tableResourceType), resourceTypeID); err != nil {
		return fmt.Errorf("store: delete resource type: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("store: commit tx: %w", err)
	}
	return nil
}

func (s *pgStore) CreateResource(ctx context.Context, r entity.Resource, slots []entity.Slot) (string, error) {
	attrJSON, err := json.Marshal(r.Attributes)
	if err != nil {
		return "", fmt.Errorf("store: marshal attributes: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var typeID string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s WHERE id = $1 AND is_active = TRUE FOR SHARE
	`, tableResourceType), r.ResourceTypeID).Scan(&typeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrResourceTypeNotFound
		}
		return "", fmt.Errorf("store: check resource type: %w", err)
	}

	var resourceID string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (tenant_type, org_id, user_id, resource_type_id, name, meeting_mode, location, attributes, is_active)
		VALUES ($1, NULLIF($2, '')::uuid, NULLIF($3, '')::uuid, $4, $5, $6,
			CASE WHEN $7::double precision IS NULL THEN NULL
			     ELSE ST_SetSRID(ST_MakePoint($7, $8), 4326)::geography END,
			$9::jsonb, TRUE)
		RETURNING id`, tableResource),
		r.TenantType.String(), r.OrgID, r.UserID, r.ResourceTypeID, r.Name, int32(r.MeetingMode), r.Longitude, r.Latitude, attrJSON,
	).Scan(&resourceID)
	if err != nil {
		return "", fmt.Errorf("store: insert resource: %w", err)
	}

	if err := insertRecurrenceRules(ctx, tx, resourceID, r.Recurrence); err != nil {
		return "", err
	}
	if err := bulkInsertSlots(ctx, tx, resourceID, slots); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("store: commit tx: %w", err)
	}
	return resourceID, nil
}

// UpdateResource changes only fields that a resource owner is allowed to edit.
// Resource type, tenant type, and organization are intentionally immutable.
func (s *pgStore) UpdateResource(ctx context.Context, userID, name string, mode entity.MeetingMode, lat, lng *float64, attributes map[string]string) (string, error) {
	if userID == "" {
		return "", entity.ErrInvalidUserID
	}
	if name == "" {
		return "", entity.ErrInvalidName
	}
	if !mode.Valid() {
		return "", entity.ErrInvalidMeetingMode
	}
	if mode.RequiresLocation() && (lat == nil || lng == nil) {
		return "", entity.ErrLocationRequired
	}

	attrJSON, err := json.Marshal(attributes)
	if err != nil {
		return "", fmt.Errorf("store: marshal resource attributes: %w", err)
	}

	var resourceID string
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s
		SET name = $2,
		    meeting_mode = $3,
		    location = CASE
		        WHEN $4::double precision IS NULL OR $5::double precision IS NULL
		            THEN NULL
		        ELSE ST_SetSRID(ST_MakePoint($4, $5), 4326)::geography
		    END,
		    attributes = $6::jsonb,
		    updated_at = now()
		WHERE user_id = $1
		RETURNING id
	`, tableResource), userID, name, int32(mode), lng, lat, attrJSON).Scan(&resourceID)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrResourceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("store: update resource: %w", err)
	}
	return resourceID, nil
}

func (s *pgStore) SetResourceStatus(ctx context.Context, resourceID string, isActive bool) (*string, error) {
	var orgID string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`UPDATE %s SET is_active = $2, updated_at = now() WHERE id = $1 RETURNING COALESCE(org_id::text, '')`, tableResource), resourceID, isActive).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: set resource status: %w", err)
	}
	if orgID == "" {
		return nil, nil
	}
	return &orgID, nil
}

func (s *pgStore) DeleteResource(ctx context.Context, resourceID string) (*string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var orgID string
	var active bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(org_id::text, ''), is_active FROM %s WHERE id = $1 FOR UPDATE
	`, tableResource), resourceID).Scan(&orgID, &active); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrResourceNotFound
		}
		return nil, fmt.Errorf("store: get resource for deletion: %w", err)
	}
	if active {
		return nil, ErrResourceMustBeInactive
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, tableResource), resourceID); err != nil {
		return nil, fmt.Errorf("store: delete resource: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("store: commit tx: %w", err)
	}
	if orgID == "" {
		return nil, nil
	}
	return &orgID, nil
}

func (s *pgStore) ReplaceRecurrence(ctx context.Context, resourceID string, rules []entity.RecurrenceRule, slots []entity.Slot, regenerateFrom time.Time) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE resource_id = $1`, tableRecurrenceRule), resourceID); err != nil {
		return 0, fmt.Errorf("store: clear recurrence rules: %w", err)
	}
	if err := insertRecurrenceRules(ctx, tx, resourceID, rules); err != nil {
		return 0, err
	}

	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		DELETE FROM %s WHERE resource_id = $1 AND source = 'recurring' AND status = 1 AND start_time >= $2
	`, tableSlot), resourceID, regenerateFrom); err != nil {
		return 0, fmt.Errorf("store: clear future recurring slots: %w", err)
	}

	if err := bulkInsertSlots(ctx, tx, resourceID, slots); err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("store: commit tx: %w", err)
	}
	return len(slots), nil
}

func (s *pgStore) AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error) {
	var slot entity.Slot
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (resource_id, start_time, end_time, status, source, reason)
		VALUES ($1, $2, $3, 1, 'exception', $4)
		RETURNING id, resource_id, start_time, end_time, status
	`, tableSlot), se.ResourceID, se.Start, se.End, se.Reason,
	).Scan(&slot.ID, &slot.ResourceID, &slot.Start, &slot.End, &slot.Status)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.Slot{}, ErrSlotAlreadyExists
		}
		return entity.Slot{}, fmt.Errorf("store: insert slot exception: %w", err)
	}
	return slot, nil
}

func (s *pgStore) BlockSlot(ctx context.Context, slotID, reason string) (entity.SlotStatus, error) {
	var status entity.SlotStatus
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s SET status = 4, reason = $2, updated_at = now() WHERE id = $1 RETURNING status
	`, tableSlot), slotID, reason).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.SlotStatusUnspecified, ErrSlotNotFound
	}
	if err != nil {
		return entity.SlotStatusUnspecified, fmt.Errorf("store: block slot: %w", err)
	}
	return status, nil
}

func (s *pgStore) SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (int, error) {
	tag, err := s.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s SET status = 4, reason = $4, updated_at = now()
		WHERE resource_id = $1 AND status = 1 AND start_time >= $2 AND start_time < $3
	`, tableSlot), lp.ResourceID, lp.Start, lp.End, lp.Reason)
	if err != nil {
		return 0, fmt.Errorf("store: set leave period: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (s *pgStore) SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error) {
	userID := ""
	resourceID := ""
	attrs := make(map[string]string, len(req.Attributes))
	for k, v := range req.Attributes {
		if k == "__user_id" {
			userID = v
			continue
		}
		if k == "__resource_id" {
			resourceID = v
			continue
		}
		if k == "__include_recurrence" {
			continue
		}
		attrs[k] = v
	}
	attrJSON, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("store: marshal attribute filter: %w", err)
	}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT r.id, r.name, rt.id, rt.name, rt.is_active, r.meeting_mode, r.attributes,
			r.tenant_type, COALESCE(r.org_id::text, ''),
			CASE WHEN $9::double precision > 0 AND r.location IS NOT NULL
			     THEN ST_Distance(r.location, ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography) / 1000.0
			     ELSE 0 END AS distance_km
		FROM %s r
		JOIN %s rt ON rt.id = r.resource_type_id
		WHERE ($1::smallint = 0 OR r.tenant_type = CASE $1::smallint WHEN 1 THEN 'individual' WHEN 2 THEN 'org' END)
		  AND ($2 = '' OR r.org_id = NULLIF($2, '')::uuid)
		  AND r.is_active = TRUE AND rt.is_active = TRUE
		  AND ($3 = '' OR r.name ILIKE '%%' || $3 || '%%')
		  AND ($4 = '' OR r.resource_type_id::text = $4)
		  AND ($5::smallint = 0 OR r.meeting_mode = $5)
		  AND ($6::jsonb = '{}'::jsonb OR r.attributes @> $6::jsonb)
		  AND ($9::double precision <= 0 OR (
		        r.location IS NOT NULL AND ST_DWithin(
		          r.location, ST_SetSRID(ST_MakePoint($8, $7), 4326)::geography, $9 * 1000
		        )
		  ))
		  AND ($10 = '' OR r.user_id::text = $10)
		  AND ($11 = '' OR r.id::text = $11)
		ORDER BY r.name LIMIT 50
	`, tableResource, tableResourceType),
		int32(req.TenantType), req.OrgID, req.Name, req.ResourceTypeID, int32(req.MeetingMode), attrJSON, req.Latitude, req.Longitude, req.RadiusKM, userID, resourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: search resources: %w", err)
	}
	defer rows.Close()

	var summaries []entity.ResourceSummary
	for rows.Next() {
		var sm entity.ResourceSummary
		var attr []byte
		var tenantType string
		var orgID string
		if err := rows.Scan(&sm.ResourceID, &sm.Name, &sm.ResourceType.ID, &sm.ResourceType.Name, &sm.ResourceType.IsActive, &sm.MeetingMode, &attr, &tenantType, &orgID, &sm.DistanceKM); err != nil {
			return nil, fmt.Errorf("store: scan resource row: %w", err)
		}
		sm.TenantType = entity.ParseTenantType(tenantType)
		sm.OrgID = orgID
		if err := json.Unmarshal(attr, &sm.Attributes); err != nil {
			return nil, fmt.Errorf("store: unmarshal attributes: %w", err)
		}
		summaries = append(summaries, sm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate search results: %w", err)
	}
	for i := range summaries {
		slots, err := nextOpenSlots(ctx, s.pool, summaries[i].ResourceID, req.WindowStart, req.WindowEnd, 21)
		if err != nil {
			return nil, err
		}
		summaries[i].NextAvailableSlots = slots
		summaries[i].IsActive = true
	}
	return summaries, nil
}

func nextOpenSlots(ctx context.Context, pool *pgxpool.Pool, resourceID string, windowStart, windowEnd time.Time, limit int) ([]entity.Slot, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT id, resource_id, start_time, end_time, status FROM %s
		WHERE resource_id = $1 AND status = 1
		  AND ($2::timestamptz IS NULL OR start_time >= $2)
		  AND ($3::timestamptz IS NULL OR start_time < $3)
		ORDER BY start_time LIMIT $4
	`, tableSlot), resourceID, nullIfZero(windowStart), nullIfZero(windowEnd), limit)
	if err != nil {
		return nil, fmt.Errorf("store: next open slots: %w", err)
	}
	defer rows.Close()

	var slots []entity.Slot
	for rows.Next() {
		var sl entity.Slot
		if err := rows.Scan(&sl.ID, &sl.ResourceID, &sl.Start, &sl.End, &sl.Status); err != nil {
			return nil, fmt.Errorf("store: scan slot row: %w", err)
		}
		slots = append(slots, sl)
	}
	return slots, rows.Err()
}

func (s *pgStore) GetSlot(ctx context.Context, slotID string) (entity.Slot, error) {
	var sl entity.Slot
	var resourceActive, typeActive bool
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT s.id, s.resource_id, s.start_time, s.end_time, s.status,
		       r.is_active, rt.is_active
		FROM %s s
		JOIN %s r ON r.id = s.resource_id
		JOIN %s rt ON rt.id = r.resource_type_id
		WHERE s.id = $1
	`, tableSlot, tableResource, tableResourceType), slotID).Scan(&sl.ID, &sl.ResourceID, &sl.Start, &sl.End, &sl.Status, &resourceActive, &typeActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Slot{}, ErrSlotNotFound
	}
	if err != nil {
		return entity.Slot{}, fmt.Errorf("store: get slot: %w", err)
	}
	if !resourceActive || !typeActive {
		return entity.Slot{}, ErrResourceUnavailable
	}
	return sl, nil
}

func (s *pgStore) GetRecurrence(ctx context.Context, resourceID string) ([]entity.RecurrenceRule, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT day_of_week, timezone, slots
		FROM %s
		WHERE resource_id = $1
		ORDER BY day_of_week
	`, tableRecurrenceRule), resourceID)
	if err != nil {
		return nil, fmt.Errorf("store: get recurrence: %w", err)
	}
	defer rows.Close()

	rules := make([]entity.RecurrenceRule, 0)
	for rows.Next() {
		var day int32
		var timezone string
		var raw []byte
		if err := rows.Scan(&day, &timezone, &raw); err != nil {
			return nil, fmt.Errorf("store: scan recurrence: %w", err)
		}
		var slots []entity.TimeSlot
		if err := json.Unmarshal(raw, &slots); err != nil {
			return nil, fmt.Errorf("store: unmarshal recurrence slots: %w", err)
		}
		rules = append(rules, entity.RecurrenceRule{Day: entity.DayOfWeek(day), Timezone: timezone, Slots: slots})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate recurrence: %w", err)
	}
	return rules, nil
}

func (s *pgStore) GetResourceOrgID(ctx context.Context, resourceID string) (*string, error) {
	var orgID string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(org_id::text, '') FROM %s WHERE id = $1`, tableResource), resourceID).Scan(&orgID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrResourceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store: get resource org_id: %w", err)
	}
	if orgID == "" {
		return nil, nil
	}
	return &orgID, nil
}

func insertRecurrenceRules(ctx context.Context, tx pgx.Tx, resourceID string, rules []entity.RecurrenceRule) error {
	for _, rule := range rules {
		slotsJSON, err := json.Marshal(rule.Slots)
		if err != nil {
			return fmt.Errorf("store: marshal recurrence slots: %w", err)
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s (resource_id, day_of_week, timezone, slots)
			VALUES ($1, $2, $3, $4::jsonb)
			ON CONFLICT (resource_id, day_of_week) DO UPDATE
				SET timezone = EXCLUDED.timezone, slots = EXCLUDED.slots, updated_at = now()
		`, tableRecurrenceRule), resourceID, int32(rule.Day), rule.Timezone, slotsJSON); err != nil {
			return fmt.Errorf("store: upsert recurrence rule: %w", err)
		}
	}
	return nil
}

func bulkInsertSlots(ctx context.Context, tx pgx.Tx, resourceID string, slots []entity.Slot) error {
	if len(slots) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	q := fmt.Sprintf(`
		INSERT INTO %s (resource_id, start_time, end_time, status, source)
		VALUES ($1, $2, $3, 1, 'recurring')
		ON CONFLICT (resource_id, start_time) DO NOTHING
	`, tableSlot)
	for _, sl := range slots {
		batch.Queue(q, resourceID, sl.Start, sl.End)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range slots {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("store: bulk insert slots: %w", err)
		}
	}
	return nil
}

func nullIfZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key"))
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
