package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/client"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/middleware"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/util"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Handler struct {
	Identity         client.IdentityClient
	Resource         client.ResourceClient
	Booking          client.BookingClient
	Issuer           *middleware.TokenIssuer
	LocationIQAPIKey string
}

func New(i client.IdentityClient, r client.ResourceClient, b client.BookingClient, issuer *middleware.TokenIssuer, apiKey string) *Handler {
	return &Handler{Identity: i, Resource: r, Booking: b, Issuer: issuer, LocationIQAPIKey: apiKey}
}
func decode(r *http.Request, msg proto.Message) error {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(b, msg)
}
func writeProto(w http.ResponseWriter, msg proto.Message) {
	b, e := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if e != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": e.Error()}, 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
func call(w http.ResponseWriter, msg proto.Message, err error) {
	if err != nil {
		util.WriteGRPCError(w, err)
		return
	}
	writeProto(w, msg)
}
func meta(ctx *http.Request) context.Context {
	p, _ := middleware.PrincipalFromContext(ctx.Context())
	return metadata.AppendToOutgoingContext(ctx.Context(), "x-user-id", p.UserID, "x-user-role", roleName(p.Role), "x-org-id", p.OrgID, "x-tenant-type", tenantTypeName(p.TenantType))
}
func roleName(r identitypb.UserRole) string {
	switch r {
	case identitypb.UserRole_USER_ROLE_CLIENT:
		return "client"
	case identitypb.UserRole_USER_ROLE_RESOURCE:
		return "resource"
	default:
		return "unspecified"
	}
}

func tenantTypeName(t identitypb.TenantType) string {
	switch t {
	case identitypb.TenantType_TENANT_TYPE_INDIVIDUAL:
		return "individual"
	case identitypb.TenantType_TENANT_TYPE_ORG:
		return "org"
	default:
		return "unspecified"
	}
}

func (h *Handler) issueToken(w http.ResponseWriter, user *identitypb.User) {
	token, err := h.Issuer.Issue(user)
	if err != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": err.Error()}, http.StatusInternalServerError)
		return
	}
	util.WriteJSON(w, map[string]interface{}{"user": user, "jwt": token}, http.StatusOK)
}

func (h *Handler) RegisterClient(w http.ResponseWriter, r *http.Request) {
	req := new(identitypb.RegisterClientRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, http.StatusBadRequest)
		return
	}
	resp, e := h.Identity.RegisterClient(r.Context(), req)
	if e != nil {
		call(w, resp, e)
		return
	}
	h.issueToken(w, resp.User)
}

func (h *Handler) RegisterResource(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": err.Error()}, 400)
		return
	}
	accountReq := new(identitypb.RegisterProviderRequest)
	resourceReq := new(resourcepb.CreateResourceRequest)
	opts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := opts.Unmarshal(body, accountReq); err != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": err.Error()}, 400)
		return
	}
	if err := opts.Unmarshal(body, resourceReq); err != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": err.Error()}, 400)
		return
	}
	accountResp, err := h.Identity.RegisterProvider(r.Context(), accountReq)
	if err != nil {
		call(w, accountResp, err)
		return
	}
	if accountResp == nil || accountResp.User == nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "resource account registration returned no user"}, 500)
		return
	}
	resourceReq.UserId = accountResp.User.UserId
	if accountResp.User.TenantType == identitypb.TenantType_TENANT_TYPE_ORG && accountResp.User.OrgId != nil {
		resourceReq.TenantType = resourcepb.TenantType_TENANT_TYPE_ORG
		resourceReq.OrgId = *accountResp.User.OrgId
	} else {
		resourceReq.TenantType = resourcepb.TenantType_TENANT_TYPE_INDIVIDUAL
		resourceReq.OrgId = ""
	}
	resourceResp, err := h.Resource.CreateResource(r.Context(), resourceReq)
	if err != nil {
		_, _ = h.Identity.SetUserStatus(r.Context(), &identitypb.SetUserStatusRequest{UserId: accountResp.User.UserId, IsActive: false})
		call(w, resourceResp, err)
		return
	}
	util.WriteJSON(w, map[string]interface{}{"user": accountResp.User, "resource_id": resourceResp.GetResourceId()}, http.StatusOK)
}

