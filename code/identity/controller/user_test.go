package controller

import (
	"context"
	"errors"
	"testing"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/store"
	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/util"
	cachemock "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/client/cache"
	storemock "github.com/PriyanshuTrivedi/nexus-scheduler/gen/mocks/identity/store"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRegisterClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u entity.User) (entity.User, error) {
			require.Equal(t, "John Doe", u.Name)
			require.Equal(t, entity.RoleClient, u.Role)
			require.Equal(t, entity.TenantTypeIndividual, u.TenantType)
			require.NotEmpty(t, u.PasswordHash)

			return u, nil
		})

	c := New(st)

	got, err := c.RegisterClient(
		context.Background(),
		" John Doe ",
		entity.UserIdentifier{Email: "john@example.com"},
		"password",
	)

	require.NoError(t, err)
	require.Equal(t, "John Doe", got.Name)
	require.Equal(t, entity.RoleClient, got.Role)
}

func TestRegisterClient_InvalidPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.RegisterClient(
		context.Background(),
		"John Doe",
		entity.UserIdentifier{Email: "john@example.com"},
		"",
	)

	require.ErrorIs(t, err, entity.ErrInvalidPassword)
}

func TestRegisterProvider_Organization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetOrganization(gomock.Any(), "org-1").
		Return(entity.Organization{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		}, nil)

	st.EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u entity.User) (entity.User, error) {
			require.Equal(t, entity.RoleResource, u.Role)
			require.Equal(t, entity.TenantTypeOrg, u.TenantType)
			require.Equal(t, "org-1", u.OrgID)
			require.NotEmpty(t, u.PasswordHash)

			return u, nil
		})

	c := New(st)

	got, err := c.RegisterProvider(
		context.Background(),
		" Provider ",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeOrg,
		" org-1 ",
	)

	require.NoError(t, err)
	require.Equal(t, entity.RoleResource, got.Role)
	require.Equal(t, entity.TenantTypeOrg, got.TenantType)
	require.Equal(t, "org-1", got.OrgID)
}

func TestRegisterProvider_OrganizationNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetOrganization(gomock.Any(), "org-1").
		Return(entity.Organization{}, errors.New("organization not found"))

	c := New(st)

	_, err := c.RegisterProvider(
		context.Background(),
		"Provider",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeOrg,
		"org-1",
	)

	require.Error(t, err)
}

func TestRegisterProvider_OrganizationInactive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetOrganization(gomock.Any(), "org-1").
		Return(entity.Organization{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: false,
		}, nil)

	c := New(st)

	_, err := c.RegisterProvider(
		context.Background(),
		"Provider",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeOrg,
		"org-1",
	)

	require.ErrorIs(t, err, store.ErrOrganizationNotActive)
}

func TestRegisterProvider_OrganizationMissingID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.RegisterProvider(
		context.Background(),
		"Provider",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeOrg,
		"",
	)

	require.ErrorIs(t, err, entity.ErrInvalidOrgID)
}

func TestRegisterProvider_Individual(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		CreateUser(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, u entity.User) (entity.User, error) {
			require.Equal(t, entity.RoleResource, u.Role)
			require.Equal(t, entity.TenantTypeIndividual, u.TenantType)
			require.Empty(t, u.OrgID)

			return u, nil
		})

	c := New(st)

	got, err := c.RegisterProvider(
		context.Background(),
		"Provider",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeIndividual,
		"some-org",
	)

	require.NoError(t, err)
	require.Empty(t, got.OrgID)
}

func TestRegisterProvider_InvalidTenantType(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.RegisterProvider(
		context.Background(),
		"Provider",
		entity.UserIdentifier{Email: "provider@example.com"},
		"password",
		entity.TenantTypeUnspecified,
		"",
	)

	require.ErrorIs(t, err, entity.ErrInvalidTenantType)
}

func TestLogin_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetUserByIdentifier(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, identifier entity.UserIdentifier) (entity.User, error) {
			return entity.User{
				ID:           "user-1",
				Name:         "John",
				Identifier:   identifier,
				PasswordHash: mustHashPassword(t, "password"),
				IsActive:     true,
			}, nil
		})

	c := New(st)

	got, err := c.Login(
		context.Background(),
		entity.UserIdentifier{Email: "john@example.com"},
		"password",
	)

	require.NoError(t, err)
	require.Equal(t, "user-1", got.ID)
}

