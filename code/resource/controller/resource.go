package controller

//go:generate mockgen -source=resource.go -destination=../../../gen/mocks/resource/controller/resource_mock.go -package=mocks

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/client/locationIQ"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/util"
)

const (
	defaultExpansionWeeks = 4
	searchCacheTTL        = 30 * time.Second
	resourceTypeCacheKey  = "resource:types"
)

type Controller interface {
	ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error)
	CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error)
	SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error)
	DeleteResourceType(ctx context.Context, resourceTypeID string) error
	CreateResource(ctx context.Context, r entity.Resource) (resourceID string, err error)
	UpdateResource(ctx context.Context, r entity.Resource) (resourceID string, err error)
	SetResourceStatus(ctx context.Context, resourceID string, isActive bool) error
	DeleteResource(ctx context.Context, resourceID string) error
	SetRecurringAvailability(ctx context.Context, resourceID string, rules []entity.RecurrenceRule) (slotsGenerated int, err error)
	AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error)
	RemoveSlotException(ctx context.Context, slotID, reason string) (entity.SlotStatus, error)
	SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (slotsRemoved int, err error)
	SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error)
	GetSlot(ctx context.Context, slotID string) (entity.Slot, error)
	GetResourceById(ctx context.Context, resourceID string) (entity.ResourceSummary, map[string]string, error)
	GetSlotsByResourceId(ctx context.Context, resourceID string, startUnix *int64, endUnix *int64) ([]entity.RecurrenceRule, []entity.Slot, error)
}

type resourceController struct {
	store            store.Store
	client           client.Client
	locationIQClinet locationIQ.LocationIQClient
	expansionWeeks   int
}

func New(s store.Store, c client.Client, lc locationIQ.LocationIQClient) Controller {
	return &resourceController{
		store:            s,
		client:           c,
		locationIQClinet: lc,
		expansionWeeks:   defaultExpansionWeeks,
	}
}

func (c *resourceController) ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error) {
	if cached, ok, cacheErr := c.client.GetCachedSearch(ctx, resourceTypeCacheKey); cacheErr == nil && ok {
		var resourceTypes []entity.ResourceType
		if json.Unmarshal(cached, &resourceTypes) == nil {
			return resourceTypes, nil
		}
	}

	resourceTypes, err := c.store.ListResourceTypes(ctx)
	if err != nil {
		return nil, err
	}

	_ = c.cacheResourceTypes(ctx, resourceTypes)
	return resourceTypes, nil
}

func (c *resourceController) cacheResourceTypes(ctx context.Context, resourceTypes []entity.ResourceType) error {
	payload, err := json.Marshal(resourceTypes)
	if err != nil {
		return err
	}
	return c.client.SetCachedSearch(ctx, resourceTypeCacheKey, payload, 0)
}

func (c *resourceController) refreshResourceTypesCache(ctx context.Context) {
	resourceTypes, err := c.store.ListResourceTypes(ctx)
	if err != nil {
		return
	}
	_ = c.cacheResourceTypes(ctx, resourceTypes)
}

func (c *resourceController) CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error) {
	rt := entity.ResourceType{Name: name}
	if err := rt.Validate(); err != nil {
		return entity.ResourceType{}, err
	}
	rt, err := c.store.CreateResourceType(ctx, name)
	if err == nil {
		c.refreshResourceTypesCache(ctx)
		_ = c.client.InvalidateGlobalSearchCache(ctx)
	}
	return rt, err
}

func (c *resourceController) SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error) {
	if resourceTypeID == "" {
		return entity.ResourceType{}, entity.ErrInvalidResourceTypeID
	}
	rt, err := c.store.SetResourceTypeStatus(ctx, resourceTypeID, isActive)
	if err == nil {
		c.refreshResourceTypesCache(ctx)
		_ = c.client.InvalidateGlobalSearchCache(ctx)
	}
	return rt, err
}

