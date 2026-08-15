package apptoken

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// setupOrgSecret 注入测试密钥并清理轮换状态。
func setupOrgSecret(t *testing.T) {
	t.Helper()
	if err := SetSecretKeys(strings.Repeat("a", 32), strings.Repeat("b", 32)); err != nil {
		t.Fatalf("set secret failed: %v", err)
	}
	t.Cleanup(func() { secretKeys = nil })
}

// signLegacy 手工构造升级前格式(无 orgId/deptId 字段)的老 token。
func signLegacy(t *testing.T, key []byte) (string, error) {
	t.Helper()
	legacy := struct {
		jwt.RegisteredClaims
		UserID    int64  `json:"userId"`
		UserEmail string `json:"userEmail"`
		Scope     string `json:"scope,omitempty"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    tokenIssuer,
		},
		UserID:    5,
		UserEmail: "old@example.com",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString(key)
}

// TestGenTokenFullRoundTrip 组织身份完整往返。
func TestGenTokenFullRoundTrip(t *testing.T) {
	setupOrgSecret(t)
	access, refresh, err := GenTokenFull(7, "dev@example.com", "order:read,order:write", 101, 202)
	if err != nil {
		t.Fatalf("GenTokenFull failed: %v", err)
	}
	claim, err := VerifyToken(access)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if claim.UserID != 7 || claim.UserEmail != "dev@example.com" {
		t.Fatalf("identity mismatch: %+v", claim)
	}
	if claim.Scope != "order:read,order:write" {
		t.Fatalf("scope mismatch: %q", claim.Scope)
	}
	if claim.OrgID != 101 || claim.DeptID != 202 {
		t.Fatalf("org identity mismatch: org=%d dept=%d", claim.OrgID, claim.DeptID)
	}
	// refresh token 可用且保留组织身份
	newAccess, _, err := RefreshToken(access, refresh)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	newClaim, err := VerifyToken(newAccess)
	if err != nil {
		t.Fatalf("verify refreshed token failed: %v", err)
	}
	if newClaim.OrgID != 101 || newClaim.DeptID != 202 {
		t.Fatalf("refresh must keep org identity: org=%d dept=%d", newClaim.OrgID, newClaim.DeptID)
	}
}

// TestGenTokenWithScopeLegacy 旧签发入口保持行为(组织身份为零值)。
func TestGenTokenWithScopeLegacy(t *testing.T) {
	setupOrgSecret(t)
	access, _, err := GenTokenWithScope(1, "a@b.com", "user:read")
	if err != nil {
		t.Fatalf("GenTokenWithScope failed: %v", err)
	}
	claim, err := VerifyToken(access)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if claim.OrgID != 0 || claim.DeptID != 0 {
		t.Fatalf("legacy token must have zero org identity, got %+v", claim)
	}
}

// TestLegacyTokenWithoutOrgFields 老 token(无 orgId 字段)解析为零值,兼容。
func TestLegacyTokenWithoutOrgFields(t *testing.T) {
	setupOrgSecret(t)
	// 手工构造不含 orgId/deptId 的 token(模拟升级前签发的老 token)
	key, err := signingKey()
	if err != nil {
		t.Fatalf("signing key failed: %v", err)
	}
	tokenString, err := signLegacy(t, key)
	if err != nil {
		t.Fatalf("sign legacy token failed: %v", err)
	}
	claim, err := VerifyToken(tokenString)
	if err != nil {
		t.Fatalf("legacy token must still verify: %v", err)
	}
	if claim.OrgID != 0 || claim.DeptID != 0 {
		t.Fatalf("legacy token must parse to zero org identity, got %+v", claim)
	}
}
