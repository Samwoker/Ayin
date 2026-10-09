package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/server/middleware"
	"github.com/seraph/triage/apps/api/internal/store"
	"github.com/seraph/triage/apps/api/internal/store/db"
)

// AuthHandler handles authentication, token refresh, and API key management.
type AuthHandler struct {
	store      *store.Store
	jwtManager *auth.JWTManager
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(st *store.Store, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		store:      st,
		jwtManager: jwtManager,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OrgID    string `json:"org_id,omitempty"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
	User         UserDTO `json:"user"`
	Org          OrgDTO  `json:"org"`
}

type UserDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type OrgDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Login verifies credentials and issues JWT token pair.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	user, err := h.store.Queries().GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	match, err := auth.ComparePassword(req.Password, user.PasswordHash)
	if err != nil || !match {
		respondError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// Fetch user's actual organization memberships
	memberships, err := h.store.Queries().ListUserMemberships(r.Context(), user.ID)
	if err != nil || len(memberships) == 0 {
		respondError(w, http.StatusForbidden, "No organization memberships found for user")
		return
	}

	selected := memberships[0]
	if req.OrgID != "" {
		for _, m := range memberships {
			if uuidToString(m.OrgID) == req.OrgID {
				selected = m
				break
			}
		}
	}

	tokens, err := h.jwtManager.GenerateTokenPair(user.ID, selected.OrgID, user.Email, selected.Role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to issue token")
		return
	}

	respondJSON(w, http.StatusOK, LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
		TokenType:    tokens.TokenType,
		User: UserDTO{
			ID:    uuidToString(user.ID),
			Email: user.Email,
			Name:  user.Name,
			Role:  selected.Role,
		},
		Org: OrgDTO{
			ID:   uuidToString(selected.OrgID),
			Name: selected.OrgName,
			Slug: selected.OrgSlug,
		},
	})
}

// Refresh handles token renewal using a valid refresh token.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		respondError(w, http.StatusBadRequest, "Valid refresh_token is required")
		return
	}

	claims, err := h.jwtManager.ValidateToken(req.RefreshToken)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	var userID, orgID pgtype.UUID
	_ = userID.Scan(claims.UserID)
	_ = orgID.Scan(claims.OrgID)

	tokens, err := h.jwtManager.GenerateTokenPair(userID, orgID, claims.Email, claims.Role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to issue refreshed token")
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// Me returns information about the currently authenticated session.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"user_id":   uuidToString(tc.UserID),
		"org_id":    uuidToString(tc.OrgID),
		"role":      tc.Role,
		"email":     tc.Email,
		"is_apikey": tc.IsAPIKey,
	})
}

// CreateAPIKey generates and registers an API key for the active organization.
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var req struct {
		Name      string `json:"name"`
		ExpiresIn string `json:"expires_in"` // e.g. "30d"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		respondError(w, http.StatusBadRequest, "Name is required for API key")
		return
	}

	generated, err := auth.GenerateAPIKey(req.Name)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	var expiresAt pgtype.Timestamptz
	if req.ExpiresIn != "" {
		if d, err := time.ParseDuration(req.ExpiresIn); err == nil {
			expiresAt = pgtype.Timestamptz{Time: time.Now().Add(d), Valid: true}
		}
	}

	apiKeyRow, err := h.store.Queries().CreateAPIKey(r.Context(), db.CreateAPIKeyParams{
		OrgID:     tc.OrgID,
		Name:      req.Name,
		KeyPrefix: generated.KeyPrefix,
		KeyHash:   generated.KeyHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to store API key")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]any{
		"id":         uuidToString(apiKeyRow.ID),
		"name":       apiKeyRow.Name,
		"key_prefix": apiKeyRow.KeyPrefix,
		"raw_key":    generated.RawKey, // Only returned once upon creation!
		"created_at": apiKeyRow.CreatedAt.Time.Format(time.RFC3339),
	})
}

// ListAPIKeys lists API keys registered for the organization.
func (h *AuthHandler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	keys, err := h.store.Queries().ListAPIKeys(r.Context(), tc.OrgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to retrieve API keys")
		return
	}

	respondJSON(w, http.StatusOK, keys)
}

// RevokeAPIKey revokes an API key.
func (h *AuthHandler) RevokeAPIKey(w http.ResponseWriter, r *http.Request) {
	tc, ok := middleware.GetTenant(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	keyIDStr := chi.URLParam(r, "id")
	var keyID pgtype.UUID
	if err := keyID.Scan(keyIDStr); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid key ID")
		return
	}

	revoked, err := h.store.Queries().RevokeAPIKey(r.Context(), db.RevokeAPIKeyParams{
		OrgID: tc.OrgID,
		ID:    keyID,
	})
	if err != nil {
		respondError(w, http.StatusNotFound, "API key not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":         uuidToString(revoked.ID),
		"revoked_at": revoked.RevokedAt.Time.Format(time.RFC3339),
	})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func uuidToString(u pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u.Bytes[0:4], u.Bytes[4:6], u.Bytes[6:8], u.Bytes[8:10], u.Bytes[10:16])
}
