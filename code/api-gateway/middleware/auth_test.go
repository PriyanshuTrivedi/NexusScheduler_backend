package middleware

import (
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func tokenForTest(t *testing.T) string {
	m := NewVerifier("secret", "issuer")
	_ = m
	claims := jwt.MapClaims{"sub": "u-1", "role": "resource", "org_id": "o-1", "iss": "issuer", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix()}
	tok, e := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	assert.NoError(t, e)
	return tok
}
func TestVerifierAndAuthMiddleware(t *testing.T) {
	v := NewVerifier("secret", "issuer")
	p, e := v.Verify(tokenForTest(t))
	assert.NoError(t, e)
	assert.Equal(t, "u-1", p.UserID)
	assert.Equal(t, identitypb.UserRole_USER_ROLE_RESOURCE, p.Role)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, "o-1", p.OrgID)
		w.WriteHeader(204)
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tokenForTest(t))
	Auth(v, next).ServeHTTP(rr, req)
	assert.Equal(t, 204, rr.Code)
}
func TestAuthRejectsMissingToken(t *testing.T) {
	v := NewVerifier("secret", "issuer")
	rr := httptest.NewRecorder()
	Auth(v, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Fatal("should not reach handler") })).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, 401, rr.Code)
}

func TestTokenIssuerProducesGatewayJWT(t *testing.T) {
	issuer := NewTokenIssuer("secret", "issuer", time.Hour)
	orgID := "o-1"
	token, err := issuer.Issue(&identitypb.User{
		UserId: "u-1",
		OrgId:  &orgID,
		Role:   identitypb.UserRole_USER_ROLE_RESOURCE,
	})
	assert.NoError(t, err)
	p, err := NewVerifier("secret", "issuer").Verify(token)
	assert.NoError(t, err)
	assert.Equal(t, "u-1", p.UserID)
	assert.Equal(t, "o-1", p.OrgID)
	assert.Equal(t, identitypb.UserRole_USER_ROLE_RESOURCE, p.Role)
}
