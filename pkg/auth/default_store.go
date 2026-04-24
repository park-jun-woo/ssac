//ff:func feature=pkg-auth type=util control=sequence topic=auth-refresh
//ff:what @call Func용 기본 refresh store — Init으로 주입
package auth

import "sync"

// defaultStore is the package-level RefreshStore used by RefreshRotate /
// Logout when the caller does not pass one explicitly. Mirrors the singleton
// pattern used by cache.Init / session.Init so every DB-using ssac package
// is wired identically:
//
//	auth.Init(<yongol-generated RefreshStore>)
//	cache.Init(<yongol-generated CacheModel>)
//	session.Init(<yongol-generated SessionModel>)
//	queue.SetBackend(<yongol-generated Backend>)
//
// Callers that still wish to pass a store per call (tests, multiple
// simultaneous stores, etc.) may do so — the explicit argument takes
// precedence over the singleton.
var (
	storeMu      sync.RWMutex
	defaultStore RefreshStore
)

// Init sets the package-level RefreshStore used by RefreshRotate and Logout
// when those helpers are called with a nil store. Called once at boot by
// yongol-generated main.go after constructing the user-sqlc-backed
// RefreshStore implementation.
func Init(store RefreshStore) {
	storeMu.Lock()
	defaultStore = store
	storeMu.Unlock()
}

// currentStore returns a snapshot of the package-level RefreshStore under a
// read-lock. Internal helper shared by RefreshRotate / Logout.
func currentStore() RefreshStore {
	storeMu.RLock()
	defer storeMu.RUnlock()
	return defaultStore
}
