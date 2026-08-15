package router

import (
	"net/http"
	"time"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/handler"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/middleware"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
)

type Config struct {
	RateLimit  int
	RateWindow time.Duration
}

func New(h *handler.Handler, auth *middleware.Verifier, limiter middleware.RateLimiter, cfg Config) http.Handler {
	mux := http.NewServeMux()
	public := func(n http.Handler) http.Handler {
		return middleware.RateLimit(limiter, cfg.RateLimit, cfg.RateWindow, n)
	}
	authn := func(n http.Handler) http.Handler {
		return middleware.RateLimit(limiter, cfg.RateLimit, cfg.RateWindow, middleware.Auth(auth, n))
	}
	resource := func(n http.Handler) http.Handler {
		return authn(middleware.RequireRole(identitypb.UserRole_USER_ROLE_RESOURCE, n))
	}

	mux.Handle("POST /api/v1/auth/client/register", public(http.HandlerFunc(h.RegisterClient)))
	mux.Handle("POST /api/v1/auth/client/login", public(http.HandlerFunc(h.LoginClient)))
	mux.Handle("POST /api/v1/auth/resource/register", public(http.HandlerFunc(h.RegisterResource)))
	mux.Handle("POST /api/v1/auth/resource/login", public(http.HandlerFunc(h.LoginResource)))

	mux.Handle("PATCH /api/v1/users/me", authn(http.HandlerFunc(h.UpdateProfile)))
	mux.Handle("GET /api/v1/users/me", authn(http.HandlerFunc(h.GetUser)))

	// Organizations and resource types are selectable during resource registration.
	mux.Handle("POST /api/v1/organizations", public(http.HandlerFunc(h.CreateOrganization)))
	mux.Handle("GET /api/v1/organizations", public(http.HandlerFunc(h.ListOrganizations)))
	mux.Handle("GET /api/v1/organizations/{organization_id}", public(http.HandlerFunc(h.GetOrganization)))
	mux.Handle("GET /api/v1/resource-types", public(http.HandlerFunc(h.ListResourceTypes)))
	mux.Handle("POST /api/v1/resource-types", public(http.HandlerFunc(h.CreateResourceType)))

	mux.Handle("POST /api/v1/resources", resource(http.HandlerFunc(h.CreateResource)))
	mux.Handle("PUT /api/v1/resources/me", resource(http.HandlerFunc(h.UpdateResourceProfile)))
	mux.Handle("GET /api/v1/resources/me", resource(http.HandlerFunc(h.GetMyResource)))
	mux.Handle("GET /api/v1/resources/me/upcoming", resource(http.HandlerFunc(h.ListResourceUpcomingBookings)))
	mux.Handle("GET /api/v1/resources/me/availability", resource(http.HandlerFunc(h.GetMyAvailability)))
	mux.Handle("GET /api/v1/resources/me/past", resource(http.HandlerFunc(h.ListResourcePastBookings)))
	mux.Handle("POST /api/v1/resources/search", public(http.HandlerFunc(h.SearchResources)))
	mux.Handle("GET /api/v1/resources/{resource_id}/availability", public(http.HandlerFunc(h.GetResourceAvailability)))
	mux.Handle("GET /api/v1/slots/{slot_id}", public(http.HandlerFunc(h.GetSlot)))
	mux.Handle("PUT /api/v1/resources/me/availability", resource(http.HandlerFunc(h.SetMyRecurringAvailability)))
	mux.Handle("PUT /api/v1/resources/{resource_id}/availability", resource(http.HandlerFunc(h.SetRecurringAvailability)))
	mux.Handle("POST /api/v1/resources/{resource_id}/slot-exceptions", resource(http.HandlerFunc(h.AddSlotException)))
	mux.Handle("DELETE /api/v1/slots/{slot_id}/exception", resource(http.HandlerFunc(h.RemoveSlotException)))
	mux.Handle("POST /api/v1/resources/{resource_id}/leave", resource(http.HandlerFunc(h.SetLeavePeriod)))
	mux.Handle("PATCH /api/v1/resources/{resource_id}/status", resource(http.HandlerFunc(h.SetResourceStatus)))

	mux.Handle("POST /api/v1/bookings", authn(http.HandlerFunc(h.CreateBooking)))
	mux.Handle("POST /api/v1/bookings/{reference}/cancel", authn(http.HandlerFunc(h.CancelBooking)))
	mux.Handle("POST /api/v1/bookings/{reference}/reschedule", authn(http.HandlerFunc(h.RescheduleBooking)))
	mux.Handle("GET /api/v1/bookings/me/upcoming", authn(http.HandlerFunc(h.ListUpcomingBookings)))
	mux.Handle("GET /api/v1/bookings/me/past", authn(http.HandlerFunc(h.ListPastBookings)))
	mux.Handle("GET /api/v1/bookings/{reference}", authn(http.HandlerFunc(h.GetBooking)))

	mux.Handle("GET /api/v1/geocode", public(http.HandlerFunc(h.Geocode)))
	return mux
}
