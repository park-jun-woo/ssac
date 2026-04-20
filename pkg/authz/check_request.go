//ff:type feature=pkg-authz type=model
//ff:what 인가 검사 요청 구조체
package authz

import (
	"context"
	"database/sql"
)

// CheckRequest holds the inputs for an authorization check.
//
// Claim is passed through to OPA input as the `claims` object. Callers should
// use a struct with JSON tags whose keys match the claim names expected by
// their rego policy (e.g. `json:"user_id"`, `json:"org_id"`). The concrete
// type is opaque to this package — rego.Input marshals it via json.Marshal.
//
// Ctx carries the request context used for OPA evaluation and the ownership
// DB query. When nil, Check falls back to context.Background() for backward
// compatibility. New callers should always propagate the handler's ctx.
//
// Tx is an optional *sql.Tx used for ownership lookups. When non-nil,
// loadOwners executes its SELECT against this transaction so that rows
// inserted/updated earlier in the same handler are visible (MVCC snapshot
// consistency). When nil, loadOwners falls back to the globalDB injected via
// Init — backward compatible for read-only handlers.
type CheckRequest struct {
	Ctx        context.Context
	Tx         *sql.Tx
	Action     string
	Resource   string
	ResourceID int64
	Claim      any
}
