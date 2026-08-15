package util

import (
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	identitypb "github.com/PriyanshuTrivedi/nexus-scheduler/gen/idl/identity"
)

func RoleFromProto(r identitypb.UserRole) entity.Role {
	switch r {
	case identitypb.UserRole_USER_ROLE_CLIENT:
		return entity.RoleClient
	case identitypb.UserRole_USER_ROLE_RESOURCE:
		return entity.RoleResource
	default:
		return entity.RoleUnspecified
	}
}

func RoleToProto(r entity.Role) identitypb.UserRole {
	switch r {
	case entity.RoleClient:
		return identitypb.UserRole_USER_ROLE_CLIENT
	case entity.RoleResource:
		return identitypb.UserRole_USER_ROLE_RESOURCE
	default:
		return identitypb.UserRole_USER_ROLE_UNSPECIFIED
	}
}

func IdentifierFromProto(i *identitypb.UserIdentifier) entity.UserIdentifier {
	if i == nil {
		return entity.UserIdentifier{}
	}
	return entity.UserIdentifier{Email: i.GetEmail(), Phone: i.GetPhone()}.Normalize()
}

func IdentifierToProto(i entity.UserIdentifier) *identitypb.UserIdentifier {
	i = i.Normalize()
	out := &identitypb.UserIdentifier{}
	if i.Email != "" {
		out.Value = &identitypb.UserIdentifier_Email{Email: i.Email}
	} else if i.Phone != "" {
		out.Value = &identitypb.UserIdentifier_Phone{Phone: i.Phone}
	}
	return out
}

func TenantTypeFromProto(t identitypb.TenantType) entity.TenantType {
	switch t {
	case identitypb.TenantType_TENANT_TYPE_INDIVIDUAL:
		return entity.TenantTypeIndividual
	case identitypb.TenantType_TENANT_TYPE_ORG:
		return entity.TenantTypeOrg
	default:
		return entity.TenantTypeUnspecified
	}
}

func TenantTypeToProto(t entity.TenantType) identitypb.TenantType {
	switch t {
	case entity.TenantTypeIndividual:
		return identitypb.TenantType_TENANT_TYPE_INDIVIDUAL
	case entity.TenantTypeOrg:
		return identitypb.TenantType_TENANT_TYPE_ORG
	default:
		return identitypb.TenantType_TENANT_TYPE_UNSPECIFIED
	}
}

func OrganizationToProto(org entity.Organization) *identitypb.Organization {
	return &identitypb.Organization{OrganizationId: org.ID, Name: org.Name, IsActive: org.IsActive, CreatedAtUnix: org.CreatedAt.Unix(), UpdatedAtUnix: org.UpdatedAt.Unix()}
}

func UserToProto(u entity.User) *identitypb.User {
	out := &identitypb.User{
		UserId:        u.ID,
		Name:          u.Name,
		Identifier:    IdentifierToProto(u.Identifier),
		Role:          RoleToProto(u.Role),
		TenantType:    TenantTypeToProto(u.TenantType),
		IsActive:      u.IsActive,
		CreatedAtUnix: u.CreatedAt.Unix(),
		UpdatedAtUnix: u.UpdatedAt.Unix(),
	}
	if u.OrgID != "" {
		out.OrgId = &u.OrgID
	}
	return out
}

func normalize(s string) string { return strings.TrimSpace(s) }