func (c *resourceController) DeleteResourceType(ctx context.Context, resourceTypeID string) error {
	if resourceTypeID == "" {
		return entity.ErrInvalidResourceTypeID
	}
	err := c.store.DeleteResourceType(ctx, resourceTypeID)
	if err == nil {
		c.refreshResourceTypesCache(ctx)
		_ = c.client.InvalidateGlobalSearchCache(ctx)
	}
	return err
}

func (c *resourceController) CreateResource(ctx context.Context, r entity.Resource) (string, error) {
	if err := util.ValidateResourceCreate(r); err != nil {
		return "", err
	}
	if r.Attributes == nil {
		r.Attributes = make(map[string]string)
	}
	if r.MeetingMode.RequiresLocation() {
		coordinates, err := c.locationIQClinet.GetCoordinates(ctx, *r.Address)
		if err != nil {
			return "", err
		}
		if len(coordinates) == 0 {
			return "", entity.ErrLocationRequired
		}
		r.Attributes["address"] = *r.Address
		r.Coordinate = &coordinates[0]
	}
	slots, err := c.expandRecurrence(r.Recurrence, time.Now())
	if err != nil {
		return "", err
	}
	id, err := c.store.CreateResource(ctx, r, slots)
	if err != nil {
		return "", err
	}
	c.invalidateCacheByOrg(ctx, r.OrgID)
	return id, nil
}

func (c *resourceController) UpdateResource(ctx context.Context, r entity.Resource) (string, error) {
	if err := util.ValidateResourceUpdate(r); err != nil {
		return "", err
	}
	if r.Attributes == nil {
		r.Attributes = make(map[string]string)
	}
	if r.Address != nil {
		coordinates, err := c.locationIQClinet.GetCoordinates(ctx, *r.Address)
		if err != nil {
			return "", err
		}
		if len(coordinates) == 0 {
			return "", entity.ErrLocationRequired
		}
		r.Attributes["address"] = *r.Address
		r.Coordinate = &coordinates[0]
	}
	resourceID, err := c.store.UpdateResource(ctx, r)
	if err != nil {
		return "", err
	}
	c.invalidateCacheByResource(ctx, resourceID)
	return resourceID, nil
}

func (c *resourceController) SetResourceStatus(ctx context.Context, resourceID string, isActive bool) error {
	if resourceID == "" {
		return entity.ErrInvalidResourceID
	}
	orgID, err := c.store.SetResourceStatus(ctx, resourceID, isActive)
	if err != nil {
		return err
	}
	c.invalidateCacheByOrg(ctx, orgID)
	return nil
}

func (c *resourceController) DeleteResource(ctx context.Context, resourceID string) error {
	if resourceID == "" {
		return entity.ErrInvalidResourceID
	}
	orgID, err := c.store.DeleteResource(ctx, resourceID)
	if err != nil {
		return err
	}
	c.invalidateCacheByOrg(ctx, orgID)
	return nil
}

func (c *resourceController) SetRecurringAvailability(ctx context.Context, resourceID string, rules []entity.RecurrenceRule) (int, error) {
	for i, rule := range rules {
		if err := rule.Validate(); err != nil {
			return 0, fmt.Errorf("controller: recurrence[%d]: %w", i, err)
		}
	}
	now := time.Now()
	slots, err := c.expandRecurrence(rules, now)
	if err != nil {
		return 0, err
	}
	n, err := c.store.ReplaceRecurrence(ctx, resourceID, rules, slots, now)
	if err != nil {
		return 0, err
	}
	c.invalidateCacheByResource(ctx, resourceID)
	return n, nil
}

func (c *resourceController) AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error) {
	if se.ResourceID == "" {
		return entity.Slot{}, entity.ErrInvalidResourceID
	}
	if !se.End.After(se.Start) {
		return entity.Slot{}, entity.ErrInvalidTimeRange
	}
	slot, err := c.store.AddSlotException(ctx, se)
	if err != nil {
		return entity.Slot{}, err
	}
	c.invalidateCacheByResource(ctx, se.ResourceID)
	return slot, nil
}