func TestLogin_InvalidPassword(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.Login(
		context.Background(),
		entity.UserIdentifier{Email: "john@example.com"},
		"",
	)

	require.ErrorIs(t, err, entity.ErrInvalidPassword)
}

func TestLogin_UserNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetUserByIdentifier(gomock.Any(), gomock.Any()).
		Return(entity.User{}, store.ErrUserNotFound)

	c := New(st)

	_, err := c.Login(
		context.Background(),
		entity.UserIdentifier{Email: "john@example.com"},
		"password",
	)

	require.ErrorIs(t, err, store.ErrUserNotFound)
}

func TestLogin_InactiveUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetUserByIdentifier(gomock.Any(), gomock.Any()).
		Return(entity.User{
			ID:           "user-1",
			PasswordHash: mustHashPassword(t, "password"),
			IsActive:     false,
		}, nil)

	c := New(st)

	_, err := c.Login(
		context.Background(),
		entity.UserIdentifier{Email: "john@example.com"},
		"password",
	)

	require.ErrorIs(t, err, store.ErrUserNotFound)
}

func TestUpdateProfile(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		UpdateUserProfile(gomock.Any(), "user-1", "John Updated").
		Return(entity.User{
			ID:   "user-1",
			Name: "John Updated",
		}, nil)

	c := New(st)

	got, err := c.UpdateProfile(
		context.Background(),
		"user-1",
		" John Updated ",
	)

	require.NoError(t, err)
	require.Equal(t, "John Updated", got.Name)
}

func TestUpdateProfile_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.UpdateProfile(
		context.Background(),
		" ",
		"John",
	)

	require.ErrorIs(t, err, entity.ErrInvalidUserID)
}

func TestUpdateProfile_InvalidName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.UpdateProfile(
		context.Background(),
		"user-1",
		" ",
	)

	require.ErrorIs(t, err, entity.ErrInvalidName)
}

func TestSetUserStatus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		SetUserStatus(gomock.Any(), "user-1", false).
		Return(entity.User{
			ID:       "user-1",
			IsActive: false,
		}, nil)

	c := New(st)

	got, err := c.SetUserStatus(
		context.Background(),
		"user-1",
		false,
	)

	require.NoError(t, err)
	require.False(t, got.IsActive)
}

func TestSetUserStatus_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.SetUserStatus(
		context.Background(),
		"",
		false,
	)

	require.ErrorIs(t, err, entity.ErrInvalidUserID)
}

func TestGetUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetUserByID(gomock.Any(), "user-1").
		Return(entity.User{
			ID:   "user-1",
			Name: "John",
		}, nil)

	c := New(st)

	got, err := c.GetUser(context.Background(), "user-1")

	require.NoError(t, err)
	require.Equal(t, "user-1", got.ID)
}

func TestGetUser_InvalidUserID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.GetUser(context.Background(), "")

	require.ErrorIs(t, err, entity.ErrInvalidUserID)
}

func TestCreateOrganization_Success_RefreshesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	created := entity.Organization{
		ID:       "org-1",
		Name:     "Acme",
		IsActive: true,
	}

	organizations := []entity.Organization{created}

	st.EXPECT().
		CreateOrganization(gomock.Any(), "Acme").
		Return(created, nil)

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(organizations, nil)

	cch.EXPECT().
		SetOrganizations(gomock.Any(), organizations).
		Return(nil)

	c := New(st, cch)

	got, err := c.CreateOrganization(
		context.Background(),
		" Acme ",
	)

	require.NoError(t, err)
	require.Equal(t, created, got)
}

func TestCreateOrganization_StoreError_DoesNotRefreshCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	storeErr := errors.New("create organization failed")

	st.EXPECT().
		CreateOrganization(gomock.Any(), "Acme").
		Return(entity.Organization{}, storeErr)

	c := New(st, cch)

	got, err := c.CreateOrganization(
		context.Background(),
		" Acme ",
	)

	require.ErrorIs(t, err, storeErr)
	require.Equal(t, entity.Organization{}, got)
}

func TestCreateOrganization_InvalidName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	c := New(st, cch)

	_, err := c.CreateOrganization(
		context.Background(),
		" ",
	)

	require.Error(t, err)
}

