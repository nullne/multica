package github

// GitHub App permission values.
const (
	PermRead  = "read"
	PermWrite = "write"
)

// Code access levels stored on the agent row.
const (
	CodeAccessRead  = "read"
	CodeAccessWrite = "write"
	CodeAccessAdmin = "admin"
)

// basePermissions returns the permissions every agent token receives
// regardless of code access level.
func basePermissions() map[string]string {
	return map[string]string{
		"issues":        PermWrite,
		"pull_requests": PermWrite,
		"checks":        PermRead,
		"statuses":      PermRead,
		"metadata":      PermRead,
	}
}

// PermissionsForCodeAccess returns the full GitHub installation token
// permission set for the given code access level.
func PermissionsForCodeAccess(level string) map[string]string {
	perms := basePermissions()
	switch level {
	case CodeAccessRead:
		perms["contents"] = PermRead
	case CodeAccessWrite, CodeAccessAdmin:
		perms["contents"] = PermWrite
	default:
		perms["contents"] = PermRead
	}
	return perms
}

// MergeAllowed returns whether the given code access level permits
// merging pull requests. Only "admin" level allows merging.
func MergeAllowed(level string) bool {
	return level == CodeAccessAdmin
}

// ValidCodeAccess returns true if the level is a recognized value.
func ValidCodeAccess(level string) bool {
	switch level {
	case CodeAccessRead, CodeAccessWrite, CodeAccessAdmin:
		return true
	}
	return false
}
