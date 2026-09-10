package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/middleware"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/api_gateway/client"
)

func newHandlerTest(t *testing.T) (*Handler, *clientmocks.MockIdentityClient, *clientmocks.MockResourceClient, *clientmocks.MockBookingClient) {
	t.Helper()

	ctrl := gomock.NewController(t)
	identity := clientmocks.NewMockIdentityClient(ctrl)
	resource := clientmocks.NewMockResourceClient(ctrl)
	booking := clientmocks.NewMockBookingClient(ctrl)

	h := New(
		identity,
		resource,
		booking,
		middleware.NewTokenIssuer("secret", "issuer", time.Hour),
	)

	return h, identity, resource, booking
}

func loginRequest(path string) *http.Request {
	return httptest.NewRequest(
		http.MethodPost,
		path,
		strings.NewReader(`{"identifier":{"email":"user@example.com"},"password":"password"}`),
	)
}

func authenticatedRequest(t *testing.T, method, target, body string, principal middleware.Principal) *http.Request {
	t.Helper()

	req := httptest.NewRequest(
		method,
		target,
		strings.NewReader(body),
	)

	return req.WithContext(
		middleware.WithPrincipalContext(req.Context(), principal),
	)
}

func clientPrincipal() middleware.Principal {
	return middleware.Principal{
		UserID:     "u1",
		Role:       identitypb.UserRole_USER_ROLE_CLIENT,
		TenantType: identitypb.TenantType_TENANT_TYPE_INDIVIDUAL,
	}
}

func resourcePrincipal() middleware.Principal {
	return middleware.Principal{
		UserID:     "resource-user",
		Role:       identitypb.UserRole_USER_ROLE_RESOURCE,
		TenantType: identitypb.TenantType_TENANT_TYPE_INDIVIDUAL,
	}
}

func TestLoginClientSuccess(t *testing.T) {
	h, identity, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&identitypb.LoginResponse{
			User: &identitypb.User{
				UserId: "client-1",
				Role:   identitypb.UserRole_USER_ROLE_CLIENT,
			},
		}, nil)

	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jwt")
}

func TestLoginClientRejectsResource(t *testing.T) {
	h, identity, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&identitypb.LoginResponse{
			User: &identitypb.User{
				UserId: "r1",
				Role:   identitypb.UserRole_USER_ROLE_RESOURCE,
			},
		}, nil)

	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestLoginResourceAllowsResource(t *testing.T) {
	h, identity, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&identitypb.LoginResponse{
			User: &identitypb.User{
				UserId: "resource-1",
				Role:   identitypb.UserRole_USER_ROLE_RESOURCE,
			},
		}, nil)

	rr := httptest.NewRecorder()
	h.LoginResource(rr, loginRequest("/api/v1/auth/resource/login"))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jwt")
}