func (c *resourceController) RemoveSlotException(ctx context.Context, slotID, reason string) (entity.SlotStatus, error) {
	status, err := c.store.BlockSlot(ctx, slotID, reason)
	if err != nil {
		return entity.SlotStatusUnspecified, err
	}
	// Best-effort: a lookup failure must never turn a successful mutation
	// into a failed RPC — cache invalidation is optimization only.
	if slot, gerr := c.store.GetSlot(ctx, slotID); gerr == nil {
		c.invalidateCacheByResource(ctx, slot.ResourceID)
	}
	return status, nil
}

func (c *resourceController) SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (int, error) {
	if lp.ResourceID == "" {
		return 0, entity.ErrInvalidResourceID
	}
	if !lp.End.After(lp.Start) {
		return 0, entity.ErrInvalidTimeRange
	}
	n, err := c.store.SetLeavePeriod(ctx, lp)
	if err != nil {
		return 0, err
	}
	c.invalidateCacheByResource(ctx, lp.ResourceID)
	return n, nil
}

func (c *resourceController) SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error) {
	if err := util.ValidateResourceSearchRequest(req); err != nil {
		return nil, err
	}
	cacheKey, _ := c.searchCacheKey(ctx, req)
	if cacheKey != "" {
		if cached, ok, err := c.client.GetCachedSearch(ctx, cacheKey); err == nil && ok {
			var summaries []entity.ResourceSummary
			if err := json.Unmarshal(cached, &summaries); err == nil {
				return summaries, nil
			}
		}
	}
	summaries, err := c.store.SearchResources(ctx, req)
	if err != nil {
		return nil, err
	}
	if cacheKey != "" {
		if payload, err := json.Marshal(summaries); err == nil {
			_ = c.client.SetCachedSearch(ctx, cacheKey, payload, searchCacheTTL)
		}
	}
	return summaries, nil
}

func (c *resourceController) GetResourceById(ctx context.Context, resourceID string) (entity.ResourceSummary, map[string]string, error) {
	if resourceID == "" {
		return entity.ResourceSummary{}, nil, entity.ErrInvalidResourceID
	}

	resource, err := c.store.GetResourceById(ctx, resourceID)
	if err != nil {
		return entity.ResourceSummary{}, nil, err
	}

	summary := entity.ResourceSummary{
		ResourceID:   resource.ID,
		TenantType:   resource.TenantType,
		OrgID:        resource.OrgID,
		Name:         resource.Name,
		ResourceType: resource.ResourceType,
		MeetingMode:  resource.MeetingMode,
		IsActive:     resource.IsActive,
	}

	return summary, resource.Attributes, nil
}

func (c *resourceController) GetSlotsByResourceId(ctx context.Context, resourceID string, startUnix *int64, endUnix *int64) ([]entity.RecurrenceRule, []entity.Slot, error) {
	if resourceID == "" {
		return nil, nil, entity.ErrInvalidResourceID
	}
	if (startUnix == nil) != (endUnix == nil) {
		return nil, nil, entity.ErrInvalidTimeRange
	}
	var start, end time.Time
	if startUnix != nil {
		start = time.Unix(*startUnix, 0)
		end = time.Unix(*endUnix, 0)
		if !end.After(start) {
			return nil, nil, entity.ErrInvalidTimeRange
		}
	}
	recurrence, err := c.store.GetRecurrence(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}
	slots, err := c.store.GetSlotsByResourceId(ctx, resourceID, start, end)
	if err != nil {
		return nil, nil, err
	}
	return recurrence, slots, nil
}

