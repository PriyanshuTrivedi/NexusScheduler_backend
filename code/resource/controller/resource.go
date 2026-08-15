package controller

//go:generate mockgen -source=resource.go -destination=../../../gen/mocks/resource/controller/resource_mock.go -package=mocks

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/resource/store"
)

const (
	defaultExpansionWeeks = 4
	searchCacheTTL        = 30 * time.Second
)

type Controller interface {
	ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error)
	CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error)
	SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error)
	DeleteResourceType(ctx context.Context, resourceTypeID string) error
	CreateResource(ctx context.Context, r entity.Resource) (resourceID string, err error)
	SetResourceStatus(ctx context.Context, resourceID string, isActive bool) error
	DeleteResource(ctx context.Context, resourceID string) error
	SetRecurringAvailability(ctx context.Context, resourceID string, rules []entity.RecurrenceRule) (slotsGenerated int, err error)
	AddSlotException(ctx context.Context, se entity.SlotException) (entity.Slot, error)
	RemoveSlotException(ctx context.Context, slotID, reason string) (entity.SlotStatus, error)
	SetLeavePeriod(ctx context.Context, lp entity.LeavePeriod) (slotsRemoved int, err error)
	SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error)
	GetSlot(ctx context.Context, slotID string) (entity.Slot, error)
}

type resourceController struct {
	store          store.Store
	client         client.Client
	expansionWeeks int
}

func New(s store.Store, c client.Client) Controller {
	return &resourceController{store: s, client: c, expansionWeeks: defaultExpansionWeeks}
}

func (c *resourceController) ListResourceTypes(ctx context.Context) ([]entity.ResourceType, error) {
	return c.store.ListResourceTypes(ctx)
}

func (c *resourceController) CreateResourceType(ctx context.Context, name string) (entity.ResourceType, error) {
	rt := entity.ResourceType{Name: name}
	if err := rt.Validate(); err != nil {
		return entity.ResourceType{}, err
	}
	return c.store.CreateResourceType(ctx, name)
}

func (c *resourceController) SetResourceTypeStatus(ctx context.Context, resourceTypeID string, isActive bool) (entity.ResourceType, error) {
	if resourceTypeID == "" {
		return entity.ResourceType{}, entity.ErrInvalidResourceTypeID
	}
	rt, err := c.store.SetResourceTypeStatus(ctx, resourceTypeID, isActive)
	if err == nil {
		_ = c.client.InvalidateGlobalSearchCache(ctx)
	}
	return rt, err
}

func (c *resourceController) DeleteResourceType(ctx context.Context, resourceTypeID string) error {
	if resourceTypeID == "" {
		return entity.ErrInvalidResourceTypeID
	}
	return c.store.DeleteResourceType(ctx, resourceTypeID)
}

func (c *resourceController) CreateResource(ctx context.Context, r entity.Resource) (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	slots, err := c.expandRecurrence(r.Recurrence, time.Now())
	if err != nil {
		return "", err
	}
	id, err := c.store.CreateResource(ctx, r, slots)
	if err != nil {
		return "", err
	}
	c.invalidateCacheByOrgValue(ctx, r.OrgID)
	return id, nil
}

// UpdateResource is deliberately kept out of the public Controller interface.
// The gRPC CreateResource RPC is reused internally for the authenticated
// resource-profile update path, while creation remains the public operation.
func (c *resourceController) UpdateResource(ctx context.Context, userID, name string, mode entity.MeetingMode, lat, lng *float64, attributes map[string]string) (string, error) {
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

	updater, ok := c.store.(interface {
		UpdateResource(context.Context, string, string, entity.MeetingMode, *float64, *float64, map[string]string) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("resource: update operation is unavailable")
	}

	resourceID, err := updater.UpdateResource(ctx, userID, name, mode, lat, lng, attributes)
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

// SearchResources is the one read path fronted by a cache — the highest-QPS
// RPC on this service, and unlike GetSlot it tolerates a few seconds of
// staleness (Booking always re-validates via GetSlot before reserving, so a
// stale search result can never itself cause a double-booking).
func (c *resourceController) SearchResources(ctx context.Context, req entity.SearchResourceRequest) ([]entity.ResourceSummary, error) {
	includeRecurrence := req.Attributes["__include_recurrence"] == "1"
	var cacheKey string
	if !includeRecurrence {
		cacheKey, _ = c.searchCacheKey(ctx, req)
		if cacheKey != "" {
			if cached, ok, err := c.client.GetCachedSearch(ctx, cacheKey); err == nil && ok {
				var summaries []entity.ResourceSummary
				if json.Unmarshal(cached, &summaries) == nil {
					return summaries, nil
				}
			}
		}
	}

	summaries, err := c.store.SearchResources(ctx, req)
	if err != nil {
		return nil, err
	}

	if includeRecurrence {
		if reader, ok := c.store.(interface {
			GetRecurrence(context.Context, string) ([]entity.RecurrenceRule, error)
		}); ok {
			for i := range summaries {
				rules, rerr := reader.GetRecurrence(ctx, summaries[i].ResourceID)
				if rerr != nil {
					return nil, rerr
				}
				payload, _ := json.Marshal(rules)
				if summaries[i].Attributes == nil {
					summaries[i].Attributes = map[string]string{}
				}
				summaries[i].Attributes["__recurrence_json"] = string(payload)
			}
		}
	}

	if cacheKey != "" {
		if payload, err := json.Marshal(summaries); err == nil {
			_ = c.client.SetCachedSearch(ctx, cacheKey, payload, searchCacheTTL)
		}
	}
	return summaries, nil
}

func (c *resourceController) searchCacheKey(ctx context.Context, req entity.SearchResourceRequest) (string, error) {
	version, err := c.client.SearchCacheVersion(ctx, req.OrgID)
	if err != nil {
		return "", err
	}
	globalVersion, err := c.client.GlobalSearchCacheVersion(ctx)
	if err != nil {
		return "", err
	}
	s := (fmt.Sprintf("%d|%d|%d|%s|%s|%s|%d|%v|%.6f|%.6f|%.3f|%d|%d", globalVersion, version, req.TenantType, req.OrgID, req.Name, req.ResourceTypeID, req.MeetingMode, req.Attributes, req.Latitude, req.Longitude, req.RadiusKM,
		req.WindowStart.Unix(), req.WindowEnd.Unix()))
	h := sha256.Sum256([]byte(s))
	scope := req.OrgID
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
				Start:  start.UTC(),
				End:    end.UTC(),
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

func (c *resourceController) invalidateCacheByOrgValue(ctx context.Context, orgID string) {
	if orgID == "" {
		c.invalidateCacheByOrg(ctx, nil)
		return
	}
	c.invalidateCacheByOrg(ctx, &orgID)
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
