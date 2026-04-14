//ff:type feature=pkg-pagination type=model
//ff:what 커서 기반 페이지네이션 응답 래퍼
package pagination

// Cursor is a generic wrapper for cursor-based paginated responses.
type Cursor[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor"`
	HasNext    bool   `json:"has_next"`
}
