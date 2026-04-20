//ff:type feature=pkg-auth type=data
//ff:what DummyHash — 타이밍 공격 방어용 bcrypt 더미 해시 상수
package auth

// DummyHash is a pre-computed bcrypt hash used to equalise response time on
// the "user not found" path of login-style flows, preventing timing-based
// email enumeration.
//
// When a user lookup misses, handlers should still call VerifyPassword
// against DummyHash with the attacker-supplied password so both branches
// pay the ~60ms bcrypt cost. The hash never matches any real password
// (its plaintext is an unreachable sentinel the caller never submits),
// so its only side effect is wall-clock symmetry.
//
// Source: bcrypt.GenerateFromPassword([]byte("dummy"), bcrypt.DefaultCost).
// Callers must NOT change the cost factor without regenerating both sides
// of the comparison to keep timings matched.
const DummyHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