func TestSetOrganizationStatus_Success_RefreshesCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	updated := entity.Organization{
		ID:       "org-1",
		Name:     "Acme",
		IsActive: false,
	}

	organizations := []entity.Organization{}

	st.EXPECT().
		SetOrganizationStatus(gomock.Any(), " org-1 ", false).
		Return(updated, nil)

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(organizations, nil)

	cch.EXPECT().
		SetOrganizations(gomock.Any(), organizations).
		Return(nil)

	c := New(st, cch)

	got, err := c.SetOrganizationStatus(
		context.Background(),
		" org-1 ",
		false,
	)

	require.NoError(t, err)
	require.Equal(t, updated, got)
}

func TestSetOrganizationStatus_StoreError_DoesNotRefreshCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	storeErr := errors.New("update organization failed")

	st.EXPECT().
		SetOrganizationStatus(gomock.Any(), "org-1", false).
		Return(entity.Organization{}, storeErr)

	c := New(st, cch)

	_, err := c.SetOrganizationStatus(
		context.Background(),
		"org-1",
		false,
	)

	require.ErrorIs(t, err, storeErr)
}

func TestSetOrganizationStatus_InvalidOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	c := New(st, cch)

	_, err := c.SetOrganizationStatus(
		context.Background(),
		"",
		false,
	)

	require.ErrorIs(t, err, entity.ErrInvalidOrgID)
}

func TestGetOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	st.EXPECT().
		GetOrganization(gomock.Any(), "org-1").
		Return(entity.Organization{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		}, nil)

	c := New(st)

	got, err := c.GetOrganization(
		context.Background(),
		"org-1",
	)

	require.NoError(t, err)
	require.Equal(t, "org-1", got.ID)
}

func TestGetOrganization_InvalidOrganizationID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	c := New(st)

	_, err := c.GetOrganization(
		context.Background(),
		" ",
	)

	require.ErrorIs(t, err, entity.ErrInvalidOrgID)
}

func TestListOrganizations_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	organizations := []entity.Organization{
		{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		},
	}

	cch.EXPECT().
		GetOrganizations(gomock.Any()).
		Return(organizations, true, nil)

	c := New(st, cch)

	got, err := c.ListOrganizations(context.Background())

	require.NoError(t, err)
	require.Equal(t, organizations, got)
}

func TestListOrganizations_CacheMiss_LoadsStoreAndCaches(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	organizations := []entity.Organization{
		{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		},
		{
			ID:       "org-2",
			Name:     "Globex",
			IsActive: true,
		},
	}

	cch.EXPECT().
		GetOrganizations(gomock.Any()).
		Return(nil, false, nil)

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(organizations, nil)

	cch.EXPECT().
		SetOrganizations(gomock.Any(), organizations).
		Return(nil)

	c := New(st, cch)

	got, err := c.ListOrganizations(context.Background())

	require.NoError(t, err)
	require.Equal(t, organizations, got)
}

func TestListOrganizations_CacheError_FallsBackToStore(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	organizations := []entity.Organization{
		{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		},
	}

	cch.EXPECT().
		GetOrganizations(gomock.Any()).
		Return(nil, false, errors.New("redis unavailable"))

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(organizations, nil)

	cch.EXPECT().
		SetOrganizations(gomock.Any(), organizations).
		Return(nil)

	c := New(st, cch)

	got, err := c.ListOrganizations(context.Background())

	require.NoError(t, err)
	require.Equal(t, organizations, got)
}

func TestListOrganizations_StoreError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)
	cch := cachemock.NewMockClient(ctrl)

	storeErr := errors.New("database unavailable")

	cch.EXPECT().
		GetOrganizations(gomock.Any()).
		Return(nil, false, nil)

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(nil, storeErr)

	c := New(st, cch)

	got, err := c.ListOrganizations(context.Background())

	require.ErrorIs(t, err, storeErr)
	require.Nil(t, got)
}

func TestListOrganizations_WithoutCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	st := storemock.NewMockStore(ctrl)

	organizations := []entity.Organization{
		{
			ID:       "org-1",
			Name:     "Acme",
			IsActive: true,
		},
	}

	st.EXPECT().
		ListOrganizations(gomock.Any()).
		Return(organizations, nil)

	c := New(st)

	got, err := c.ListOrganizations(context.Background())

	require.NoError(t, err)
	require.Equal(t, organizations, got)
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()

	hash, err := util.HashPassword(password)
	require.NoError(t, err)

	return hash
}
