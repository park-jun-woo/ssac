//ff:func feature=pkg-auth type=test control=sequence topic=auth-refresh
//ff:what sql.ErrNoRows 를 재노출해 sqlmock 테스트의 database/sql 의존을 격리한다
package auth

import "database/sql"

var errSQLNoRows = sql.ErrNoRows
