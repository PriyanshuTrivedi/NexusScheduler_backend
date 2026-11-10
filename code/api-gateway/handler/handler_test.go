package handler

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/api-gateway/middleware"
	bookingpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/booking"
	chatbotpb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/chatbot"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	resourcepb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/resource"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/api_gateway/client"
)

func newHandlerTest(t *testing.T) (*Handler, *clientmocks.MockIdentityClient, *clientmocks.MockResourceClient, *clientmocks.MockBookingClient, *clientmocks.MockChatbotClient) {
	t.Helper()

	ctrl := gomock.NewController(t)
	identity := clientmocks.NewMockIdentityClient(ctrl)
	resource := clientmocks.NewMockResourceClient(ctrl)
	booking := clientmocks.NewMockBookingClient(ctrl)
	chatbot := clientmocks.NewMockChatbotClient(ctrl)

	h := New(
		identity,
		resource,
		booking,
		chatbot,
		middleware.NewTokenIssuer("secret", "issuer", time.Hour),
	)

	return h, identity, resource, booking, chatbot
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
	h, identity, _, _, _ := newHandlerTest(t)

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
	h, identity, _, _, _ := newHandlerTest(t)

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
	h, identity, _, _, _ := newHandlerTest(t)

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
	h, identity, _, _, _ := newHandlerTest(t)

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
	h, identity, _, _, _ := newHandlerTest(t)

	identity.EXPECT().
		Login(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.Unauthenticated, "invalid credentials"))

	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))

	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestLoginClientInvalidBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

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
	h, _, _, _, _ := newHandlerTest(t)

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
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), &resourcepb.GetSlotsByResourceIdRequest{
			ResourceId: "r1",
			StartUnix:  ptrInt64(100),
			EndUnix:    ptrInt64(200),
		}).
		Return(&resourcepb.GetSlotsByResourceIdResponse{
			Slots: []*resourcepb.Slot{
				{
					SlotId:     "slot-1",
					ResourceId: "r1",
					SlotTiming: &resourcepb.SlotTiming{
						StartUnix: 100,
						EndUnix:   200,
					},
					Status: resourcepb.SlotStatus_SLOT_STATUS_OPEN,
				},
			},
		}, nil)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.True(t, h.resourceSlotExists(r, "r1", 100, 200))
}

func TestResourceSlotExists_NotFound(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), &resourcepb.GetSlotsByResourceIdRequest{
			ResourceId: "r1",
			StartUnix:  ptrInt64(100),
			EndUnix:    ptrInt64(300),
		}).
		Return(&resourcepb.GetSlotsByResourceIdResponse{
			Slots: []*resourcepb.Slot{
				{
					SlotId:     "slot-1",
					ResourceId: "r1",
					SlotTiming: &resourcepb.SlotTiming{
						StartUnix: 100,
						EndUnix:   200,
					},
					Status: resourcepb.SlotStatus_SLOT_STATUS_OPEN,
				},
			},
		}, nil)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "r1", 100, 300))
}

func TestResourceSlotExists_InvalidInput(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "", 100, 200))
	assert.False(t, h.resourceSlotExists(r, "r1", 200, 100))
	assert.False(t, h.resourceSlotExists(r, "r1", 0, 100))
}

func TestResourceSlotExists_ServiceError(t *testing.T) {
	h, _, resource, _, _ := newHandlerTest(t)

	resource.EXPECT().
		GetSlotsByResourceId(gomock.Any(), &resourcepb.GetSlotsByResourceIdRequest{
			ResourceId: "r1",
			StartUnix:  ptrInt64(100),
			EndUnix:    ptrInt64(200),
		}).
		Return(nil, assert.AnError)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	assert.False(t, h.resourceSlotExists(r, "r1", 100, 200))
}

func ptrInt64(v int64) *int64 {
	return &v
}

func TestCanAccessBooking_Unauthenticated(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/", nil)

	booking.EXPECT().
		GetBookingStatus(gomock.Any(), gomock.Any()).
		Return(nil, assert.AnError)

	assert.False(t, h.canAccessBooking(r, "NXS-1"))
}

