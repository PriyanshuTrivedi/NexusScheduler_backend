package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/entity"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
)

func executeChatToolRequest(
	h *Handler,
	tool string,
	body string,
	userID string,
	role string,
) *httptest.ResponseRecorder {
	reqBody := `{"tool":"` + tool + `","body":` + body + `}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/chat/tools/execute",
		strings.NewReader(reqBody),
	)
	req.Header.Set("Content-Type", "application/json")

	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	if role != "" {
		req.Header.Set("X-User-Role", role)
	}

	rr := httptest.NewRecorder()
	h.ExecuteChatTool(rr, req)

	return rr
}

func TestExecuteChatTool_InvalidRequestBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/chat/tools/execute",
		strings.NewReader(`invalid-json`),
	)
	rr := httptest.NewRecorder()

	h.ExecuteChatTool(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_MissingIdentity(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolListUpcomingBookings,
		`{}`,
		"",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_InvalidRole(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolListUpcomingBookings,
		`{}`,
		"user-1",
		"admin",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_UnsupportedTool(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		"unsupported_tool",
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestChatToolIdentityFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		userID     string
		role       string
		wantOK     bool
		wantUserID string
		wantRole   string
	}{
		{
			name:       "client",
			userID:     "user-1",
			role:       "client",
			wantOK:     true,
			wantUserID: "user-1",
			wantRole:   identitypb.UserRole_USER_ROLE_CLIENT.String(),
		},
		{
			name:       "resource",
			userID:     "resource-user-1",
			role:       "resource",
			wantOK:     true,
			wantUserID: "resource-user-1",
			wantRole:   identitypb.UserRole_USER_ROLE_RESOURCE.String(),
		},
		{
			name:   "missing user id",
			userID: "",
			role:   "client",
		},
		{
			name:   "missing role",
			userID: "user-1",
			role:   "",
		},
		{
			name:   "invalid role",
			userID: "user-1",
			role:   "admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("X-User-ID", tt.userID)
			req.Header.Set("X-User-Role", tt.role)

			got, ok := chatToolIdentityFromRequest(req)

			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantUserID, got.UserID)
				assert.Equal(t, tt.wantRole, got.Role)
			}
		})
	}
}

func TestChatToolContext(t *testing.T) {
	identity := entity.ChatToolIdentity{
		UserID: "user-1",
		Role:   identitypb.UserRole_USER_ROLE_CLIENT.String(),
	}

	md := chatToolContext(identity)

	assert.Equal(t, []string{"user-1"}, md.Get("x-user-id"))
	assert.Equal(t, []string{"client"}, md.Get("x-user-role"))
}

func TestExecuteChatTool_GetBooking_ClientOwnsBooking(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *bookingpb.GetBookingStatusRequest, _ ...interface{}) (*bookingpb.GetBookingStatusResponse, error) {
			assert.Equal(t, "REF-123", req.GetReferenceCode())

			md, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)
			assert.Equal(t, []string{"user-1"}, md.Get("x-user-id"))
			assert.Equal(t, []string{"client"}, md.Get("x-user-role"))

			return &bookingpb.GetBookingStatusResponse{
				UserId:     "user-1",
				ResourceId: "resource-1",
			}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetBooking_ClientDoesNotOwnBooking(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "another-user",
			ResourceId: "resource-1",
		}, nil)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestExecuteChatTool_GetBooking_ResourceOwnsBooking(t *testing.T) {
	h, _, resource, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "client-1",
			ResourceId: "resource-1",
		}, nil)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.SearchResourcesRequest, _ ...interface{}) (*resourcepb.SearchResourcesResponse, error) {
			assert.Equal(t, "resource-user-1", req.GetAttributes()["__user_id"])
			assert.Equal(t, "resource-1", req.GetAttributes()["__resource_id"])

			return &resourcepb.SearchResourcesResponse{
				Resources: []*resourcepb.ResourceSummary{
					{ResourceId: "resource-1"},
				},
			}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"resource-user-1",
		"resource",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetBooking_ResourceDoesNotOwnBooking(t *testing.T) {
	h, _, resource, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "client-1",
			ResourceId: "resource-1",
		}, nil)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(&resourcepb.SearchResourcesResponse{
			Resources: []*resourcepb.ResourceSummary{
				{ResourceId: "another-resource"},
			},
		}, nil)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"resource-user-1",
		"resource",
	)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestExecuteChatTool_GetBooking_ResourceLookupFails(t *testing.T) {
	h, _, resource, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(&bookingpb.GetBookingStatusResponse{
			UserId:     "client-1",
			ResourceId: "resource-1",
		}, nil)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "lookup failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"resource-user-1",
		"resource",
	)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestExecuteChatTool_GetBooking_MissingReferenceCode(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_GetBooking_InvalidBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"unknownField":true}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_GetBooking_ServiceError(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "booking service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetBooking_NotFound(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(nil, nil)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetBooking,
		`{"referenceCode":"REF-123"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestExecuteChatTool_ListUpcomingBookings(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		ListUpcomingBookings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *bookingpb.ListUserBookingsRequest, _ ...interface{}) (*bookingpb.ListUserBookingsResponse, error) {
			assert.Equal(t, "user-1", req.GetUserId())

			md, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)
			assert.Equal(t, []string{"user-1"}, md.Get("x-user-id"))
			assert.Equal(t, []string{"client"}, md.Get("x-user-role"))

			return &bookingpb.ListUserBookingsResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolListUpcomingBookings,
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_ListPastBookings(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		ListPastBookings(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *bookingpb.ListUserBookingsRequest, _ ...interface{}) (*bookingpb.ListUserBookingsResponse, error) {
			assert.Equal(t, "user-1", req.GetUserId())
			return &bookingpb.ListUserBookingsResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolListPastBookings,
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_ListBookings_RejectsArguments(t *testing.T) {
	tests := []struct {
		name string
		tool string
	}{
		{
			name: "upcoming",
			tool: entity.ToolListUpcomingBookings,
		},
		{
			name: "past",
			tool: entity.ToolListPastBookings,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, _, _ := newHandlerTest(t)

			rr := executeChatToolRequest(
				h,
				tt.tool,
				`{"user_id":"another-user"}`,
				"user-1",
				"client",
			)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestExecuteChatTool_ListUpcomingBookings_ServiceError(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	booking.EXPECT().
		ListUpcomingBookings(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "booking service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolListUpcomingBookings,
		`{}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_SearchResources(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.SearchResourcesRequest, _ ...interface{}) (*resourcepb.SearchResourcesResponse, error) {
			assert.Equal(t, "doctor", req.GetAttributes()["type"])

			return &resourcepb.SearchResourcesResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolSearchResources,
		`{"attributes":{"type":"doctor"}}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_SearchResources_InvalidBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolSearchResources,
		`{"unknownField":true}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_SearchResources_ServiceError(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		SearchResources(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "resource service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolSearchResources,
		`{}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetResource(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetResourceById(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.GetResourceByIdRequest, _ ...interface{}) (*resourcepb.GetResourceByIdResponse, error) {
			assert.Equal(t, "resource-1", req.GetResourceId())
			return &resourcepb.GetResourceByIdResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResource,
		`{"resourceId":"resource-1"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetResource_MissingResourceID(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResource,
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_GetResource_InvalidBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResource,
		`{"unknownField":true}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_GetResource_ServiceError(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetResourceById(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "resource service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResource,
		`{"resourceId":"resource-1"}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetResourceAvailability_DefaultWindow(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.GetSlotsByResourceIdRequest, _ ...interface{}) (*resourcepb.GetSlotsByResourceIdResponse, error) {
			require.NotNil(t, req.StartUnix)
			require.NotNil(t, req.EndUnix)
			assert.Greater(t, *req.EndUnix, *req.StartUnix)
			assert.Equal(t, "resource-1", req.GetResourceId())

			return &resourcepb.GetSlotsByResourceIdResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResourceAvailability,
		`{"resourceId":"resource-1"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetResourceAvailability_ExplicitWindow(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.GetSlotsByResourceIdRequest, _ ...interface{}) (*resourcepb.GetSlotsByResourceIdResponse, error) {
			require.NotNil(t, req.StartUnix)
			require.NotNil(t, req.EndUnix)
			assert.Equal(t, int64(100), *req.StartUnix)
			assert.Equal(t, int64(200), *req.EndUnix)

			return &resourcepb.GetSlotsByResourceIdResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResourceAvailability,
		`{"resourceId":"resource-1","startUnix":"100","endUnix":"200"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_GetResourceAvailability_MissingResourceID(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResourceAvailability,
		`{"startUnix":"100","endUnix":"200"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_GetResourceAvailability_InvalidWindows(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "only start provided",
			body: `{"resourceId":"resource-1","startUnix":"100"}`,
		},
		{
			name: "only end provided",
			body: `{"resourceId":"resource-1","endUnix":"200"}`,
		},
		{
			name: "zero start",
			body: `{"resourceId":"resource-1","startUnix":"0","endUnix":"200"}`,
		},
		{
			name: "negative start",
			body: `{"resourceId":"resource-1","startUnix":"-1","endUnix":"200"}`,
		},
		{
			name: "end before start",
			body: `{"resourceId":"resource-1","startUnix":"200","endUnix":"100"}`,
		},
		{
			name: "same start and end",
			body: `{"resourceId":"resource-1","startUnix":"100","endUnix":"100"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, _, _ := newHandlerTest(t)

			rr := executeChatToolRequest(
				h,
				entity.ToolGetResourceAvailability,
				tt.body,
				"user-1",
				"client",
			)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestExecuteChatTool_GetResourceAvailability_ServiceError(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "availability service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolGetResourceAvailability,
		`{"resourceId":"resource-1"}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_ListResourceTypes(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		ListResourceTypes(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, req *resourcepb.ListResourceTypesRequest, _ ...interface{}) (*resourcepb.ListResourceTypesResponse, error) {
			require.NotNil(t, req)
			return &resourcepb.ListResourceTypesResponse{}, nil
		})

	rr := executeChatToolRequest(
		h,
		entity.ToolListResourceTypes,
		`{}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestExecuteChatTool_ListResourceTypes_RejectsArguments(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	rr := executeChatToolRequest(
		h,
		entity.ToolListResourceTypes,
		`{"user_id":"user-1"}`,
		"user-1",
		"client",
	)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestExecuteChatTool_ListResourceTypes_ServiceError(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		ListResourceTypes(gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Internal, "resource service failed"))

	rr := executeChatToolRequest(
		h,
		entity.ToolListResourceTypes,
		`{}`,
		"user-1",
		"client",
	)

	assert.NotEqual(t, http.StatusOK, rr.Code)
}
