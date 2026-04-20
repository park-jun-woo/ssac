//ff:func feature=pkg-authz type=util control=iteration dimension=1
//ff:what DB에서 리소스 소유권 정보를 조회한다
package authz

import (
	"context"
	"database/sql"
	"fmt"
)

// ownerQuerier is the minimal interface satisfied by both *sql.DB and *sql.Tx.
// It lets loadOwners switch between globalDB and a request-scoped tx without
// branching in the row-fetch loop.
type ownerQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// loadOwners queries DB for ownership data based on registered mappings.
// The caller-supplied ctx is propagated into QueryRowContext so that request
// cancellation reaches the DB driver.
//
// When req.Tx is non-nil, ownership reads run inside that transaction so
// rows created earlier in the same handler are visible (MVCC snapshot
// consistency). Otherwise globalDB is used — backward compatible for
// read-only handlers that do not open a tx.
func loadOwners(ctx context.Context, req CheckRequest) (map[string]interface{}, error) {
	owners := make(map[string]interface{})

	var q ownerQuerier
	if req.Tx != nil {
		q = req.Tx
	} else if globalDB != nil {
		q = globalDB
	} else {
		return owners, nil
	}
	if len(globalOwnerships) == 0 {
		return owners, nil
	}

	for _, om := range globalOwnerships {
		var ownerID int64
		query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", om.Column, om.Table)
		err := q.QueryRowContext(ctx, query, req.ResourceID).Scan(&ownerID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("query %s.%s: %w", om.Table, om.Column, err)
		}

		resMap, ok := owners[om.Resource].(map[string]interface{})
		if !ok {
			resMap = make(map[string]interface{})
			owners[om.Resource] = resMap
		}
		resMap[fmt.Sprint(req.ResourceID)] = ownerID
	}

	return owners, nil
}
