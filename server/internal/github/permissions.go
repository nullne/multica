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
//
// Note: pull_requests defaults to read here. Only the admin tier upgrades
// it to write in PermissionsForCodeAccess. GitHub bundles PR
// create/update/merge/close under a single "write" permission, so giving
// the write tier pull_requests=write would also let it merge — see
// PermissionsForCodeAccess for the full matrix.
func basePermissions() map[string]string {
	return map[string]string{
		"issues":        PermWrite,
		"pull_requests": PermRead,
		"checks":        PermRead,
		"statuses":      PermRead,
		"metadata":      PermRead,
	}
}

// PermissionsForCodeAccess returns the full GitHub installation token
// permission set for the given code access level.
//
// Matrix:
//
//	level   contents  pull_requests  issues  checks  statuses  metadata
//	read    read      read           write   read    read      read
//	write   write     read           write   read    read      read
//	admin   write     write          write   read    read      read
//
// pull_requests=write covers PR create / update / merge / close at the
// GitHub API level; it is intentionally restricted to admin so that
// read/write agents cannot merge or close PRs even by calling the API
// directly with their installation token.
func PermissionsForCodeAccess(level string) map[string]string {
	perms := basePermissions()
	switch level {
	case CodeAccessRead:
		perms["contents"] = PermRead
	case CodeAccessWrite:
		perms["contents"] = PermWrite
	case CodeAccessAdmin:
		perms["contents"] = PermWrite
		perms["pull_requests"] = PermWrite
	default:
		perms["contents"] = PermRead
	}
	return perms
}

// MergeAllowed returns whether the given code access level permits
// merging pull requests. Only "admin" level allows merging.
//
// This is a defense-in-depth check for any future server-side merge proxy
// flows. The primary enforcement is the GitHub installation token scope —
// see PermissionsForCodeAccess.
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
