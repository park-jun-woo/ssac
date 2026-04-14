//ff:func feature=pkg-authz type=loader control=sequence
//ff:what 글로벌 인가 상태를 초기화한다
package authz

import (
	"database/sql"
	"fmt"
	"os"
)

var globalPolicy string
var globalDB *sql.DB
var globalOwnerships []OwnershipMapping

// Init initializes the global authz state.
// Reads OPA policy from OPA_POLICY_PATH environment variable.
// Skips initialization when DISABLE_AUTHZ=1.
func Init(db *sql.DB, ownerships []OwnershipMapping) error {
	globalDB = db
	globalOwnerships = ownerships

	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return nil
	}

	policyPath := os.Getenv("OPA_POLICY_PATH")
	if policyPath == "" {
		return fmt.Errorf("OPA_POLICY_PATH environment variable is required (set DISABLE_AUTHZ=1 to skip)")
	}

	policyData, err := os.ReadFile(policyPath)
	if err != nil {
		return fmt.Errorf("read OPA policy %s: %w", policyPath, err)
	}

	globalPolicy = string(policyData)
	return nil
}