func (h *Handler) LoginResource(w http.ResponseWriter, r *http.Request) {
	req := new(identitypb.LoginRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, http.StatusBadRequest)
		return
	}
	resp, e := h.Identity.Login(r.Context(), req)
	if e != nil {
		call(w, resp, e)
		return
	}
	if resp.User == nil || resp.User.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "resource login is only available to resource accounts")
		return
	}
	h.issueToken(w, resp.User)
}

func (h *Handler) LoginClient(w http.ResponseWriter, r *http.Request) {
	req := new(identitypb.LoginRequest)

	if e := decode(r, req); e != nil {
		util.WriteJSON(
			w,
			map[string]string{
				"code":    "INVALID_ARGUMENT",
				"message": e.Error(),
			},
			http.StatusBadRequest,
		)
		return
	}

	resp, e := h.Identity.Login(r.Context(), req)
	if e != nil {
		call(w, resp, e)
		return
	}

	if resp.User.Role != identitypb.UserRole_USER_ROLE_CLIENT {
		writeForbidden(w, "invalid account type for this login")
		return
	}

	h.issueToken(w, resp.User)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	req := new(identitypb.UpdateProfileRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	p, _ := middleware.PrincipalFromContext(r.Context())
	req.UserId = p.UserID
	resp, e := h.Identity.UpdateProfile(meta(r), req)
	call(w, resp, e)
}
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	p, _ := middleware.PrincipalFromContext(r.Context())
	resp, e := h.Identity.GetUser(meta(r), &identitypb.GetUserRequest{UserId: p.UserID})
	call(w, resp, e)
}

func writeForbidden(w http.ResponseWriter, message string) {
	util.WriteJSON(w, map[string]string{"code": "PERMISSION_DENIED", "message": message}, http.StatusForbidden)
}

func (h *Handler) CreateResource(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.CreateResourceRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can manage resources")
		return
	}
	req.UserId = p.UserID
	if p.TenantType == identitypb.TenantType_TENANT_TYPE_ORG {
		req.TenantType = resourcepb.TenantType_TENANT_TYPE_ORG
		if p.OrgID == "" {
			writeForbidden(w, "organization context is required")
			return
		}
		req.OrgId = p.OrgID
	} else {
		req.TenantType = resourcepb.TenantType_TENANT_TYPE_INDIVIDUAL
		req.OrgId = ""
	}
	resp, e := h.Resource.CreateResource(meta(r), req)
	call(w, resp, e)
}

func (h *Handler) UpdateResourceProfile(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.CreateResourceRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, http.StatusBadRequest)
		return
	}

	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can update their resource profile")
		return
	}

	req.UserId = p.UserID
	if p.TenantType == identitypb.TenantType_TENANT_TYPE_ORG {
		req.TenantType = resourcepb.TenantType_TENANT_TYPE_ORG
		req.OrgId = p.OrgID
	} else {
		req.TenantType = resourcepb.TenantType_TENANT_TYPE_INDIVIDUAL
		req.OrgId = ""
	}

	ctx := metadata.AppendToOutgoingContext(meta(r), "x-resource-operation", "update")
	resp, e := h.Resource.CreateResource(ctx, req)
	call(w, resp, e)
}

func (h *Handler) GetMyResource(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can view their resource profile")
		return
	}

	resp, e := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
		Attributes: map[string]string{"__user_id": p.UserID},
	})
	if e != nil {
		call(w, resp, e)
		return
	}
	if len(resp.GetResources()) == 0 {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "resource profile not found"}, http.StatusNotFound)
		return
	}

	resource := resp.GetResources()[0]
	response := map[string]interface{}{"resource": resource}
	if resource.GetOrgId() != "" {
		if orgResp, orgErr := h.Identity.GetOrganization(meta(r), &identitypb.GetOrganizationRequest{
			OrganizationId: resource.GetOrgId(),
		}); orgErr == nil && orgResp != nil && orgResp.Organization != nil {
			response["organization"] = orgResp.Organization
		}
	}
	util.WriteJSON(w, response, http.StatusOK)
}

