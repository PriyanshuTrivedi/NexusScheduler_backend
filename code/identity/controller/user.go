package controller

import (
	"context"
	"strings"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/client/cache"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/util"
)

//go:generate mockgen -source=user.go -destination=../../../gen/mocks/identity/controller/user_mock.go -package=mocks

type Controller interface {
	RegisterClient(ctx context.Context, name string, identifier entity.UserIdentifier, password string) (entity.User, error)
	RegisterProvider(ctx context.Context, name string, identifier entity.UserIdentifier, password string, tenantType entity.TenantType, orgID string) (entity.User, error)
	Login(ctx context.Context, identifier entity.UserIdentifier, password string) (entity.User, error)
	UpdateProfile(ctx context.Context, userID, name string) (entity.User, error)
	SetUserStatus(ctx context.Context, userID string, isActive bool) (entity.User, error)
	GetUser(ctx context.Context, userID string) (entity.User, error)
	CreateOrganization(ctx context.Context, name string) (entity.Organization, error)
	GetOrganization(ctx context.Context, organizationID string) (entity.Organization, error)
	ListOrganizations(ctx context.Context) ([]entity.Organization, error)
	SetOrganizationStatus(ctx context.Context, organizationID string, isActive bool) (entity.Organization, error)
}

type identityController struct {
	store store.Store
	cache cache.Client
}

func New(s store.Store, caches ...cache.Client) Controller {
	var c cache.Client
	if len(caches) > 0 {
		c = caches[0]
	}
	return &identityController{store: s, cache: c}
}

func (c *identityController) RegisterClient(ctx context.Context, name string, identifier entity.UserIdentifier, password string) (entity.User, error) {
	u := entity.User{
		Name:       strings.TrimSpace(name),
		Identifier: identifier.Normalize(),
		Role:       entity.RoleClient,
		TenantType: entity.TenantTypeIndividual,
	}
	return c.registerUser(ctx, u, password)
}

func (c *identityController) RegisterProvider(ctx context.Context, name string, identifier entity.UserIdentifier, password string, tenantType entity.TenantType, orgID string) (entity.User, error) {
	orgID = strings.TrimSpace(orgID)
	if tenantType == entity.TenantTypeOrg {
		if orgID == "" {
			return entity.User{}, entity.ErrInvalidOrgID
		}
		org, err := c.store.GetOrganization(ctx, orgID)
		if err != nil {
			return entity.User{}, err
		}
		if !org.IsActive {
			return entity.User{}, store.ErrOrganizationNotActive
		}
	} else if tenantType == entity.TenantTypeIndividual {
		orgID = ""
	} else {
		return entity.User{}, entity.ErrInvalidTenantType
	}
	u := entity.User{Name: strings.TrimSpace(name), Identifier: identifier.Normalize(), Role: entity.RoleResource, TenantType: tenantType, OrgID: orgID}
	return c.registerUser(ctx, u, password)
}

func (c *identityController) registerUser(ctx context.Context, u entity.User, password string) (entity.User, error) {
	if err := u.ValidateForRegistration(); err != nil {
		return entity.User{}, err
	}
	if password == "" {
		return entity.User{}, entity.ErrInvalidPassword
	}
	hash, err := util.HashPassword(password)
	if err != nil {
		return entity.User{}, err
	}
	u.PasswordHash = hash
	return c.store.CreateUser(ctx, u)
}

func (c *identityController) Login(ctx context.Context, identifier entity.UserIdentifier, password string) (entity.User, error) {
	identifier = identifier.Normalize()
	if err := identifier.Validate(); err != nil {
		return entity.User{}, err
	}
	if password == "" {
		return entity.User{}, entity.ErrInvalidPassword
	}
	u, err := c.store.GetUserByIdentifier(ctx, identifier)
	if err != nil {
		return entity.User{}, err
	}
	if !u.IsActive || !util.CheckPassword(u.PasswordHash, password) {
		return entity.User{}, store.ErrUserNotFound
	}
	return u, nil
}

func (c *identityController) UpdateProfile(ctx context.Context, userID, name string) (entity.User, error) {
	if strings.TrimSpace(userID) == "" {
		return entity.User{}, entity.ErrInvalidUserID
	}
	if strings.TrimSpace(name) == "" {
		return entity.User{}, entity.ErrInvalidName
	}
	return c.store.UpdateUserProfile(ctx, userID, strings.TrimSpace(name))
}

func (c *identityController) SetUserStatus(ctx context.Context, userID string, isActive bool) (entity.User, error) {
	if strings.TrimSpace(userID) == "" {
		return entity.User{}, entity.ErrInvalidUserID
	}
	return c.store.SetUserStatus(ctx, userID, isActive)
}

func (c *identityController) GetUser(ctx context.Context, userID string) (entity.User, error) {
	if strings.TrimSpace(userID) == "" {
		return entity.User{}, entity.ErrInvalidUserID
	}
	return c.store.GetUserByID(ctx, userID)
}

func (c *identityController) CreateOrganization(ctx context.Context, name string) (entity.Organization, error) {
	org := entity.Organization{Name: strings.TrimSpace(name)}
	if err := org.Validate(); err != nil {
		return entity.Organization{}, err
	}
	created, err := c.store.CreateOrganization(ctx, org.Name)
	if err == nil {
		c.refreshOrganizationsCache(ctx)
	}
	return created, err
}

func (c *identityController) SetOrganizationStatus(ctx context.Context, organizationID string, isActive bool) (entity.Organization, error) {
	if strings.TrimSpace(organizationID) == "" {
		return entity.Organization{}, entity.ErrInvalidOrgID
	}
	updated, err := c.store.SetOrganizationStatus(ctx, organizationID, isActive)
	if err == nil {
		c.refreshOrganizationsCache(ctx)
	}
	return updated, err
}

func (c *identityController) GetOrganization(ctx context.Context, organizationID string) (entity.Organization, error) {
	if strings.TrimSpace(organizationID) == "" {
		return entity.Organization{}, entity.ErrInvalidOrgID
	}
	return c.store.GetOrganization(ctx, organizationID)
}

func (c *identityController) refreshOrganizationsCache(ctx context.Context) {
	if c.cache == nil {
		return
	}
	organizations, err := c.store.ListOrganizations(ctx)
	if err != nil {
		return
	}
	_ = c.cache.SetOrganizations(ctx, organizations)
}

func (c *identityController) ListOrganizations(ctx context.Context) ([]entity.Organization, error) {
	if c.cache != nil {
		if organizations, ok, cacheErr := c.cache.GetOrganizations(ctx); cacheErr == nil && ok {
			return organizations, nil
		}
	}

	organizations, err := c.store.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}

	if c.cache != nil {
		_ = c.cache.SetOrganizations(ctx, organizations)
	}
	return organizations, nil
}
