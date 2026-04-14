//ff:type feature=pkg-authz type=model
//ff:what 리소스-테이블 소유권 매핑 구조체
package authz

// OwnershipMapping represents a resource-to-table ownership mapping from @ownership annotations.
type OwnershipMapping struct {
	Resource string // "gig", "proposal"
	Table    string // "gigs", "proposals"
	Column   string // "client_id", "freelancer_id"
}