func (h *Handler) SearchResources(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.SearchResourcesRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	resp, e := h.Resource.SearchResources(r.Context(), req)
	call(w, resp, e)
}
func (h *Handler) Geocode(w http.ResponseWriter, r *http.Request) {
	address := strings.TrimSpace(r.URL.Query().Get("address"))
	if address == "" {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": "address is required"}, 400)
		return
	}

	if h.LocationIQAPIKey == "" {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "geocoding configuration missing"}, 500)
		return
	}

	u := fmt.Sprintf("https://us1.locationiq.com/v1/search?key=%s&q=%s&format=json&", h.LocationIQAPIKey, url.QueryEscape(address))

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, u, nil)
	if err != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": err.Error()}, 500)
		return
	}

	req.Header.Set("User-Agent", "NexusScheduler/1.0 (resource scheduling application)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "geocoding service unavailable"}, 502)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "address could not be located"}, 404)
		return
	}

	if resp.StatusCode != http.StatusOK {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "geocoding service returned an error"}, 502)
		return
	}

	var places []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&places); err != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "invalid geocoding response"}, 502)
		return
	}

	if len(places) == 0 {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "address could not be located"}, 404)
		return
	}

	lat, err1 := strconv.ParseFloat(places[0].Lat, 64)
	lng, err2 := strconv.ParseFloat(places[0].Lon, 64)
	if err1 != nil || err2 != nil {
		util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "invalid geocoding coordinates"}, 502)
		return
	}

	util.WriteJSON(w, map[string]interface{}{
		"latitude":     lat,
		"longitude":    lng,
		"display_name": places[0].DisplayName,
	}, 200)
}

func (h *Handler) GetSlot(w http.ResponseWriter, r *http.Request) {
	resp, e := h.Resource.GetSlot(r.Context(), &resourcepb.GetSlotRequest{SlotId: pathID(r.URL.Path)})
	call(w, resp, e)
}

func (h *Handler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	req := new(bookingpb.CreateBookingRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	if !h.resourceSlotExists(r, req.GetResourceId(), req.GetStartUnix(), req.GetEndUnix()) {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": "the selected time is not an available slot for this resource"}, http.StatusBadRequest)
		return
	}
	p, _ := middleware.PrincipalFromContext(r.Context())
	req.UserId = p.UserID
	if userResp, userErr := h.Identity.GetUser(meta(r), &identitypb.GetUserRequest{UserId: p.UserID}); userErr == nil && userResp.User != nil && userResp.User.Identifier != nil {
		req.UserEmail = userResp.User.Identifier.GetEmail()
	}
	resp, e := h.Booking.CreateBooking(meta(r), req)
	call(w, resp, e)
}

func (h *Handler) resourceSlotExists(r *http.Request, resourceID string, startUnix, endUnix int64) bool {
	if resourceID == "" || startUnix <= 0 || endUnix <= startUnix {
		return false
	}
	resp, err := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
		Attributes:      map[string]string{"__resource_id": resourceID},
		WindowStartUnix: startUnix,
		WindowEndUnix:   endUnix,
	})
	if err != nil || resp == nil || len(resp.GetResources()) == 0 {
		return false
	}
	for _, slot := range resp.Resources[0].GetNextAvailableSlots() {
		if slot.GetStartUnix() == startUnix && slot.GetEndUnix() == endUnix {
			return true
		}
	}
	return false
}
func (h *Handler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	ref := bookingReferencePath(r.URL.Path)
	if ref == "" {
		writeForbidden(w, "invalid booking reference")
		return
	}

	if !h.canAccessBooking(r, ref) {
		writeForbidden(w, "you do not have access to this booking")
		return
	}

	resp, e := h.Booking.CancelBooking(
		meta(r),
		&bookingpb.CancelBookingRequest{
			ReferenceCode: ref,
		},
	)
	call(w, resp, e)
}
func (h *Handler) RescheduleBooking(w http.ResponseWriter, r *http.Request) {
	req := new(bookingpb.RescheduleBookingRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(
			w,
			map[string]string{
				"code":    "INVALID_ARGUMENT",
				"message": e.Error(),
			},
			http.StatusBadRequest,
		)
		return
	}
	req.ReferenceCode = bookingReferencePath(r.URL.Path)
	if req.ReferenceCode == "" {
		util.WriteJSON(
			w,
			map[string]string{
				"code":    "INVALID_ARGUMENT",
				"message": "invalid booking reference",
			},
			http.StatusBadRequest,
		)
		return
	}
	if !h.canAccessBooking(r, req.ReferenceCode) {
		writeForbidden(w, "you do not have access to this booking")
		return
	}
	if req.StartUnix <= 0 || req.EndUnix <= req.StartUnix {
		util.WriteJSON(
			w,
			map[string]string{
				"code":    "INVALID_ARGUMENT",
				"message": "invalid reschedule time window",
			},
			http.StatusBadRequest,
		)
		return
	}
	current, e := h.Booking.GetBookingStatus(
		meta(r),
		&bookingpb.GetBookingStatusRequest{
			ReferenceCode: req.ReferenceCode,
		},
	)
	if e != nil || current == nil ||
		!h.resourceSlotExists(
			r,
			current.GetResourceId(),
			req.GetStartUnix(),
			req.GetEndUnix(),
		) {
		util.WriteJSON(
			w,
			map[string]string{
				"code":    "INVALID_ARGUMENT",
				"message": "the selected time is not an available slot for this resource",
			},
			http.StatusBadRequest,
		)
		return
	}
	resp, e := h.Booking.RescheduleBooking(meta(r), req)
	call(w, resp, e)
}

