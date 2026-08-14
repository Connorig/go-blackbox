package apptoken

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// TestGenTokenRequiresSecret 验证未注入密钥时签发会返回明确错误，拒绝使用弱默认密钥。
func TestGenTokenRequiresSecret(t *testing.T) {
	previous := secretKeys
	secretKeys = nil
	t.Cleanup(func() { secretKeys = previous })

	if _, _, err := GenToken(1, "user@example.com"); err != ErrSecretNotConfigured {
		t.Fatalf("expected ErrSecretNotConfigured, got: %v", err)
	}
}

// TestSetSecretKeyRejectsShortSecret 验证过短的密钥会被拒绝。
func TestSetSecretKeyRejectsShortSecret(t *testing.T) {
	if err := SetSecretKey("short"); err == nil {
		t.Fatal("short secret must be rejected")
	}
}

// TestGenAndVerifyRoundTrip 验证设置密钥后签发与验证成功，且声明字段正确。
func TestGenAndVerifyRoundTrip(t *testing.T) {
	if err := SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	const userID = int64(42)
	const userEmail = "user@example.com"
	accessToken, refreshToken, err := GenToken(userID, userEmail)
	if err != nil {
		t.Fatalf("generate tokens failed: %v", err)
	}

	claim, err := VerifyToken(accessToken)
	if err != nil {
		t.Fatalf("verify access token failed: %v", err)
	}
	if claim.UserID != userID || claim.UserEmail != userEmail {
		t.Fatalf("unexpected claims: %+v", claim)
	}
	if _, err := jwt.Parse(refreshToken, keyFuncFor(secretKeys[0])); err != nil {
		t.Fatalf("refresh token must be parseable: %v", err)
	}
}

// TestVerifyRejectsNonHS256Algorithm 验证算法白名单生效（防算法混淆攻击）。
func TestVerifyRejectsNonHS256Algorithm(t *testing.T) {
	if err := SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	// 使用 HS384 签发，验证时必须被拒绝
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS384, MyClaim{
		UserID:    1,
		UserEmail: "attacker@example.com",
	}).SignedString([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("sign forged token failed: %v", err)
	}
	if _, verifyErr := VerifyToken(forged); verifyErr == nil {
		t.Fatal("token signed with non-whitelisted algorithm must be rejected")
	} else if !strings.Contains(verifyErr.Error(), "signing method") && !strings.Contains(verifyErr.Error(), "not allowed") {
		t.Fatalf("unexpected algorithm rejection error: %v", verifyErr)
	}
}

// TestVerifyRejectsWrongIssuer 验证签发者不匹配的 token 会被拒绝。
func TestVerifyRejectsWrongIssuer(t *testing.T) {
	if err := SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	claim := MyClaim{
		UserID:    1,
		UserEmail: "user@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
			Issuer:    "evil-issuer",
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claim).SignedString(secretKeys[0])
	if err != nil {
		t.Fatalf("sign token failed: %v", err)
	}
	if _, err := VerifyToken(token); err == nil {
		t.Fatal("token with wrong issuer must be rejected")
	}
}

// TestRefreshTokenWithExpiredAccess 验证 access token 正常过期后可用 refresh token 刷新。
func TestRefreshTokenWithExpiredAccess(t *testing.T) {
	if err := SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	aTokenExpiredDuration = -time.Minute
	accessToken, refreshToken, err := GenToken(7, "user@example.com")
	if err != nil {
		t.Fatalf("generate tokens failed: %v", err)
	}
	aTokenExpiredDuration = 30 * time.Minute

	newAccess, newRefresh, err := RefreshToken(accessToken, refreshToken)
	if err != nil {
		t.Fatalf("refresh expired access token failed: %v", err)
	}
	if newAccess == "" || newRefresh == "" {
		t.Fatal("refresh must return both tokens")
	}
	claim, err := VerifyToken(newAccess)
	if err != nil {
		t.Fatalf("verify refreshed access token failed: %v", err)
	}
	if claim.UserID != 7 {
		t.Fatalf("unexpected refreshed user id: %d", claim.UserID)
	}
}

// TestRefreshTokenRejectsInvalidRefresh 验证无效 refresh token 会返回错误而不是双 nil。
func TestRefreshTokenRejectsInvalidRefresh(t *testing.T) {
	if err := SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret key failed: %v", err)
	}
	newAccess, newRefresh, err := RefreshToken("invalid-access", "invalid-refresh")
	if err == nil {
		t.Fatal("refresh with invalid tokens must return an error")
	}
	if newAccess != "" || newRefresh != "" {
		t.Fatal("failed refresh must not return tokens")
	}
}
