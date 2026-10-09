package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/seraph/triage/apps/api/internal/auth"
	"github.com/seraph/triage/apps/api/internal/store"
)

type tenantCtxKey struct{}

// TenantContext holds the authenticated identity and organization context for the request.
type TenantContext struct {
	UserID    pgtype.UUID
	OrgID     pgtype.UUID
	Role      string
	Email     string
	IsAPIKey  bool
	APIKeyID  pgtype.UUID
	KeyPrefix string
}

// GetTenant retrieves the TenantContext from the request context.
func GetTenant(ctx context.Context) (TenantContext, bool) {
	val, ok := ctx.Value(tenantCtxKey{}).(TenantContext)
	return val, ok
}

// SetTenantContext injects TenantContext into a context.
func SetTenantContext(ctx context.Context, tc TenantContext) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tc)
}

// Authenticator validates Bearer JWT tokens or X-API-Key headers.
type Authenticator struct {
	jwtManager *auth.JWTManager
	store      *store.Store
}

// NewAuthenticator creates a new Authenticator.
func NewAuthenticator(jwtManager *auth.JWTManager, st *store.Store) *Authenticator {
	return &Authenticator{
		jwtManager: jwtManager,
		store:      st,
	}
}

// AuthenticateMiddleware extracts and verifies credentials, populating TenantContext.
func (a *Authenticator) AuthenticateMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check for API Key first (X-API-Key header, query param, or Authorization: Bearer tr_live_...)
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		authHeader := r.Header.Get("Authorization")
		if apiKey == "" && strings.HasPrefix(authHeader, "Bearer tr_live_") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if apiKey != "" && a.store != nil {
			keyHash := auth.HashAPIKey(apiKey)
			dbKey, err := a.store.Queries().GetAPIKeyByHash(r.Context(), keyHash)
			if err == nil {
				tc := TenantContext{
					OrgID:     dbKey.OrgID,
					Role:      auth.RoleAdmin, // API Keys granted admin permissions within their org
					IsAPIKey:  true,
					APIKeyID:  dbKey.ID,
					KeyPrefix: dbKey.KeyPrefix,
				}
				ctx := SetTenantContext(r.Context(), tc)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// 2. Check for JWT Bearer token (Authorization header or query param ?token=...)
		tokenStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenCandidate := strings.TrimPrefix(authHeader, "Bearer ")
			if !strings.HasPrefix(tokenCandidate, "tr_live_") {
				tokenStr = tokenCandidate
			}
		} else if qToken := r.URL.Query().Get("token"); qToken != "" {
			if strings.HasPrefix(qToken, "tr_live_") && a.store != nil {
				// Query token was actually an API key
				keyHash := auth.HashAPIKey(qToken)
				if dbKey, err := a.store.Queries().GetAPIKeyByHash(r.Context(), keyHash); err == nil {
					tc := TenantContext{
						OrgID:     dbKey.OrgID,
						Role:      auth.RoleAdmin,
						IsAPIKey:  true,
						APIKeyID:  dbKey.ID,
						KeyPrefix: dbKey.KeyPrefix,
					}
					ctx := SetTenantContext(r.Context(), tc)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			} else {
				tokenStr = qToken
			}
		}

		if tokenStr != "" && a.jwtManager != nil {
			claims, err := a.jwtManager.ValidateToken(tokenStr)
			if err == nil {
				var userID, orgID pgtype.UUID
				_ = userID.Scan(claims.UserID)
				_ = orgID.Scan(claims.OrgID)

				tc := TenantContext{
					UserID:   userID,
					OrgID:    orgID,
					Role:     claims.Role,
					Email:    claims.Email,
					IsAPIKey: false,
				}
				ctx := SetTenantContext(r.Context(), tc)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Optional / unauthenticated request passes through; RequireAuth blocks if needed
		next.ServeHTTP(w, r)
	})
}

// RequireAuth middleware ensures the request has valid tenant authentication.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := GetTenant(r.Context())
		if !ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required. Provide a valid Bearer token or X-API-Key.",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole middleware ensures the authenticated user has at least the specified RBAC role.
func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tc, ok := GetTenant(r.Context())
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
				return
			}

			if !auth.HasMinimumRole(tc.Role, minRole) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "Insufficient permissions for this operation",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