func (h *Handler) canAccessBooking(r *http.Request, reference string) bool {
	booking, err := h.Booking.GetBookingStatus(meta(r), &bookingpb.GetBookingStatusRequest{ReferenceCode: reference})
	if err != nil || booking == nil {
		return false
	}
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		return false
	}
	switch p.Role {
	case identitypb.UserRole_USER_ROLE_CLIENT:
		return booking.GetUserId() == p.UserID
	case identitypb.UserRole_USER_ROLE_RESOURCE:
		// Resolve the exact resource owned by this account. The old check only
		// looked at the first resource returned by SearchResources, which could
		// fail when the search/cache result did not line up with the booking.
		lookup, err := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
			Attributes: map[string]string{
				"__user_id":     p.UserID,
				"__resource_id": booking.GetResourceId(),
			},
		})
		return err == nil && len(lookup.GetResources()) == 1 && lookup.Resources[0].GetResourceId() == booking.GetResourceId()
	default:
		return false
	}
}
func bookingReferencePath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "bookings" {
			return parts[i+1]
		}
	}
	return ""
}
func (h *Handler) ListUpcomingBookings(w http.ResponseWriter, r *http.Request) {
	p, _ := middleware.PrincipalFromContext(r.Context())
	resp, e := h.Booking.ListUpcomingBookings(meta(r), &bookingpb.ListUserBookingsRequest{UserId: p.UserID})
	h.writeBookingList(w, r, resp, e)
}

func (h *Handler) ListPastBookings(w http.ResponseWriter, r *http.Request) {
	p, _ := middleware.PrincipalFromContext(r.Context())
	resp, e := h.Booking.ListPastBookings(meta(r), &bookingpb.ListUserBookingsRequest{UserId: p.UserID})
	h.writeBookingList(w, r, resp, e)
}

func (h *Handler) ListResourceUpcomingBookings(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can view resource bookings")
		return
	}
	lookup := &resourcepb.SearchResourcesRequest{Attributes: map[string]string{"__user_id": p.UserID}}
	resources, e := h.Resource.SearchResources(meta(r), lookup)
	if e != nil {
		call(w, resources, e)
		return
	}
	if len(resources.GetResources()) == 0 {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "resource profile not found"}, 404)
		return
	}
	ctx := metadata.AppendToOutgoingContext(meta(r), "x-list-scope", "resource")
	resp, e := h.Booking.ListUpcomingBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: resources.Resources[0].ResourceId})
	h.writeBookingList(w, r, resp, e)
}

