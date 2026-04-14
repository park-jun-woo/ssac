//ff:func feature=pkg-authz type=util control=sequence
//ff:what OPA 정책을 평가하여 인가를 검사한다
package authz

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

// Check evaluates the OPA policy. Returns error if denied or evaluation fails.
// Set DISABLE_AUTHZ=1 to bypass authorization checks.
func Check(req CheckRequest) (CheckResponse, error) {
	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return CheckResponse{}, nil
	}
	if globalPolicy == "" {
		return CheckResponse{}, fmt.Errorf("authz not initialized")
	}

	// Build data.owners by querying DB for matching ownership mappings.
	owners, err := loadOwners(req)
	if err != nil {
		return CheckResponse{}, fmt.Errorf("load owners: %w", err)
	}

	opaInput := map[string]interface{}{
		"claims":      map[string]interface{}{"user_id": req.UserID, "role": req.Role},
		"action":      req.Action,
		"resource":    req.Resource,
		"resource_id": req.ResourceID,
	}

	// Build in-memory store with owners data for OPA evaluation.
	store := inmem.NewFromObject(map[string]interface{}{
		"owners": owners,
	})

	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("policy.rego", globalPolicy),
		rego.Store(store),
		rego.Input(opaInput),
	).Eval(context.Background())
	if err != nil {
		return CheckResponse{}, fmt.Errorf("OPA eval failed: %w", err)
	}
	if len(query) == 0 {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	allowed, ok := query[0].Expressions[0].Value.(bool)
	if !ok || !allowed {
		return CheckResponse{}, fmt.Errorf("forbidden")
	}
	return CheckResponse{}, nil
}
