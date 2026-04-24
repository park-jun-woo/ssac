//ff:type feature=pkg-authz type=model
//ff:what 인가 검사 요청 구조체 — caller 가 Owners 를 사전 로드해 주입
package authz

import "context"

// CheckRequest holds the inputs for an authorization check.
//
// Claim is passed through to OPA input as the `claims` object. Callers should
// use a struct with JSON tags whose keys match the claim names expected by
// the rego policy (e.g. `json:"user_id"`, `json:"org_id"`). The concrete type
// is opaque to this package — rego.Input marshals it via json.Marshal.
//
// Ctx carries the request context used for OPA evaluation. When nil, Check
// falls back to context.Background(). New callers should always propagate the
// handler's ctx.
//
// Owners is the caller-loaded ownership lookup table. Keys are resource
// names (matching the `@ownership <resource>:` declarations in the Rego
// policy) and values are maps from resource-id to owner-id. Outer and inner
// keys are strings because OPA's in-memory store and JSON object keys are
// string-only, so callers stringify the resource-id when populating the
// inner map (typically `fmt.Sprint(id)`). The owner-id value is typed as
// `any` so its natural Go type is marshalled verbatim into rego input —
// an `int64` owner column stays a JSON number, a `uuid` column stays a
// JSON string, and rego policies compare it directly against the matching
// claim (`data.owners.<resource>[id] == input.claims.user_id`) without an
// extra stringification layer.
//
// Since ssac does not touch the database, all DB-dependent fields
// (`*sql.Tx`, `*sql.DB`) have been removed; the caller is the single
// source of authority for DB access.
type CheckRequest struct {
	Ctx        context.Context
	Action     string
	Resource   string
	ResourceID int64
	Claim      any
	Owners     map[string]map[string]any
}