func (h *Handler) ListResourcePastBookings(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can view resource events")
		return
	}

	lookup := &resourcepb.SearchResourcesRequest{
		Attributes: map[string]string{"__user_id": p.UserID},
	}
	resources, e := h.Resource.SearchResources(meta(r), lookup)
	if e != nil {
		call(w, resources, e)
		return
	}
	if len(resources.GetResources()) == 0 {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "resource profile not found"}, http.StatusNotFound)
		return
	}

	ctx := metadata.AppendToOutgoingContext(meta(r), "x-list-scope", "resource")
	resp, e := h.Booking.ListPastBookings(ctx, &bookingpb.ListUserBookingsRequest{
		UserId: resources.Resources[0].ResourceId,
	})
	h.writeBookingList(w, r, resp, e)
}

func (h *Handler) GetBooking(w http.ResponseWriter, r *http.Request) {
	ref := pathID(r.URL.Path)
	if !h.canAccessBooking(r, ref) {
		writeForbidden(w, "you do not have access to this booking")
		return
	}
	resp, e := h.Booking.GetBookingStatus(meta(r), &bookingpb.GetBookingStatusRequest{ReferenceCode: ref})
	h.writeBookingList(w, r, &bookingpb.ListUserBookingsResponse{Bookings: []*bookingpb.GetBookingStatusResponse{resp}}, e)
}
func (h *Handler) writeBookingList(w http.ResponseWriter, r *http.Request, resp *bookingpb.ListUserBookingsResponse, err error) {
	if err != nil {
		call(w, resp, err)
		return
	}
	if resp == nil {
		util.WriteJSON(w, map[string]interface{}{"bookings": []interface{}{}}, http.StatusOK)
		return
	}

	p, _ := middleware.PrincipalFromContext(r.Context())
	// A reschedule keeps the same reference code but creates a new row. The
	// list RPC therefore may contain the old RESCHEDULED row and the new
	// CONFIRMED row separately. Build a tiny lineage index from the past list
	// so the UI can render one logical event without changing the DB model.
	rescheduledByRef := map[string]*bookingpb.GetBookingStatusResponse{}
	if len(resp.GetBookings()) > 0 {
		var lineage *bookingpb.ListUserBookingsResponse
		var lineageErr error
		if p.Role == identitypb.UserRole_USER_ROLE_CLIENT {
			lineage, lineageErr = h.Booking.ListPastBookings(meta(r), &bookingpb.ListUserBookingsRequest{UserId: p.UserID})
		} else if p.Role == identitypb.UserRole_USER_ROLE_RESOURCE {
			resourceID := resp.Bookings[0].GetResourceId()
			ctx := metadata.AppendToOutgoingContext(meta(r), "x-list-scope", "resource")
			lineage, lineageErr = h.Booking.ListPastBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: resourceID})
		}
		if lineageErr == nil && lineage != nil {
			for _, item := range lineage.GetBookings() {
				if item == nil || item.GetStatus().String() != "BOOKING_STATUS_RESCHEDULED" {
					continue
				}
				current := rescheduledByRef[item.GetReferenceCode()]
				if current == nil || item.GetStartUnix() > current.GetStartUnix() {
					rescheduledByRef[item.GetReferenceCode()] = item
				}
			}
		}
	}

	views := make([]map[string]interface{}, 0, len(resp.GetBookings()))
	seen := make(map[string]bool)
	nowUnix := time.Now().Unix()
	for _, booking := range resp.GetBookings() {
		if booking == nil || seen[booking.GetReferenceCode()] {
			continue
		}

		current := booking
		statusText := booking.GetStatus().String()
		var previousStart, previousEnd int64

		if booking.GetStatus().String() == "BOOKING_STATUS_RESCHEDULED" {
			latest, latestErr := h.Booking.GetBookingStatus(meta(r), &bookingpb.GetBookingStatusRequest{ReferenceCode: booking.GetReferenceCode()})
			if latestErr == nil && latest != nil {
				// If the new booking is still upcoming, the logical event belongs
				// in Upcoming, not twice across both tabs.
				if latest.GetStartUnix() >= nowUnix && booking.GetStartUnix() < nowUnix {
					seen[booking.GetReferenceCode()] = true
					continue
				}
				current = latest
				previousStart = booking.GetStartUnix()
				previousEnd = booking.GetEndUnix()
			}
			statusText = "BOOKING_STATUS_RESCHEDULED"
		} else if prior := rescheduledByRef[booking.GetReferenceCode()]; prior != nil {
			previousStart = prior.GetStartUnix()
			previousEnd = prior.GetEndUnix()
			statusText = "BOOKING_STATUS_RESCHEDULED"
		}

		view := map[string]interface{}{
			"reference_code": current.GetReferenceCode(),
			"user_id":        current.GetUserId(),
			"resource_id":    current.GetResourceId(),
			"start_unix":     current.GetStartUnix(),
			"end_unix":       current.GetEndUnix(),
			"title":          current.GetTitle(),
			"subtitle":       current.GetSubtitle(),
			"status":         statusText,
		}
		if previousStart > 0 {
			view["previous_start_unix"] = previousStart
			view["previous_end_unix"] = previousEnd
		}

		resourceResp, resourceErr := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
			Attributes: map[string]string{"__resource_id": current.GetResourceId()},
		})
		if resourceErr == nil && resourceResp != nil && len(resourceResp.GetResources()) > 0 {
			resource := resourceResp.GetResources()[0]
			view["resource_name"] = resource.GetName()
			view["meeting_mode"] = resource.GetMeetingMode().String()
			if address := resource.GetAttributes()["address"]; address != "" {
				view["address"] = address
			}
			if p.Role == identitypb.UserRole_USER_ROLE_CLIENT {
				view["display_title"] = "Meeting with " + resource.GetName()
			}
		}

		if p.Role == identitypb.UserRole_USER_ROLE_RESOURCE {
			if userResp, userErr := h.Identity.GetUser(meta(r), &identitypb.GetUserRequest{UserId: current.GetUserId()}); userErr == nil && userResp != nil && userResp.GetUser() != nil {
				view["client_name"] = userResp.GetUser().GetName()
				view["display_title"] = "Meeting with " + userResp.GetUser().GetName()
			}
		}

		seen[booking.GetReferenceCode()] = true
		views = append(views, view)
	}
	util.WriteJSON(w, map[string]interface{}{"bookings": views}, http.StatusOK)
}

