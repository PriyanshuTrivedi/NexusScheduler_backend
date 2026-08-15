package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PriyanshuTrivedi/nexus-scheduler/code/identity/entity"
)

const tableOrganization, tableUser = "organization", "app_user"

var (
	ErrUserNotFound               = errors.New("store: user not found")
	ErrUserExists                 = errors.New("store: user already exists")
	ErrOrganizationNotFound       = errors.New("store: organization not found")
	ErrOrganizationExists         = errors.New("store: organization already exists")
	ErrOrganizationNotActive      = errors.New("store: organization is inactive")
	ErrOrganizationMustBeInactive = errors.New("store: organization must be inactive before deletion")
	ErrOrganizationHasUsers       = errors.New("store: organization still has users")
)

//go:generate mockgen -source=user.go -destination=../../../gen/mocks/identity/store/user_mock.go -package=mocks

type Store interface {
	CreateOrganization(ctx context.Context, name string) (entity.Organization, error)
	GetOrganization(ctx context.Context, organizationID string) (entity.Organization, error)
	ListOrganizations(ctx context.Context) ([]entity.Organization, error)
	SetOrganizationStatus(ctx context.Context, organizationID string, isActive bool) (entity.Organization, error)
	CreateUser(ctx context.Context, u entity.User) (entity.User, error)
	GetUserByID(ctx context.Context, id string) (entity.User, error)
	GetUserByIdentifier(ctx context.Context, identifier entity.UserIdentifier) (entity.User, error)
	UpdateUserProfile(ctx context.Context, userID, name string) (entity.User, error)
	SetUserStatus(ctx context.Context, userID string, isActive bool) (entity.User, error)
}

type pgStore struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) Store { return &pgStore{pool: pool} }

func (s *pgStore) CreateOrganization(ctx context.Context, name string) (entity.Organization, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(
		`SELECT EXISTS (SELECT 1 FROM %s WHERE lower(name) = lower($1))`,
		tableOrganization,
	), name).Scan(&exists); err != nil {
		return entity.Organization{}, fmt.Errorf("store: check organization: %w", err)
	}
	if exists {
		return entity.Organization{}, ErrOrganizationExists
	}

	var org entity.Organization
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        INSERT INTO %s (name) VALUES ($1)
        RETURNING id, name, is_active, created_at, updated_at
    `, tableOrganization), name).Scan(&org.ID, &org.Name, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.Organization{}, ErrOrganizationExists
		}
		return entity.Organization{}, fmt.Errorf("store: create organization: %w", err)
	}
	return org, nil
}

func (s *pgStore) GetOrganization(ctx context.Context, organizationID string) (entity.Organization, error) {
	var org entity.Organization
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        SELECT id, name, is_active, created_at, updated_at
        FROM %s WHERE id = $1
    `, tableOrganization), organizationID).Scan(&org.ID, &org.Name, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Organization{}, ErrOrganizationNotFound
	}
	if err != nil {
		return entity.Organization{}, fmt.Errorf("store: get organization: %w", err)
	}
	return org, nil
}

func (s *pgStore) SetOrganizationStatus(ctx context.Context, organizationID string, isActive bool) (entity.Organization, error) {
	var org entity.Organization
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`UPDATE %s SET is_active=$2,updated_at=now() WHERE id=$1 RETURNING id,name,is_active,created_at,updated_at`, tableOrganization), organizationID, isActive).Scan(&org.ID, &org.Name, &org.IsActive, &org.CreatedAt, &org.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.Organization{}, ErrOrganizationNotFound
	}
	if err != nil {
		return entity.Organization{}, fmt.Errorf("store: set organization status: %w", err)
	}
	return org, nil
}

