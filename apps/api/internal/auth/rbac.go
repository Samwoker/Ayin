package auth

// Role constants
const (
	RoleOwner     = "owner"
	RoleAdmin     = "admin"
	RoleResponder = "responder"
	RoleViewer    = "viewer"
)

// RoleWeights define hierarchy level
var roleHierarchy = map[string]int{
	RoleOwner:     4,
	RoleAdmin:     3,
	RoleResponder: 2,
	RoleViewer:    1,
}

// HasMinimumRole checks if a user's role satisfies the required minimum role level.
func HasMinimumRole(userRole, requiredRole string) bool {
	userWeight, userExists := roleHierarchy[userRole]
	reqWeight, reqExists := roleHierarchy[requiredRole]
	if !userExists || !reqExists {
		return false
	}
	return userWeight >= reqWeight
}

// IsValidRole validates if a role string is known.
func IsValidRole(role string) bool {
	_, ok := roleHierarchy[role]
	return ok
}