func pathID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func resourceIDPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "resources" {
			return parts[i+1]
		}
	}
	return ""
}

func (h *Handler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	req := new(identitypb.CreateOrganizationRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	resp, e := h.Identity.CreateOrganization(r.Context(), req)
	call(w, resp, e)
}

func (h *Handler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	resp, e := h.Identity.ListOrganizations(r.Context(), &identitypb.ListOrganizationsRequest{})
	call(w, resp, e)
}

func (h *Handler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	resp, e := h.Identity.GetOrganization(r.Context(), &identitypb.GetOrganizationRequest{OrganizationId: pathID(r.URL.Path)})
	call(w, resp, e)
}

func (h *Handler) ListResourceTypes(w http.ResponseWriter, r *http.Request) {
	resp, e := h.Resource.ListResourceTypes(r.Context(), &resourcepb.ListResourceTypesRequest{})
	call(w, resp, e)
}

func (h *Handler) CreateResourceType(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.CreateResourceTypeRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	resp, e := h.Resource.CreateResourceType(r.Context(), req)
	call(w, resp, e)
}

func (h *Handler) ownsResource(r *http.Request, resourceID string) bool {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		return false
	}
	lookup, err := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{Attributes: map[string]string{"__user_id": p.UserID}})
	return err == nil && len(lookup.GetResources()) > 0 && lookup.Resources[0].ResourceId == resourceID
}

func (h *Handler) SetResourceStatus(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.SetResourceStatusRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.ResourceId = pathID(r.URL.Path)
	p, _ := middleware.PrincipalFromContext(r.Context())
	// A resource may only mutate its own resource profile.
	lookup, e := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{Attributes: map[string]string{"__user_id": p.UserID}})
	if e != nil {
		call(w, lookup, e)
		return
	}
	if len(lookup.GetResources()) == 0 || lookup.Resources[0].ResourceId != req.ResourceId {
		writeForbidden(w, "resource access denied")
		return
	}
	resp, e := h.Resource.SetResourceStatus(meta(r), req)
	if e != nil {
		call(w, resp, e)
		return
	}
	if lookup.Resources[0].OrgId != nil {
		orgID := *lookup.Resources[0].OrgId
		activeLookup, _ := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{OrgId: orgID})
		if activeLookup != nil {
			_, _ = h.Identity.SetOrganizationStatus(meta(r), &identitypb.SetOrganizationStatusRequest{OrganizationId: orgID, IsActive: len(activeLookup.Resources) > 0})
		}
	}
	call(w, resp, e)
}