func (s *pgStore) ListOrganizations(ctx context.Context) ([]entity.Organization, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
        SELECT id, name, is_active, created_at, updated_at
        FROM %s
        WHERE is_active = TRUE
        ORDER BY lower(name), id
    `, tableOrganization))
	if err != nil {
		return nil, fmt.Errorf("store: list organizations: %w", err)
	}
	defer rows.Close()

	organizations := make([]entity.Organization, 0)
	for rows.Next() {
		var org entity.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.IsActive, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, fmt.Errorf("store: scan organization: %w", err)
		}
		organizations = append(organizations, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list organizations rows: %w", err)
	}
	return organizations, nil
}

func (s *pgStore) CreateUser(ctx context.Context, u entity.User) (entity.User, error) {
	var out entity.User
	var role, tenantType string
	i := u.Identifier.Normalize()
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        INSERT INTO %s (tenant_type, org_id, name, email, phone, password_hash, role, is_active)
        VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF(lower($4), ''), NULLIF($5, ''), $6, $7, TRUE)
        RETURNING id, tenant_type, COALESCE(org_id::text, ''), name, COALESCE(email, ''), COALESCE(phone, ''), password_hash, role, is_active, created_at, updated_at
    `, tableUser), u.TenantType.String(), u.OrgID, u.Name, i.Email, i.Phone, u.PasswordHash, u.Role.String()).Scan(
		&out.ID, &tenantType, &out.OrgID, &out.Name, &out.Identifier.Email, &out.Identifier.Phone, &out.PasswordHash, &role, &out.IsActive, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return entity.User{}, ErrUserExists
		}
		return entity.User{}, fmt.Errorf("store: create user: %w", err)
	}
	out.TenantType = entity.ParseTenantType(tenantType)
	out.Role = entity.ParseRole(role)
	return out, nil
}

func (s *pgStore) GetUserByID(ctx context.Context, id string) (entity.User, error) {
	var u entity.User
	var role, tenantType string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        SELECT id, tenant_type, COALESCE(org_id::text, ''), name, COALESCE(email, ''), COALESCE(phone, ''), password_hash, role, is_active, created_at, updated_at
        FROM %s WHERE id = $1
    `, tableUser), id).Scan(&u.ID, &tenantType, &u.OrgID, &u.Name, &u.Identifier.Email, &u.Identifier.Phone, &u.PasswordHash, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.User{}, ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("store: get user: %w", err)
	}
	u.TenantType = entity.ParseTenantType(tenantType)
	u.Role = entity.ParseRole(role)
	return u, nil
}

func (s *pgStore) GetUserByIdentifier(ctx context.Context, identifier entity.UserIdentifier) (entity.User, error) {
	i := identifier.Normalize()
	var u entity.User
	var role, tenantType string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        SELECT id, tenant_type, COALESCE(org_id::text, ''), name, COALESCE(email, ''), COALESCE(phone, ''), password_hash, role, is_active, created_at, updated_at
        FROM %s
        WHERE ($1 <> '' AND email = lower($1)) OR ($2 <> '' AND phone = $2)
    `, tableUser), i.Email, i.Phone).Scan(&u.ID, &tenantType, &u.OrgID, &u.Name, &u.Identifier.Email, &u.Identifier.Phone, &u.PasswordHash, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.User{}, ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("store: get user by identifier: %w", err)
	}
	u.TenantType = entity.ParseTenantType(tenantType)
	u.Role = entity.ParseRole(role)
	return u, nil
}

func (s *pgStore) UpdateUserProfile(ctx context.Context, userID, name string) (entity.User, error) {
	var u entity.User
	var role, tenantType string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        UPDATE %s SET name = $2, updated_at = now()
        WHERE id = $1
        RETURNING id, tenant_type, COALESCE(org_id::text, ''), name, COALESCE(email, ''), COALESCE(phone, ''), password_hash, role, is_active, created_at, updated_at
    `, tableUser), userID, name).Scan(&u.ID, &tenantType, &u.OrgID, &u.Name, &u.Identifier.Email, &u.Identifier.Phone, &u.PasswordHash, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.User{}, ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("store: update profile: %w", err)
	}
	u.TenantType = entity.ParseTenantType(tenantType)
	u.Role = entity.ParseRole(role)
	return u, nil
}

func (s *pgStore) SetUserStatus(ctx context.Context, userID string, isActive bool) (entity.User, error) {
	var u entity.User
	var role, tenantType string
	err := s.pool.QueryRow(ctx, fmt.Sprintf(`
        UPDATE %s SET is_active = $2, updated_at = now()
        WHERE id = $1
        RETURNING id, tenant_type, COALESCE(org_id::text, ''), name, COALESCE(email, ''), COALESCE(phone, ''), password_hash, role, is_active, created_at, updated_at
    `, tableUser), userID, isActive).Scan(&u.ID, &tenantType, &u.OrgID, &u.Name, &u.Identifier.Email, &u.Identifier.Phone, &u.PasswordHash, &role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.User{}, ErrUserNotFound
	}
	if err != nil {
		return entity.User{}, fmt.Errorf("store: set user status: %w", err)
	}
	u.TenantType = entity.ParseTenantType(tenantType)
	u.Role = entity.ParseRole(role)
	return u, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key"))
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