func (c *resourceController) searchCacheKey(ctx context.Context, req entity.SearchResourceRequest) (string, error) {
	orgID := ""
	if req.OrgID != nil {
		orgID = *req.OrgID
	}
	version, err := c.client.SearchCacheVersion(ctx, orgID)
	if err != nil {
		return "", err
	}
	globalVersion, err := c.client.GlobalSearchCacheVersion(ctx)
	if err != nil {
		return "", err
	}
	keyPayload := struct {
		GlobalVersion  int64               `json:"global_version"`
		Version        int64               `json:"version"`
		TenantType     *entity.TenantType  `json:"tenant_type"`
		OrgID          *string             `json:"org_id"`
		Name           *string             `json:"name"`
		ResourceTypeID string              `json:"resource_type_id"`
		MeetingMode    *entity.MeetingMode `json:"meeting_mode"`
		Attributes     map[string]string   `json:"attributes"`
		Latitude       *float64            `json:"latitude"`
		Longitude      *float64            `json:"longitude"`
		RadiusKM       *float64            `json:"radius_km"`
		WindowStart    *time.Time          `json:"window_start"`
		WindowEnd      *time.Time          `json:"window_end"`
	}{
		GlobalVersion:  globalVersion,
		Version:        version,
		TenantType:     req.TenantType,
		OrgID:          req.OrgID,
		Name:           req.Name,
		ResourceTypeID: req.ResourceTypeID,
		MeetingMode:    req.MeetingMode,
		Attributes:     req.Attributes,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		RadiusKM:       req.RadiusKM,
		WindowStart:    req.WindowStart,
		WindowEnd:      req.WindowEnd,
	}

	payload, err := json.Marshal(keyPayload)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(payload)
	scope := orgID
	if scope == "" {
		scope = "__standalone__"
	}
	return fmt.Sprintf("resource:search:%s:v%d:%x", scope, version, h), nil
}

func (c *resourceController) GetSlot(ctx context.Context, slotID string) (entity.Slot, error) {
	return c.store.GetSlot(ctx, slotID)
}

// expandRecurrence walks each calendar day over the expansion window,
// matches it against the rule for that weekday (if any), and materializes
// one entity.Slot per TimeSlot in the resource's own IANA timezone before
// converting to UTC for storage.
func (c *resourceController) expandRecurrence(rules []entity.RecurrenceRule, from time.Time) ([]entity.Slot, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	byWeekday := make(map[time.Weekday]entity.RecurrenceRule, len(rules))
	for _, r := range rules {
		byWeekday[toTimeWeekday(r.Day)] = r
	}

	now := time.Now()
	var slots []entity.Slot
	totalDays := c.expansionWeeks * 7

	for i := 0; i < totalDays; i++ {
		day := from.AddDate(0, 0, i)
		rule, ok := byWeekday[day.Weekday()]
		if !ok {
			continue
		}
		loc, err := time.LoadLocation(rule.Timezone)
		if err != nil {
			return nil, fmt.Errorf("controller: load location %q: %w", rule.Timezone, err)
		}
		for _, ts := range rule.Slots {
			start := time.Date(day.Year(), day.Month(), day.Day(), ts.StartHour, ts.StartMinute, 0, 0, loc)
			end := time.Date(day.Year(), day.Month(), day.Day(), ts.EndHour, ts.EndMinute, 0, 0, loc)
			if end.Before(now) {
				continue // never materialize a slot already in the past
			}
			slots = append(slots, entity.Slot{
				SlotTiming: entity.SlotTiming{
					Start: start.UTC(),
					End:   end.UTC(),
				},
				Status: entity.SlotStatusOpen,
			})
		}
	}
	return slots, nil
}

// toTimeWeekday reconciles entity.DayOfWeek (1=Monday..7=Sunday) with Go's
// time.Weekday (0=Sunday..6=Saturday).
func toTimeWeekday(d entity.DayOfWeek) time.Weekday {
	if d == entity.Sunday {
		return time.Sunday
	}
	return time.Weekday(d)
}

func (c *resourceController) invalidateCacheByOrg(ctx context.Context, orgID *string) {
	if orgID == nil {
		_ = c.client.InvalidateOrgSearchCache(ctx, "__standalone__")
		return
	}
	_ = c.client.InvalidateOrgSearchCache(ctx, *orgID)
}

func (c *resourceController) invalidateCacheByResource(ctx context.Context, resourceID string) {
	orgID, err := c.store.GetResourceOrgID(ctx, resourceID)
	if err != nil {
		return
	}
	c.invalidateCacheByOrg(ctx, orgID)
}