func (h *Handler) DeleteResource(w http.ResponseWriter, r *http.Request) {
	resp, e := h.Resource.DeleteResource(meta(r), &resourcepb.DeleteResourceRequest{ResourceId: pathID(r.URL.Path)})
	call(w, resp, e)
}

// GetResourceAvailability returns the actual booking-facing calendar for a
// resource. Open slots come from ResourceService; current confirmed/waitlisted
// bookings are overlaid from BookingService. This keeps the existing resource
// slot model intact while giving the UI the four states it needs: available,
// booked, past, and no-slot.
func (h *Handler) GetResourceAvailability(w http.ResponseWriter, r *http.Request) {
	resourceID := resourceIDPath(r.URL.Path)
	if resourceID == "" {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": "resource id is required"}, http.StatusBadRequest)
		return
	}
	startUnix, _ := strconv.ParseInt(r.URL.Query().Get("start_unix"), 10, 64)
	endUnix, _ := strconv.ParseInt(r.URL.Query().Get("end_unix"), 10, 64)
	if startUnix <= 0 || endUnix <= startUnix {
		now := time.Now()
		startUnix = now.Unix()
		endUnix = now.AddDate(0, 0, 7).Unix()
	}

	// SearchResources has a preview limit of 21 slots. Fetch one day at a time
	// so a full calendar week cannot silently lose later slots.
	type calendarSlot struct {
		StartUnix int64  `json:"start_unix"`
		EndUnix   int64  `json:"end_unix"`
		Status    string `json:"status"`
	}
	open := make(map[string]calendarSlot)
	cursor := time.Unix(startUnix, 0)
	end := time.Unix(endUnix, 0)
	for cursor.Before(end) {
		next := cursor.Add(24 * time.Hour)
		if next.After(end) {
			next = end
		}
		resp, err := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
			Attributes:      map[string]string{"__resource_id": resourceID},
			WindowStartUnix: cursor.Unix(),
			WindowEndUnix:   next.Unix(),
		})
		if err != nil {
			call(w, resp, err)
			return
		}
		for _, resource := range resp.GetResources() {
			for _, slot := range resource.GetNextAvailableSlots() {
				key := fmt.Sprintf("%d-%d", slot.GetStartUnix(), slot.GetEndUnix())
				open[key] = calendarSlot{StartUnix: slot.GetStartUnix(), EndUnix: slot.GetEndUnix(), Status: "available"}
			}
		}
		cursor = next
	}

	// BookingService already has a resource-scoped list path. We use both
	// current and past rows because a past booked slot still needs to be shown
	// as Past, not Booked. Cancelled/rescheduled rows are ignored below.
	ctx := metadata.AppendToOutgoingContext(meta(r), "x-list-scope", "resource")
	upcoming, upErr := h.Booking.ListUpcomingBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: resourceID})
	past, pastErr := h.Booking.ListPastBookings(ctx, &bookingpb.ListUserBookingsRequest{UserId: resourceID})
	if upErr != nil || pastErr != nil {
		if upErr != nil {
			util.WriteGRPCError(w, upErr)
		} else {
			util.WriteGRPCError(w, pastErr)
		}
		return
	}

	for _, list := range []*bookingpb.ListUserBookingsResponse{upcoming, past} {
		for _, booking := range list.GetBookings() {
			if booking == nil || booking.GetResourceId() != resourceID {
				continue
			}
			status := booking.GetStatus().String()
			if status != "BOOKING_STATUS_CONFIRMED" && status != "BOOKING_STATUS_WAITLISTED" {
				continue
			}
			if booking.GetStartUnix() < startUnix || booking.GetStartUnix() >= endUnix {
				continue
			}
			// A booking is considered booked only when it exactly corresponds to
			// a generated resource slot. Invalid legacy bookings therefore cannot
			// invent a red slot on the resource calendar.
			key := fmt.Sprintf("%d-%d", booking.GetStartUnix(), booking.GetEndUnix())
			if _, exists := open[key]; exists {
				open[key] = calendarSlot{StartUnix: booking.GetStartUnix(), EndUnix: booking.GetEndUnix(), Status: "booked"}
			}
		}
	}

	slots := make([]calendarSlot, 0, len(open))
	for _, slot := range open {
		slots = append(slots, slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].StartUnix < slots[j].StartUnix })
	util.WriteJSON(w, map[string]interface{}{"slots": slots}, http.StatusOK)
}

