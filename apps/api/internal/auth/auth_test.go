package auth

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestArgon2PasswordHashing(t *testing.T) {
	password := "SecretP@ssword123!"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	match, err := ComparePassword(password, hash)
	if err != nil {
		t.Fatalf("failed to compare password: %v", err)
	}
	if !match {
		t.Fatalf("expected password to match hash")
	}

	wrongMatch, err := ComparePassword("WrongPassword!", hash)
	if err != nil {
		t.Fatalf("unexpected error on wrong password: %v", err)
	}
	if wrongMatch {
		t.Fatalf("expected wrong password not to match")
	}
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	manager := NewJWTManager("dev-test-jwt-secret-key-32chars!!")

	var userID, orgID pgtype.UUID
	_ = userID.Scan("10000000-0000-0000-0000-000000000001")
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	tokens, err := manager.GenerateTokenPair(userID, orgID, "alice@acme.com", RoleAdmin)
	if err != nil {
		t.Fatalf("failed to generate token pair: %v", err)
	}

	claims, err := manager.ValidateToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("failed to validate access token: %v", err)
	}

	if claims.Email != "alice@acme.com" {
		t.Errorf("expected email alice@acme.com, got %s", claims.Email)
	}
	if claims.Role != RoleAdmin {
		t.Errorf("expected role admin, got %s", claims.Role)
	}
	if claims.OrgID != "00000000-0000-0000-0000-000000000001" {
		t.Errorf("expected org ID 00000000-0000-0000-0000-000000000001, got %s", claims.OrgID)
	}
}

func TestAPIKeyGenerationAndHashing(t *testing.T) {
	key, err := GenerateAPIKey("acme")
	if err != nil {
		t.Fatalf("failed to generate API key: %v", err)
	}

	if key.KeyPrefix != "tr_live_acme" {
		t.Errorf("expected prefix tr_live_acme, got %s", key.KeyPrefix)
	}

	hash := HashAPIKey(key.RawKey)
	if hash != key.KeyHash {
		t.Errorf("expected deterministic hash %s, got %s", key.KeyHash, hash)
	}
}

func TestRBACHierarchy(t *testing.T) {
	tests := []struct {
		userRole     string
		requiredRole string
		expected     bool
	}{
		{RoleOwner, RoleViewer, true},
		{RoleAdmin, RoleResponder, true},
		{RoleResponder, RoleAdmin, false},
		{RoleViewer, RoleResponder, false},
		{RoleResponder, RoleResponder, true},
		{"unknown", RoleViewer, false},
	}

	for _, tc := range tests {
		result := HasMinimumRole(tc.userRole, tc.requiredRole)
		if result != tc.expected {
			t.Errorf("HasMinimumRole(%s, %s) = %v, want %v", tc.userRole, tc.requiredRole, result, tc.expected)
		}
	}
}

func TestJWTExpiredToken(t *testing.T) {
	manager := &JWTManager{
		secretKey:     []byte("test-key"),
		accessExpiry:  -1 * time.Second, // Already expired
		refreshExpiry: 1 * time.Hour,
	}

	var userID, orgID pgtype.UUID
	_ = userID.Scan("10000000-0000-0000-0000-000000000001")
	_ = orgID.Scan("00000000-0000-0000-0000-000000000001")

	tokens, err := manager.GenerateTokenPair(userID, orgID, "test@test.com", RoleViewer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = manager.ValidateToken(tokens.AccessToken)
	if err != ErrTokenExpired {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}
