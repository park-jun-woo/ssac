//ff:type feature=pkg-pagination type=model
//ff:what 오프셋 기반 페이지네이션 응답 래퍼
package pagination

// Page is a generic wrapper for offset-based paginated responses.
type Page[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}
