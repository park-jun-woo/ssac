# pkg/pagination

`Page[T]` / `Cursor[T]` 제네릭 페이지네이션 응답 래퍼.

## 개요

SSaC 코드젠이 리스트 조회 모델 메서드의 반환 타입으로 사용하는 제네릭 래퍼. `@get Page[Course] list = Course.List({...})` 같은 시퀀스에서 `*pagination.Page[Course]` 또는 `*pagination.Cursor[Course]` 타입이 생성 코드에 삽입된다. OpenAPI의 `x-pagination: offset` → `Page[T]`, `x-pagination: cursor` → `Cursor[T]`와 교차 검증되며 불일치 시 validator ERROR를 낸다. `@response list` 간단쓰기 응답 시 OpenAPI response schema의 고정 필드(items/total, items/next_cursor/has_next)와 필드명·JSON 태그가 매칭되어야 한다.

## 공개 API

### `Page[T any]`

offset 기반 페이지네이션 응답. 전체 개수(`total`)를 포함하므로 DB에서 `COUNT(*)` 쿼리가 필요하다.

| 필드 | 타입 | JSON | 설명 |
|---|---|---|---|
| Items | []T | `items` | 현재 페이지 항목 배열 |
| Total | int64 | `total` | 전체 레코드 수 |

### `Cursor[T any]`

커서 기반 페이지네이션 응답. 다음 커서와 추가 페이지 유무만 전달한다(total 없음).

| 필드 | 타입 | JSON | 설명 |
|---|---|---|---|
| Items | []T | `items` | 현재 페이지 항목 배열 |
| NextCursor | string | `next_cursor` | 다음 페이지 요청 시 전달할 커서 토큰 |
| HasNext | bool | `has_next` | 다음 페이지 존재 여부 |

## 사용 예시

### SSaC 시퀀스 (offset)

```go
// @get Page[Course] list = Course.List({query: query})
// @response list
```

생성 코드:

```go
func ListCourses(c *gin.Context) {
    opts := parseQueryOpts(c)
    list, err := courseModel.List(c.Request.Context(), opts)
    if err != nil { /* 500 */ }
    c.JSON(200, list)  // *pagination.Page[Course] → {"items":[...], "total":N}
}
```

모델 인터페이스 시그니처:

```go
type CourseModel interface {
    List(ctx context.Context, opts QueryOpts) (*pagination.Page[Course], error)
}
```

### SSaC 시퀀스 (cursor)

```go
// @get Cursor[Post] feed = Post.Feed({query: query})
// @response feed
```

모델 시그니처:

```go
type PostModel interface {
    Feed(ctx context.Context, opts QueryOpts) (*pagination.Cursor[Post], error)
}
```

### Go 직접 호출

```go
import "github.com/park-jun-woo/ssac/pkg/pagination"

page := &pagination.Page[Course]{
    Items: courses,
    Total: total,
}

cursor := &pagination.Cursor[Post]{
    Items:      posts,
    NextCursor: "abc123",
    HasNext:    true,
}
```

### OpenAPI 교차 검증

```yaml
paths:
  /courses:
    get:
      operationId: listCourses
      x-pagination: offset   # → Page[T] 요구
      responses:
        '200':
          content:
            application/json:
              schema:
                type: object
                properties:
                  items: { type: array, items: { $ref: '#/components/schemas/Course' } }
                  total: { type: integer }
```

`x-pagination: offset`인데 SSaC가 `Cursor[T]`를 쓰면 validator ERROR.
