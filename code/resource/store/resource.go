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
	UpdateResource(ctx context.Context, r entity.Resource) (string, error)
	SetResourceStatus(ctx context.Context, resourceID string, isActive bool) (*string, error)
	DeleteResource(ctx context.Context, resourceID string) (*string, error)
	ReplaceRecurrence(ctx context.Context, resourceID string, rules []entity.RecurrenceRule, slots []entity.Slot, regenerateFrom time.Time) (int, error)
	AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error)
	BlockSlot(ctx context.Context, slotID, reason string) (entity.SlotStatus, error)
	SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (int, error)
	SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error)
	GetSlot(ctx context.Context, slotID string) (entity.Slot, error)
	GetResourceOrgID(ctx context.Context, resourceID string) (*string, error)
	GetResourceById(ctx context.Context, resourceID string) (entity.Resource, error)
	GetRecurrence(ctx context.Context, resourceID string) ([]entity.RecurrenceRule, error)
	GetSlotsByResourceId(ctx context.Context, resourceID string, start time.Time, end time.Time) ([]entity.Slot, error)
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
	var lat, lng *float64
	if r.MeetingMode.RequiresLocation() {
		_, ok := r.Attributes["address"]
		if r.Address == nil && r.Coordinate == nil || !ok {
			return "", fmt.Errorf("store: address not found")
		}
		lat = &r.Coordinate.Latitude
		lng = &r.Coordinate.Longitude
	}

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
		SELECT id FROM %s
		WHERE id = $1 AND is_active = TRUE
		FOR SHARE
	`, tableResourceType), r.ResourceType.ID).Scan(&typeID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrResourceTypeNotFound
		}
		return "", fmt.Errorf("store: check resource type: %w", err)
	}

	var resourceID string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			tenant_type,
			org_id,
			user_id,
			resource_type_id,
			name,
			meeting_mode,
			location,
			attributes,
			is_active
		)
		VALUES (
			$1,
			NULLIF($2, '')::uuid,
			NULLIF($3, '')::uuid,
			$4,
			$5,
			$6,
			CASE
				WHEN $7::double precision IS NULL
					OR $8::double precision IS NULL
				THEN NULL
				ELSE ST_SetSRID(
					ST_MakePoint($7, $8),
					4326
				)::geography
			END,
			$9::jsonb,
			TRUE
		)
		RETURNING id
	`, tableResource),
		r.TenantType.String(),
		r.OrgID,
		r.UserID,
		r.ResourceType.ID,
		r.Name,
		int32(r.MeetingMode),
		lng,
		lat,
		attrJSON,
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

