package handler

import (
	"context"
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
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	clientmocks "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/api_gateway/client"
)

func newAuthTestHandler(t *testing.T) (*Handler, *clientmocks.MockIdentityClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	im := clientmocks.NewMockIdentityClient(ctrl)
	return New(im, clientmocks.NewMockResourceClient(ctrl), clientmocks.NewMockBookingClient(ctrl), middleware.NewTokenIssuer("secret", "issuer", time.Hour), "dummy-key"), im
}

func loginRequest(path string) *http.Request {
	return httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"identifier":{"email":"user@example.com"},"password":"password"}`))
}

func TestLoginClientSuccess(t *testing.T) {
	h, identity := newAuthTestHandler(t)
	identity.EXPECT().Login(gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *identitypb.LoginRequest, ...grpc.CallOption) (*identitypb.LoginResponse, error) {
		return &identitypb.LoginResponse{User: &identitypb.User{UserId: "client-1", Role: identitypb.UserRole_USER_ROLE_CLIENT}}, nil
	})
	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jwt")
}

func TestLoginClientRejectsResource(t *testing.T) {
	h, identity := newAuthTestHandler(t)
	identity.EXPECT().Login(gomock.Any(), gomock.Any()).Return(&identitypb.LoginResponse{User: &identitypb.User{UserId: "r1", Role: identitypb.UserRole_USER_ROLE_RESOURCE}}, nil)
	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestLoginResourceAllowsResource(t *testing.T) {
	h, identity := newAuthTestHandler(t)
	identity.EXPECT().Login(gomock.Any(), gomock.Any()).Return(&identitypb.LoginResponse{User: &identitypb.User{UserId: "r1", Role: identitypb.UserRole_USER_ROLE_RESOURCE}}, nil)
	rr := httptest.NewRecorder()
	h.LoginResource(rr, loginRequest("/api/v1/auth/resource/login"))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "jwt")
}

func TestLoginResourceRejectsClient(t *testing.T) {
	h, identity := newAuthTestHandler(t)
	identity.EXPECT().Login(gomock.Any(), gomock.Any()).Return(&identitypb.LoginResponse{User: &identitypb.User{UserId: "c1", Role: identitypb.UserRole_USER_ROLE_CLIENT}}, nil)
	rr := httptest.NewRecorder()
	h.LoginResource(rr, loginRequest("/api/v1/auth/resource/login"))
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestLoginClientInvalidJSON(t *testing.T) {
	h, _ := newAuthTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/client/login", strings.NewReader(`{"identifier":`))
	h.LoginClient(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "INVALID_ARGUMENT")
}

func TestLoginResourceInvalidJSON(t *testing.T) {
	h, _ := newAuthTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resource/login", strings.NewReader(`{"identifier":`))
	h.LoginResource(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestLoginClientIdentityServiceError(t *testing.T) {
	h, identity := newAuthTestHandler(t)
	identity.EXPECT().Login(gomock.Any(), gomock.Any()).Return(nil, status.Error(codes.Unauthenticated, "invalid credentials"))
	rr := httptest.NewRecorder()
	h.LoginClient(rr, loginRequest("/api/v1/auth/client/login"))
	assert.NotEqual(t, http.StatusOK, rr.Code)
}