func (h *Handler) GetMyAvailability(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can view availability")
		return
	}
	resp, err := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
		Attributes: map[string]string{"__user_id": p.UserID, "__include_recurrence": "1"},
	})
	if err != nil {
		call(w, resp, err)
		return
	}
	if len(resp.GetResources()) == 0 {
		util.WriteJSON(w, map[string]string{"code": "NOT_FOUND", "message": "resource profile not found"}, http.StatusNotFound)
		return
	}
	raw := resp.Resources[0].GetAttributes()["__recurrence_json"]
	var recurrence interface{} = []interface{}{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &recurrence); err != nil {
			util.WriteJSON(w, map[string]string{"code": "INTERNAL", "message": "invalid stored recurrence"}, http.StatusInternalServerError)
			return
		}
	}
	util.WriteJSON(w, map[string]interface{}{"recurrence": recurrence}, http.StatusOK)
}

func (h *Handler) SetRecurringAvailability(w http.ResponseWriter, r *http.Request) {
	resourceID := pathID(r.URL.Path)
	if !h.ownsResource(r, resourceID) {
		writeForbidden(w, "resource access denied")
		return
	}
	req := new(resourcepb.SetRecurringAvailabilityRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.ResourceId = resourceID
	resp, e := h.Resource.SetRecurringAvailability(meta(r), req)
	call(w, resp, e)
}

// SetMyRecurringAvailability resolves the resource from the authenticated
// user instead of trusting a resource id supplied by the browser. This keeps
// the ownership check tied to the JWT identity and avoids stale/mismatched ids
// in the profile UI.
func (h *Handler) SetMyRecurringAvailability(w http.ResponseWriter, r *http.Request) {
	p, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || p.Role != identitypb.UserRole_USER_ROLE_RESOURCE {
		writeForbidden(w, "only resource accounts can manage availability")
		return
	}

	lookup, e := h.Resource.SearchResources(meta(r), &resourcepb.SearchResourcesRequest{
		Attributes: map[string]string{"__user_id": p.UserID},
	})
	if e != nil {
		call(w, lookup, e)
		return
	}
	if len(lookup.GetResources()) == 0 {
		writeForbidden(w, "resource profile not found")
		return
	}

	req := new(resourcepb.SetRecurringAvailabilityRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.ResourceId = lookup.Resources[0].ResourceId
	resp, e := h.Resource.SetRecurringAvailability(meta(r), req)
	call(w, resp, e)
}
func (h *Handler) AddSlotException(w http.ResponseWriter, r *http.Request) {
	if !h.ownsResource(r, pathID(r.URL.Path)) {
		writeForbidden(w, "resource access denied")
		return
	}
	req := new(resourcepb.AddSlotExceptionRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.ResourceId = pathID(r.URL.Path)
	resp, e := h.Resource.AddSlotException(meta(r), req)
	call(w, resp, e)
}
func (h *Handler) RemoveSlotException(w http.ResponseWriter, r *http.Request) {
	req := new(resourcepb.RemoveSlotExceptionRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.SlotId = pathID(r.URL.Path)
	resp, e := h.Resource.RemoveSlotException(meta(r), req)
	call(w, resp, e)
}
func (h *Handler) SetLeavePeriod(w http.ResponseWriter, r *http.Request) {
	if !h.ownsResource(r, pathID(r.URL.Path)) {
		writeForbidden(w, "resource access denied")
		return
	}
	req := new(resourcepb.SetLeavePeriodRequest)
	if e := decode(r, req); e != nil {
		util.WriteJSON(w, map[string]string{"code": "INVALID_ARGUMENT", "message": e.Error()}, 400)
		return
	}
	req.ResourceId = pathID(r.URL.Path)
	resp, e := h.Resource.SetLeavePeriod(meta(r), req)
	call(w, resp, e)
}