func (s *pgStore) UpdateResource(ctx context.Context, r entity.Resource) (string, error) {
	var lat, lng *float64
	if r.MeetingMode.RequiresLocation() {
		_, ok := r.Attributes["address"]
		if r.Address == nil && r.Coordinate == nil || !ok {
			return "", fmt.Errorf("store: address not found")
		}
		lat = &r.Coordinate.Latitude
		lng = &r.Coordinate.Longitude
	}

	attrJSON, err := json.Marshal(r.Attributes)
	if err != nil {
		return "", fmt.Errorf("store: marshal resource attributes: %w", err)
	}

	var resourceID string
	err = s.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s
		SET name = $2,
		    org_id = NULLIF($3, '')::uuid,
		    meeting_mode = $4,
		    location = CASE
		        WHEN $5::double precision IS NULL
		            OR $6::double precision IS NULL
		        THEN NULL
		        ELSE ST_SetSRID(
		            ST_MakePoint($5, $6),
		            4326
		        )::geography
		    END,
		    attributes = $7::jsonb,
		    updated_at = now()
		WHERE id = $1
		RETURNING id
	`, tableResource),
		r.ID,
		r.Name,
		r.OrgID,
		int32(r.MeetingMode),
		lng,
		lat,
		attrJSON,
	).Scan(&resourceID)

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
	).Scan(&slot.ID, &slot.ResourceID, &slot.SlotTiming.Start, &slot.SlotTiming.End, &slot.Status)
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

func (s *pgStore) SearchResources(
	ctx context.Context,
	req entity.SearchResourceRequest,
) ([]entity.ResourceSummary, error) {
	attributeFilters := req.Attributes
	if attributeFilters == nil {
		attributeFilters = make(map[string]string)
	}

	// Internal filter used by GetMyResource.
	userID := ""
	if value, ok := attributeFilters["__user_id"]; ok {
		userID = value

		attributeFilters = make(map[string]string, len(attributeFilters)-1)
		for key, value := range req.Attributes {
			if key != "__user_id" {
				attributeFilters[key] = value
			}
		}
	}

	attrJSON, err := json.Marshal(attributeFilters)
	if err != nil {
		return nil, fmt.Errorf("store: marshal attribute filter: %w", err)
	}

	var tenantType string
	if req.TenantType != nil {
		tenantType = req.TenantType.String()
	}

	var orgID string
	if req.OrgID != nil {
		orgID = *req.OrgID
	}

	var name string
	if req.Name != nil {
		name = *req.Name
	}

	var meetingMode int32
	if req.MeetingMode != nil {
		meetingMode = int32(*req.MeetingMode)
	}

	lat := req.Latitude
	lng := req.Longitude
	radiusKM := req.RadiusKM

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			r.id,
			r.name,
			rt.id,
			rt.name,
			rt.is_active,
			r.meeting_mode,
			r.tenant_type,
			r.org_id,
			CASE
				WHEN $9::double precision IS NOT NULL
					AND r.location IS NOT NULL
				THEN ST_Distance(
					r.location,
					ST_SetSRID(
						ST_MakePoint($8, $7),
						4326
					)::geography
				) / 1000.0
				ELSE NULL
			END AS distance_km
		FROM %s r
		JOIN %s rt
			ON rt.id = r.resource_type_id
		WHERE
			-- Tenant type: nil => all tenant types
			($1 = '' OR r.tenant_type = $1)

			-- Organization: nil => all organizations
			AND ($2 = '' OR r.org_id = NULLIF($2, '')::uuid)

			-- Only active resources/resource types
			AND r.is_active = TRUE
			AND rt.is_active = TRUE

			-- Name: nil => all names
			AND ($3 = '' OR r.name ILIKE '%%' || $3 || '%%')

			-- Resource type: empty => all resource types
			AND ($4 = '' OR r.resource_type_id::text = $4)

			-- Meeting mode: 0 => all meeting modes
			AND ($5 = 0 OR r.meeting_mode = $5)

			-- Attributes: empty map => no attribute filter
			AND (
				$6::jsonb = '{}'::jsonb
				OR r.attributes @> $6::jsonb
			)

			-- Internal user filter: empty => all users
			AND (
				$10 = ''
				OR r.user_id = NULLIF($10, '')::uuid
			)

			-- Radius: nil => no location filtering
			AND (
				$9::double precision IS NULL
				OR (
					r.location IS NOT NULL
					AND ST_DWithin(
						r.location,
						ST_SetSRID(
							ST_MakePoint($8, $7),
							4326
						)::geography,
						$9 * 1000
					)
				)
			)

		ORDER BY r.name
		LIMIT 50
	`, tableResource, tableResourceType),
		tenantType,
		orgID,
		name,
		req.ResourceTypeID,
		meetingMode,
		attrJSON,
		lat,
		lng,
		radiusKM,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("store: search resources: %w", err)
	}

	defer rows.Close()

	var summaries []entity.ResourceSummary

	for rows.Next() {
		var summary entity.ResourceSummary
		var tenantType string
		var orgID *string

		if err := rows.Scan(
			&summary.ResourceID,
			&summary.Name,
			&summary.ResourceType.ID,
			&summary.ResourceType.Name,
			&summary.ResourceType.IsActive,
			&summary.MeetingMode,
			&tenantType,
			&orgID,
			&summary.DistanceKM,
		); err != nil {
			return nil, fmt.Errorf("store: scan resource row: %w", err)
		}

		summary.TenantType = entity.ParseTenantType(tenantType)
		summary.OrgID = orgID
		summary.IsActive = true

		summaries = append(summaries, summary)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate search results: %w", err)
	}

	// Search only needs the earliest available slot.
	for i := range summaries {
		slots, err := nextOpenSlots(
			ctx,
			s.pool,
			summaries[i].ResourceID,
			req.WindowStart,
			req.WindowEnd,
			1,
		)
		if err != nil {
			return nil, err
		}

		if len(slots) > 0 {
			summaries[i].NextAvailableSlotTime = slots[0].SlotTiming
		}
	}

	return summaries, nil
}

