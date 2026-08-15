package controller

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
)

type testStore struct {
	createUserFn func(context.Context, entity.User) (entity.User, error)
	getOrgFn     func(context.Context, string) (entity.Organization, error)
}

func (s *testStore) CreateOrganization(context.Context, string) (entity.Organization, error) {
	return entity.Organization{}, nil
}
func (s *testStore) UpdateOrganization(context.Context, string, string) (entity.Organization, error) {
	return entity.Organization{}, nil
}
func (s *testStore) GetOrganization(ctx context.Context, id string) (entity.Organization, error) {
	if s.getOrgFn != nil {
		return s.getOrgFn(ctx, id)
	}
	return entity.Organization{}, nil
}
func (s *testStore) ListOrganizations(context.Context) ([]entity.Organization, error) {
	return nil, nil
}
func (s *testStore) SetOrganizationStatus(context.Context, string, bool) (entity.Organization, error) {
	return entity.Organization{}, nil
}
func (s *testStore) DeleteOrganization(context.Context, string) error { return nil }
func (s *testStore) CreateUser(ctx context.Context, u entity.User) (entity.User, error) {
	if s.createUserFn != nil {
		return s.createUserFn(ctx, u)
	}
	return u, nil
}
func (s *testStore) GetUserByID(context.Context, string) (entity.User, error) {
	return entity.User{}, nil
}
func (s *testStore) GetUserByIdentifier(context.Context, entity.UserIdentifier) (entity.User, error) {
	return entity.User{}, nil
}
func (s *testStore) UpdateUserProfile(context.Context, string, string) (entity.User, error) {
	return entity.User{}, nil
}
func (s *testStore) SetUserStatus(context.Context, string, bool) (entity.User, error) {
	return entity.User{}, nil
}

func TestRegisterClientWithEmail(t *testing.T) {
	st := &testStore{createUserFn: func(_ context.Context, u entity.User) (entity.User, error) {
		u.ID = "u-1"
		u.IsActive = true
		return u, nil
	}}
	c := New(st)

	u, err := c.RegisterClient(context.Background(), "Alice", entity.UserIdentifier{Email: "a@example.com"}, "password")
	assert.NoError(t, err)
	assert.Equal(t, entity.RoleClient, u.Role)
	assert.Equal(t, entity.TenantTypeIndividual, u.TenantType)
	assert.Equal(t, "a@example.com", u.Identifier.Email)
}

func TestRegisterProviderAsIndividualResource(t *testing.T) {
	st := &testStore{createUserFn: func(_ context.Context, u entity.User) (entity.User, error) {
		u.ID = "r-1"
		u.IsActive = true
		return u, nil
	}}
	c := New(st)

	u, err := c.RegisterProvider(context.Background(), "Dr. Mehta", entity.UserIdentifier{Email: "doctor@example.com"}, "password", entity.TenantTypeIndividual, "")
	assert.NoError(t, err)
	assert.Equal(t, entity.RoleResource, u.Role)
	assert.Equal(t, entity.TenantTypeIndividual, u.TenantType)
}

func TestRegisterProviderAsOrganizationResource(t *testing.T) {
	st := &testStore{
		getOrgFn: func(context.Context, string) (entity.Organization, error) {
			return entity.Organization{ID: "org-1", Name: "Apollo", IsActive: true}, nil
		},
		createUserFn: func(_ context.Context, u entity.User) (entity.User, error) {
			u.ID = "u-1"
			u.IsActive = true
			return u, nil
		},
	}
	c := New(st)

	u, err := c.RegisterProvider(context.Background(), "Alice", entity.UserIdentifier{Email: "a@example.com"}, "password", entity.TenantTypeOrg, "org-1")
	assert.NoError(t, err)
	assert.Equal(t, entity.RoleResource, u.Role)
	assert.Equal(t, entity.TenantTypeOrg, u.TenantType)
	assert.Equal(t, "org-1", u.OrgID)
}
