//ff:func feature=pkg-authz type=util control=iteration dimension=1
//ff:what DB에서 리소스 소유권 정보를 조회한다
package authz

import (
	"database/sql"
	"fmt"
)

// loadOwners queries DB for ownership data based on registered mappings.
func loadOwners(req CheckRequest) (map[string]interface{}, error) {
	owners := make(map[string]interface{})
	if globalDB == nil || len(globalOwnerships) == 0 {
		return owners, nil
	}

	for _, om := range globalOwnerships {
		var ownerID int64
		query := fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", om.Column, om.Table)
		err := globalDB.QueryRow(query, req.ResourceID).Scan(&ownerID)
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