func nextOpenSlots(
	ctx context.Context,
	pool *pgxpool.Pool,
	resourceID string,
	windowStart *time.Time,
	windowEnd *time.Time,
	limit int,
) ([]entity.Slot, error) {
	rows, err := pool.Query(ctx, fmt.Sprintf(`
		SELECT
			id,
			resource_id,
			start_time,
			end_time,
			status
		FROM %s
		WHERE resource_id = $1
		  AND status = 1
		  AND ($2::timestamptz IS NULL OR start_time >= $2)
		  AND ($3::timestamptz IS NULL OR start_time < $3)
		ORDER BY start_time
		LIMIT $4
	`, tableSlot),
		resourceID,
		windowStart,
		windowEnd,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("store: next open slots: %w", err)
	}

	defer rows.Close()

	var slots []entity.Slot

	for rows.Next() {
		var slot entity.Slot

		if err := rows.Scan(
			&slot.ID,
			&slot.ResourceID,
			&slot.SlotTiming.Start,
			&slot.SlotTiming.End,
			&slot.Status,
		); err != nil {
			return nil, fmt.Errorf("store: scan slot row: %w", err)
		}

		slots = append(slots, slot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate next open slots: %w", err)
	}

	return slots, nil
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
	`, tableSlot, tableResource, tableResourceType), slotID).Scan(&sl.ID, &sl.ResourceID, &sl.SlotTiming.Start, &sl.SlotTiming.End, &sl.Status, &resourceActive, &typeActive)
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

func (s *pgStore) GetResourceById(ctx context.Context, resourceID string) (entity.Resource, error) {
	var resource entity.Resource
	var tenantType string
	var orgID *string
	var latitude, longitude *float64
	var resourceType entity.ResourceType

	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			r.id,
			r.user_id,
			r.tenant_type,
			r.org_id,
			r.resource_type_id,
			r.name,
			r.meeting_mode,
			r.attributes,
			ST_Y(r.location::geometry),
			ST_X(r.location::geometry),
			r.is_active,
			rt.id,
			rt.name,
			rt.is_active
		FROM %s r
		JOIN %s rt ON rt.id = r.resource_type_id
		WHERE r.id = $1
	`, tableResource, tableResourceType), resourceID).Scan(
		&resource.ID,
		&resource.UserID,
		&tenantType,
		&orgID,
		&resource.ResourceType.ID,
		&resource.Name,
		&resource.MeetingMode,
		&resource.Attributes,
		&latitude,
		&longitude,
		&resource.IsActive,
		&resourceType.ID,
		&resourceType.Name,
		&resourceType.IsActive,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Resource{}, ErrResourceNotFound
	}
	if err != nil {
		return entity.Resource{}, fmt.Errorf("store: get resource by id: %w", err)
	}
	resource.TenantType = entity.ParseTenantType(tenantType)
	resource.OrgID = orgID
	resource.ResourceType = resourceType
	if address, ok := resource.Attributes["address"]; ok {
		resource.Address = &address
	}
	if latitude != nil && longitude != nil {
		resource.Coordinate = &entity.Coordinate{
			Latitude:  *latitude,
			Longitude: *longitude,
		}
	}
	return resource, nil
}

func (s *pgStore) GetSlotsByResourceId(ctx context.Context, resourceID string, start time.Time, end time.Time) ([]entity.Slot, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			s.id,
			s.resource_id,
			s.start_time,
			s.end_time,
			s.status
		FROM %s s
		JOIN %s r ON r.id = s.resource_id
		JOIN %s rt ON rt.id = r.resource_type_id
		WHERE s.resource_id = $1
		  AND r.is_active = TRUE
		  AND rt.is_active = TRUE
		  AND ($2::timestamptz IS NULL OR s.start_time >= $2)
		  AND ($3::timestamptz IS NULL OR s.start_time < $3)
		ORDER BY s.start_time
	`, tableSlot, tableResource, tableResourceType),
		resourceID,
		nullIfZero(start),
		nullIfZero(end),
	)
	if err != nil {
		return nil, fmt.Errorf("store: get slots by resource id: %w", err)
	}
	defer rows.Close()
	slots := make([]entity.Slot, 0)
	for rows.Next() {
		var slot entity.Slot
		if err := rows.Scan(
			&slot.ID,
			&slot.ResourceID,
			&slot.SlotTiming.Start,
			&slot.SlotTiming.End,
			&slot.Status,
		); err != nil {
			return nil, fmt.Errorf("store: scan slot: %w", err)
		}
		slots = append(slots, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate slots: %w", err)
	}
	return slots, nil
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
		batch.Queue(q, resourceID, sl.SlotTiming.Start, sl.SlotTiming.End)
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
