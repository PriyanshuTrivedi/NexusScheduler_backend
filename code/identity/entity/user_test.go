package entity

import "testing"

func TestUserValidateForRegistration(t *testing.T) {
	if err := (User{Name: "Alice", TenantType: TenantTypeIndividual, Identifier: UserIdentifier{Email: "a@example.com"}}).ValidateForRegistration(); err != nil {
		t.Fatal(err)
	}
	if err := (User{Name: "Alice", TenantType: TenantTypeIndividual, Identifier: UserIdentifier{Phone: "999"}}).ValidateForRegistration(); err != nil {
		t.Fatal(err)
	}
	if err := (User{Identifier: UserIdentifier{Email: "a@example.com"}}).ValidateForRegistration(); err != ErrInvalidName {
		t.Fatalf("got %v", err)
	}
	if err := (User{Name: "Alice"}).ValidateForRegistration(); err != ErrInvalidIdentifier {
		t.Fatalf("got %v", err)
	}
	if err := (User{Name: "Alice", TenantType: TenantTypeIndividual, Identifier: UserIdentifier{Email: "a@example.com", Phone: "999"}}).ValidateForRegistration(); err != ErrInvalidIdentifier {
		t.Fatalf("got %v", err)
	}
}

func TestRoleRoundTrip(t *testing.T) {
	for _, r := range []Role{RoleClient, RoleResource} {
		if ParseRole(r.String()) != r {
			t.Fatalf("role %v did not round trip", r)
		}
	}
}