func TestCanAccessBooking_Client(t *testing.T) {
	h, _, _, booking, _ := newHandlerTest(t)

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
	h, _, _, booking, _ := newHandlerTest(t)

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
	h, _, resource, booking, _ := newHandlerTest(t)

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
	h, _, resource, booking, _ := newHandlerTest(t)

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
	h, _, _, booking, _ := newHandlerTest(t)

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
	h, _, _, booking, _ := newHandlerTest(t)

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
	h, _, _, _, _ := newHandlerTest(t)

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

func TestSendChatMessage(t *testing.T) {
	h, _, _, _, chatbot := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/chat/messages",
		`{"message":"Hello","user_id":"attacker-user"}`,
		clientPrincipal(),
	)

	chatbot.EXPECT().
		SendMessage(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, req *chatbotpb.SendMessageRequest, _ ...grpc.CallOption) (*chatbotpb.SendMessageResponse, error) {
			assert.Equal(t, "u1", req.UserId)
			assert.Equal(t, "Hello", req.Message)
			return &chatbotpb.SendMessageResponse{}, nil
		})

	rr := httptest.NewRecorder()
	h.SendChatMessage(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestSendChatMessage_InvalidBody(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/chat/messages",
		`{"message":`,
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.SendChatMessage(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCloseChatConversation(t *testing.T) {
	h, _, _, _, chatbot := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/chat/conversations/close",
		"",
		clientPrincipal(),
	)

	chatbot.EXPECT().
		CloseConversation(gomock.Any(), &chatbotpb.CloseConversationRequest{
			UserId: "u1",
		}).
		Return(&chatbotpb.CloseConversationResponse{}, nil)

	rr := httptest.NewRecorder()
	h.CloseChatConversation(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListChatConversations(t *testing.T) {
	h, _, _, _, chatbot := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations?limit=10&offset=5",
		"",
		clientPrincipal(),
	)

	chatbot.EXPECT().
		ListConversations(gomock.Any(), &chatbotpb.ListConversationsRequest{
			UserId: "u1",
			Limit:  10,
			Offset: 5,
		}).
		Return(&chatbotpb.ListConversationsResponse{}, nil)

	rr := httptest.NewRecorder()
	h.ListChatConversations(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListChatConversations_InvalidLimit(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations?limit=invalid",
		"",
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.ListChatConversations(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListChatConversations_NegativeLimit(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations?limit=-1",
		"",
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.ListChatConversations(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestListChatConversations_InvalidOffset(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations?offset=-1",
		"",
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.ListChatConversations(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetChatMessages(t *testing.T) {
	h, _, _, _, chatbot := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations/conversation-1/messages",
		"",
		clientPrincipal(),
	)
	r.SetPathValue("conversation_id", "conversation-1")

	chatbot.EXPECT().
		GetMessages(gomock.Any(), &chatbotpb.GetMessagesRequest{
			UserId:         "u1",
			ConversationId: "conversation-1",
		}).
		Return(&chatbotpb.GetMessagesResponse{}, nil)

	rr := httptest.NewRecorder()
	h.GetChatMessages(rr, r)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetChatMessages_MissingConversationID(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	r := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/chat/conversations//messages",
		"",
		clientPrincipal(),
	)

	rr := httptest.NewRecorder()
	h.GetChatMessages(rr, r)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func documentUploadRequest(t *testing.T, content []byte, principal middleware.Principal) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	file, err := writer.CreateFormFile("file", "resume.pdf")
	assert.NoError(t, err)

	_, err = file.Write(content)
	assert.NoError(t, err)
	assert.NoError(t, writer.Close())

	req := authenticatedRequest(
		t,
		http.MethodPost,
		"/api/v1/documents",
		body.String(),
		principal,
	)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

func TestUploadDocument(t *testing.T) {
	h, identity, _, _, _ := newHandlerTest(t)

	content := []byte("%PDF-1.4\nsample pdf content")
	req := documentUploadRequest(t, content, resourcePrincipal())
	req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")

	identity.EXPECT().
		UploadFile(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, request *identitypb.UploadFileRequest, _ ...grpc.CallOption) (*identitypb.UploadFileResponse, error) {
			assert.Equal(t, "resource-user", request.UserId)
			assert.Equal(t, "resume.pdf", request.FileName)
			assert.Equal(t, identitypb.MimeType_MIME_TYPE_PDF, request.MimeType)
			assert.Equal(t, identitypb.ObjectType_OBJECT_TYPE_RESOURCE_DOCUMENT, request.ObjectType)
			assert.Equal(t, identitypb.ObjectVisibility_OBJECT_VISIBILITY_PRIVATE, request.Visibility)
			assert.Equal(t, content, request.Content)
			assert.NotNil(t, request.IdempotencyKey)
			assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", *request.IdempotencyKey)

			return &identitypb.UploadFileResponse{
				Object: &identitypb.ObjectMetadata{
					ObjectId:         "object-1",
					OriginalFileName: "resume.pdf",
				},
			}, nil
		})

	rr := httptest.NewRecorder()
	h.UploadDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "object-1")
}

func TestUploadDocument_InvalidFileType(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := documentUploadRequest(
		t,
		[]byte("this is not a PDF"),
		resourcePrincipal(),
	)

	rr := httptest.NewRecorder()
	h.UploadDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUploadDocument_EmptyFile(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := documentUploadRequest(t, []byte{}, resourcePrincipal())

	rr := httptest.NewRecorder()
	h.UploadDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUploadDocument_Unauthenticated(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/documents",
		nil,
	)

	rr := httptest.NewRecorder()
	h.UploadDocument(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestGetDocument(t *testing.T) {
	h, identity, _, _, _ := newHandlerTest(t)

	content := []byte("%PDF-1.4\nsample pdf content")

	req := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/documents/object-1",
		"",
		resourcePrincipal(),
	)
	req.SetPathValue("object_id", "object-1")

	identity.EXPECT().
		GetFile(gomock.Any(), &identitypb.GetFileRequest{
			UserId:   "resource-user",
			ObjectId: "object-1",
		}, gomock.Any()).
		Return(&identitypb.GetFileResponse{
			Object: &identitypb.ObjectMetadata{
				ObjectId:         "object-1",
				OriginalFileName: "resume.pdf",
			},
			Content: content,
		}, nil)

	rr := httptest.NewRecorder()
	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "inline")
	assert.Equal(t, string(content), rr.Body.String())
}

func TestGetDocument_MissingObjectID(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/documents/",
		"",
		resourcePrincipal(),
	)

	rr := httptest.NewRecorder()
	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetDocument_NotFound(t *testing.T) {
	h, identity, _, _, _ := newHandlerTest(t)

	req := authenticatedRequest(
		t,
		http.MethodGet,
		"/api/v1/documents/object-1",
		"",
		resourcePrincipal(),
	)
	req.SetPathValue("object_id", "object-1")

	identity.EXPECT().
		GetFile(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, status.Error(codes.NotFound, "object not found"))

	rr := httptest.NewRecorder()
	h.GetDocument(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestDeleteDocument(t *testing.T) {
	h, identity, _, _, _ := newHandlerTest(t)

	req := authenticatedRequest(
		t,
		http.MethodDelete,
		"/api/v1/documents/object-1",
		"",
		resourcePrincipal(),
	)
	req.SetPathValue("object_id", "object-1")

	identity.EXPECT().
		DeleteFile(gomock.Any(), &identitypb.DeleteFileRequest{
			UserId:   "resource-user",
			ObjectId: "object-1",
		}, gomock.Any()).
		Return(&identitypb.DeleteFileResponse{
			Deleted: true,
		}, nil)

	rr := httptest.NewRecorder()
	h.DeleteDocument(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"deleted":true`)
}

func TestDeleteDocument_MissingObjectID(t *testing.T) {
	h, _, _, _, _ := newHandlerTest(t)

	req := authenticatedRequest(
		t,
		http.MethodDelete,
		"/api/v1/documents/",
		"",
		resourcePrincipal(),
	)

	rr := httptest.NewRecorder()
	h.DeleteDocument(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
