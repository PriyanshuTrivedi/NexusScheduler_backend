package entity

import (
	"errors"
	"strings"
	"time"
)

type Role int

const (
	RoleUnspecified Role = iota
	RoleClient
	RoleResource
)

func (r Role) String() string {
	switch r {
	case RoleClient:
		return "client"
	case RoleResource:
		return "resource"
	default:
		return "unspecified"
	}
}

func ParseRole(v string) Role {
	switch v {
	case "client":
		return RoleClient
	case "resource":
		return RoleResource
	default:
		return RoleUnspecified
	}
}

type TenantType int

const (
	TenantTypeUnspecified TenantType = iota
	TenantTypeIndividual
	TenantTypeOrg
)

func (t TenantType) String() string {
	switch t {
	case TenantTypeIndividual:
		return "individual"
	case TenantTypeOrg:
		return "org"
	default:
		return "unspecified"
	}
}

func ParseTenantType(v string) TenantType {
	switch v {
	case "individual":
		return TenantTypeIndividual
	case "org":
		return TenantTypeOrg
	default:
		return TenantTypeUnspecified
	}
}

type UserIdentifier struct {
	Email string
	Phone string
}

func (i UserIdentifier) Normalize() UserIdentifier {
	return UserIdentifier{Email: strings.ToLower(strings.TrimSpace(i.Email)), Phone: strings.TrimSpace(i.Phone)}
}

func (i UserIdentifier) Validate() error {
	i = i.Normalize()
	if (i.Email == "") == (i.Phone == "") {
		return ErrInvalidIdentifier
	}
	return nil
}

type Organization struct {
	ID        string
	Name      string
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (o Organization) Validate() error {
	if strings.TrimSpace(o.Name) == "" {
		return ErrInvalidOrganizationName
	}
	return nil
}

type User struct {
	ID           string
	TenantType   TenantType
	OrgID        string
	Name         string
	Identifier   UserIdentifier
	PasswordHash string
	Role         Role
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u User) ValidateForRegistration() error {
	if strings.TrimSpace(u.Name) == "" {
		return ErrInvalidName
	}
	if err := u.Identifier.Validate(); err != nil {
		return err
	}
	if u.TenantType == TenantTypeUnspecified {
		return ErrInvalidTenantType
	}
	if u.TenantType == TenantTypeIndividual && u.OrgID != "" {
		return ErrInvalidOrgID
	}
	if u.TenantType == TenantTypeOrg && u.OrgID == "" {
		return ErrInvalidOrgID
	}
	return nil
}

var (
	ErrInvalidUserID           = errors.New("entity: user_id is required")
	ErrInvalidName             = errors.New("entity: name is required")
	ErrInvalidEmail            = errors.New("entity: email is required")
	ErrInvalidPhone            = errors.New("entity: phone is required")
	ErrInvalidIdentifier       = errors.New("entity: exactly one of email or phone is required")
	ErrInvalidPassword         = errors.New("entity: password is required")
	ErrInvalidOrgID            = errors.New("entity: org_id is required")
	ErrInvalidOrganizationName = errors.New("entity: organization name is required")
	ErrInvalidRole             = errors.New("entity: invalid role")
	ErrInvalidTenantType       = errors.New("entity: invalid tenant type")
)