func TestLoginResourceRejectsClient(t *testing.T) {
	h, identity, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&identitypb.LoginResponse{
			User: &identitypb.User{
				UserId: "client-1",
				Role:   identitypb.UserRole_USER_ROLE_CLIENT,
			},
		}, nil)

	rr := httptest.NewRecorder()
	h.LoginResource(rr, loginRequest("/api/v1/auth/resource/login"))

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestLoginClientIdentityServiceError(t *testing.T) {
	h, identity, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Unauthenticated, "invalid credentials"))

	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestLoginClientInvalidBody(t *testing.T) {
	h, _, _, _ := newHandlerTest(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/client/login",
		strings.NewReader(`{"invalid":`),
	)

	rr := httptest.NewRecorder()
	h.LoginClient(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLoginResourceInvalidBody(t *testing.T) {
	h, _, _, _ := newHandlerTest(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/resource/login",
		strings.NewReader(`{"invalid":`),
	)

	rr := httptest.NewRecorder()
	h.LoginResource(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRoleNameAndTenantTypeName(t *testing.T) {
	assert.Equal(t, "client", roleName(identitypb.UserRole_USER_ROLE_CLIENT))
	assert.Equal(t, "resource", roleName(identitypb.UserRole_USER_ROLE_RESOURCE))
	assert.Equal(t, "unspecified", roleName(identitypb.UserRole_USER_ROLE_UNSPECIFIED))

	assert.Equal(t, "individual", tenantTypeName(identitypb.TenantType_TENANT_TYPE_INDIVIDUAL))
	assert.Equal(t, "org", tenantTypeName(identitypb.TenantType_TENANT_TYPE_ORG))
	assert.Equal(t, "unspecified", tenantTypeName(identitypb.TenantType_TENANT_TYPE_UNSPECIFIED))
}

func TestBookingReferencePath(t *testing.T) {
	assert.Equal(t, "NXS-1", bookingReferencePath("/api/v1/bookings/NXS-1"))
	assert.Equal(t, "NXS-1", bookingReferencePath("/bookings/NXS-1/anything"))
	assert.Empty(t, bookingReferencePath("/api/v1/users/u1"))
	assert.Empty(t, bookingReferencePath("/bookings/"))
}

func TestAuthenticatedRequest(t *testing.T) {
	principal := middleware.Principal{
		UserID:     "u1",
		OrgID:      "org1",
		Role:       identitypb.UserRole_USER_ROLE_CLIENT,
		TenantType: identitypb.TenantType_TENANT_TYPE_ORG,
	}

	req := authenticatedRequest(
		t,
		http.MethodGet,
		"/test",
		"",
		principal,
	)

	got, ok := middleware.PrincipalFromContext(req.Context())

	assert.True(t, ok)
	assert.Equal(t, principal, got)
}

func TestResourceSlotExists(t *testing.T) {
	h, _, resource, _ := newHandlerTest(t)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(&resourcepb.SearchResourcesResponse{
			Resources: []*resourcepb.ResourceSummary{
				{
					ResourceId: "r1",
					NextAvailableSlots: []*resourcepb.Slot{
						{
							StartUnix: 100,
							EndUnix:   200,
						},
					},
				},
			},
		}, nil)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.True(t, h.resourceSlotExists(r, "r1", 100, 200))
}

func TestResourceSlotExists_NotFound(t *testing.T) {
	h, _, resource, _ := newHandlerTest(t)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(&resourcepb.SearchResourcesResponse{
			Resources: []*resourcepb.ResourceSummary{
				{
					ResourceId: "r1",
					NextAvailableSlots: []*resourcepb.Slot{
						{
							StartUnix: 100,
							EndUnix:   200,
						},
					},
				},
			},
		}, nil)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "r1", 100, 300))
}

func TestResourceSlotExists_InvalidInput(t *testing.T) {
	h, _, _, _ := newHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "", 100, 200))
	assert.False(t, h.resourceSlotExists(r, "r1", 200, 100))
	assert.False(t, h.resourceSlotExists(r, "r1", 0, 100))
}

func TestResourceSlotExists_ServiceError(t *testing.T) {
	h, _, resource, _ := newHandlerTest(t)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "r1", 100, 200))
}

func TestCanAccessBooking_Unauthenticated(t *testing.T) {
	h, _, _, booking := newHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	assert.False(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_Client(t *testing.T) {
	h, _, _, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/",
		"",
		clientPrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "u1",
			ResourceId: "r1",
		}, nil)

	assert.True(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_ClientWrongUser(t *testing.T) {
	h, _, _, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/",
		"",
		clientPrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "u2",
			ResourceId: "r1",
		}, nil)

	assert.False(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_Resource(t *testing.T) {
	h, _, resource, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/",
		"",
		resourcePrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "u1",
			ResourceId: "r1",
		}, nil)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(&resourcepb.SearchResourcesResponse{
			Resources: []*resourcepb.ResourceSummary{
				{
					ResourceId: "r1",
				},
			},
		}, nil)

	assert.True(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_ResourceWrongResource(t *testing.T) {
	h, _, resource, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/",
		"",
		resourcePrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "u1",
			ResourceId: "r2",
		}, nil)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(&resourcepb.SearchResourcesResponse{
			Resources: []*resourcepb.ResourceSummary{
				{
					ResourceId: "r1",
				},
			},
		}, nil)

	assert.False(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_BookingServiceError(t *testing.T) {
	h, _, _, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/",
		"",
		clientPrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	assert.False(t, h.canAccessBooking(r, "NXS-1"))
}

func TestRescheduleBooking_RejectsInvalidTime(t *testing.T) {
	h, _, _, booking := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/bookings/NXS-1/reschedule",
		`{"start_unix":100,"end_unix":50}`,
		clientPrincipal(),
	)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "u1",
			ResourceId: "r1",
		}, nil)

	rr := httptest.NewRecorder()
	h.RescheduleBooking(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRescheduleBooking_RejectsInvalidReference(t *testing.T) {
	h, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/bookings/",
		`{"start_unix":1900007200,"end_unix":1900010800}`,
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.RescheduleBooking(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
