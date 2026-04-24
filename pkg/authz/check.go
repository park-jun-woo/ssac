//ff:func feature=pkg-authz type=util control=sequence
//ff:what OPA 정책을 평가하여 인가를 검사한다 — owners 는 caller 가 주입
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
//
// req.Claim is passed through to OPA as the `claims` object. A nil Claim is
// normalized to an empty map so rego never observes `null`.
//
// req.Owners is the caller-populated ownership lookup. Typically the handler
// runs a yongol-generated `OwnerLookup<Resource>` sqlc query under the
// request's pgx.Tx before calling Check, then places the result into
// `req.Owners[resource][fmt.Sprint(resourceID)] = ownerID`. The owner-id
// value is typed as `any`, so its native Go type (int64 / uuid / string)
// is marshalled verbatim into rego input and compared directly against the
// matching claim.
func Check(req CheckRequest) (CheckResponse, error) {
	if os.Getenv("DISABLE_AUTHZ") == "1" {
		return CheckResponse{}, nil
	}
	if globalPolicy == "" {
		return CheckResponse{}, fmt.Errorf("authz not initialized")
	}

	// Nil-ctx fallback keeps legacy callers working while propagating request
	// cancellation when callers pass their handler ctx.
	ctx := req.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	owners := req.Owners
	if owners == nil {
		owners = map[string]map[string]any{}
	}

	var claims any = req.Claim
	if claims == nil {
		claims = map[string]any{}
	}
	opaInput := map[string]any{
		"claims":      claims,
		"action":      req.Action,
		"resource":    req.Resource,
		"resource_id": req.ResourceID,
	}

	// Build in-memory store with owners data for OPA evaluation. The owners
	// map is a nested `map[resource]map[resourceID]ownerID` so rego can
	// dereference `data.owners.<resource>[input.resource_id]`.
	store := inmem.NewFromObject(map[string]any{
		"owners": owners,
	})

	query, err := rego.New(
		rego.Query("data.authz.allow"),
		rego.Module("policy.rego", globalPolicy),
		rego.Store(store),
		rego.Input(opaInput),
	).Eval(ctx)
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
