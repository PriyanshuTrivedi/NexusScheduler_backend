package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	identity "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
	"github.com/golang-jwt/jwt/v5"
)

type Principal struct {
	UserID, OrgID string
	Role          identity.UserRole
	TenantType    identity.TenantType
}
type contextKey string

const principalKey contextKey = "principal"

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}

// Verifier validates JWTs at the public edge. Identity is not called for
// every request; the gateway owns JWT issuance and validation.
type Verifier struct {
	secret []byte
	issuer string
}

type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenIssuer(secret, issuer string, ttl time.Duration) *TokenIssuer {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

func (i *TokenIssuer) Issue(user *identity.User) (string, error) {
	if user == nil || user.UserId == "" {
		return "", errors.New("user is required")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":         user.UserId,
		"role":        roleString(user.Role),
		"tenant_type": tenantTypeString(user.TenantType),
		"iss":         i.issuer,
		"iat":         now.Unix(),
		"exp":         now.Add(i.ttl).Unix(),
	}
	if user.OrgId != nil && *user.OrgId != "" {
		claims["org_id"] = *user.OrgId
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

func roleString(r identity.UserRole) string {
	switch r {
	case identity.UserRole_USER_ROLE_CLIENT:
		return "client"
	case identity.UserRole_USER_ROLE_RESOURCE:
		return "resource"
	default:
		return "unspecified"
	}
}

func tenantTypeString(t identity.TenantType) string {
	switch t {
	case identity.TenantType_TENANT_TYPE_INDIVIDUAL:
		return "individual"
	case identity.TenantType_TENANT_TYPE_ORG:
		return "org"
	default:
		return "unspecified"
	}
}

func tenantTypeStringToEnum(s string) string {
	switch s {
	case "individual":
		return "TENANT_TYPE_INDIVIDUAL"
	case "org":
		return "TENANT_TYPE_ORG"
	default:
		return "TENANT_TYPE_UNSPECIFIED"
	}
}

func NewVerifier(secret, issuer string) *Verifier {
	return &Verifier{secret: []byte(secret), issuer: issuer}
}
func (v *Verifier) Verify(tokenString string) (Principal, error) {
	t, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return v.secret, nil
	}, jwt.WithIssuer(v.issuer))
	if err != nil || !t.Valid {
		return Principal{}, errors.New("invalid token")
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, errors.New("invalid token claims")
	}
	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return Principal{}, errors.New("missing subject")
	}
	roleString, _ := claims["role"].(string)
	role := identity.UserRole(identity.UserRole_value[roleStringToEnum(roleString)])
	if role == identity.UserRole_USER_ROLE_UNSPECIFIED {
		return Principal{}, errors.New("invalid role")
	}
	org, _ := claims["org_id"].(string)
	tenantTypeString, _ := claims["tenant_type"].(string)
	tenantType := identity.TenantType(identity.TenantType_value[tenantTypeStringToEnum(tenantTypeString)])
	if tenantType == identity.TenantType_TENANT_TYPE_UNSPECIFIED {
		// Backward-compatible interpretation for tokens issued before tenant_type existed.
		if org != "" {
			tenantType = identity.TenantType_TENANT_TYPE_ORG
		} else {
			tenantType = identity.TenantType_TENANT_TYPE_INDIVIDUAL
		}
	}
	return Principal{UserID: sub, OrgID: org, Role: role, TenantType: tenantType}, nil
}
func roleStringToEnum(s string) string {
	switch s {
	case "client":
		return "USER_ROLE_CLIENT"
	case "resource":
		return "USER_ROLE_RESOURCE"
	default:
		return "USER_ROLE_UNSPECIFIED"
	}
}

func Auth(v *Verifier, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		parts := strings.Fields(raw)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			http.Error(w, `{"code":"UNAUTHENTICATED","message":"authorization token is required"}`, http.StatusUnauthorized)
			return
		}
		p, err := v.Verify(parts[1])
		if err != nil {
			http.Error(w, `{"code":"UNAUTHENTICATED","message":"invalid authorization token"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, p)))
	})
}

func RequireRole(role identity.UserRole, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFromContext(r.Context())
		if !ok || p.Role != role {
			http.Error(w, `{"code":"PERMISSION_DENIED","message":"insufficient role"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireAnyRole(roles ...identity.UserRole) func(http.Handler) http.Handler {
	allowed := make(map[identity.UserRole]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			if !ok {
				http.Error(w, `{"code":"PERMISSION_DENIED","message":"insufficient role"}`, http.StatusForbidden)
				return
			}
			if _, exists := allowed[p.Role]; !exists {
				http.Error(w, `{"code":"PERMISSION_DENIED","message":"insufficient role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
