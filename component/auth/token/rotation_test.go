package apptoken

import (
	"testing"
	"time"
)

// TestKeyRotationOldTokenStillValid 验证轮换后旧密钥签发的 token 在宽限期内仍可验证。
func TestKeyRotationOldTokenStillValid(t *testing.T) {
	oldSecret := "0123456789abcdef0123456789abcdef"
	newSecret := "fedcba9876543210fedcba9876543210"

	if err := SetSecretKey(oldSecret); err != nil {
		t.Fatalf("set old secret failed: %v", err)
	}
	oldAccess, _, err := GenToken(1, "old@example.com")
	if err != nil {
		t.Fatalf("generate with old secret failed: %v", err)
	}

	// 轮换：新密钥签名，旧密钥仍可验证
	if err := SetSecretKeys(newSecret, oldSecret); err != nil {
		t.Fatalf("rotate secrets failed: %v", err)
	}
	newAccess, _, err := GenToken(2, "new@example.com")
	if err != nil {
		t.Fatalf("generate with new secret failed: %v", err)
	}

	// 两个 token 都应可验证
	oldClaim, err := VerifyToken(oldAccess)
	if err != nil {
		t.Fatalf("old token must verify during rotation grace period: %v", err)
	}
	if oldClaim.UserID != 1 {
		t.Fatalf("unexpected old claim: %+v", oldClaim)
	}
	newClaim, err := VerifyToken(newAccess)
	if err != nil {
		t.Fatalf("new token must verify: %v", err)
	}
	if newClaim.UserID != 2 {
		t.Fatalf("unexpected new claim: %+v", newClaim)
	}
}

// TestKeyRotationRevokedOldSecret 验证轮换结束后旧密钥失效。
func TestKeyRotationRevokedOldSecret(t *testing.T) {
	oldSecret := "0123456789abcdef0123456789abcdef"
	newSecret := "fedcba9876543210fedcba9876543210"

	if err := SetSecretKey(oldSecret); err != nil {
		t.Fatalf("set old secret failed: %v", err)
	}
	oldAccess, _, err := GenToken(1, "old@example.com")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	// 完全切换到新密钥（旧密钥从验证列表移除）
	if err := SetSecretKeys(newSecret); err != nil {
		t.Fatalf("switch secrets failed: %v", err)
	}
	if _, err := VerifyToken(oldAccess); err == nil {
		t.Fatal("revoked old token must be rejected")
	}
}

// TestSetSecretKeysValidation 验证密钥列表校验。
func TestSetSecretKeysValidation(t *testing.T) {
	if err := SetSecretKeys(); err == nil {
		t.Fatal("empty secret list must return an error")
	}
	if err := SetSecretKeys("short"); err == nil {
		t.Fatal("short secret must be rejected")
	}
}

// TestRefreshWithRotatedSecret 验证轮换后 refresh token 仍可刷新。
func TestRefreshWithRotatedSecret(t *testing.T) {
	oldSecret := "0123456789abcdef0123456789abcdef"
	newSecret := "fedcba9876543210fedcba9876543210"

	if err := SetSecretKey(oldSecret); err != nil {
		t.Fatalf("set old secret failed: %v", err)
	}
	access, refresh, err := GenToken(7, "user@example.com")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	// 让 access token 过期以便走刷新路径
	aTokenExpiredDuration = -1
	expiredAccess, _, err := GenToken(7, "user@example.com")
	if err != nil {
		t.Fatalf("generate expired token failed: %v", err)
	}
	aTokenExpiredDuration = 30 * time.Minute

	// 轮换后刷新
	if err := SetSecretKeys(newSecret, oldSecret); err != nil {
		t.Fatalf("rotate failed: %v", err)
	}
	newAccess, _, err := RefreshToken(expiredAccess, refresh)
	if err != nil {
		t.Fatalf("refresh with rotated secret failed: %v", err)
	}
	if newAccess == "" {
		t.Fatal("refreshed token must not be empty")
	}
	// 未过期的旧 access 也支持直接刷新
	_, _, err = RefreshToken(access, refresh)
	if err != nil {
		t.Fatalf("refresh valid access failed: %v", err)
	}
}
