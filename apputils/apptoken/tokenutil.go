package apptoken

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var (
	aTokenExpiredDuration = 30 * time.Minute   //默认30分钟
	rTokenExpiredDuration = 7 * 24 * time.Hour // 默认7天
	tokenIssuer           = "Homelander"
	// secretKeys 保存 HS256 验证密钥列表；列表首个为当前签名密钥。
	// 轮换期间旧密钥保留在列表中，新签发的 token 使用最新密钥。
	secretKeys [][]byte
)

var (
	ErrorInvalidToken = errors.New("verify Token Failed")
	// ErrSecretNotConfigured 表示 JWT 签名密钥尚未注入，签发与验证都会拒绝执行。
	ErrSecretNotConfigured = errors.New("JWT secret key is not configured, call SetSecretKey first")
	// ErrUnsupportedSigningMethod 表示 token 使用了签名算法白名单之外的算法。
	ErrUnsupportedSigningMethod = errors.New("JWT signing method is not allowed")
	// ErrEmptySecret 表示注入的密钥列表为空或包含空密钥。
	ErrEmptySecret = errors.New("JWT secret list is empty")
)

// Init 初始化设置token-过期时间、重新刷新时间、token签名
func Init(AMinute, RHour time.Duration, TokenIssuer string) {

	if AMinute > 0 {
		aTokenExpiredDuration = AMinute
	}
	if RHour > 0 {
		rTokenExpiredDuration = RHour
	}
	if TokenIssuer != "" {
		tokenIssuer = TokenIssuer
	}
}

// SetSecretKey 注入单个 JWT 签名密钥（同时作为验证密钥）。
// 出于安全考虑，密钥长度必须至少 32 字节；未注入密钥时签发与验证都会返回错误。
func SetSecretKey(secret string) error {
	return SetSecretKeys(secret)
}

// SetSecretKeys 注入 JWT 密钥列表，支持密钥轮换：
// 列表首个密钥用于签发新 token；全部密钥均可用于验证（轮换宽限期）。
// 每个密钥长度必须至少 32 字节。
func SetSecretKeys(secrets ...string) error {
	if len(secrets) == 0 {
		return ErrEmptySecret
	}
	keys := make([][]byte, 0, len(secrets))
	for _, secret := range secrets {
		if len(secret) < 32 {
			return fmt.Errorf("JWT secret key must be at least 32 bytes, got %d", len(secret))
		}
		keys = append(keys, []byte(secret))
	}
	secretKeys = keys
	return nil
}

// MyClaim 是 Access Token 携带的业务声明。
type MyClaim struct {
	UserID    int64  `json:"userId"`
	UserEmail string `json:"userEmail"`
	jwt.RegisteredClaims
}

func getJWTTime(t time.Duration) *jwt.NumericDate {
	return jwt.NewNumericDate(time.Now().Add(t))
}

// signingKey 返回当前签名密钥。
func signingKey() ([]byte, error) {
	if len(secretKeys) == 0 {
		return nil, ErrSecretNotConfigured
	}
	return secretKeys[0], nil
}

// keyFuncFor 为指定密钥构造算法白名单校验函数。
func keyFuncFor(key []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("%w: %v", ErrUnsupportedSigningMethod, token.Header["alg"])
		}
		return key, nil
	}
}

// GenToken 颁发token access token 和 refresh token
func GenToken(UserID int64, Username string) (atoken, rtoken string, err error) {
	key, err := signingKey()
	if err != nil {
		return "", "", err
	}
	rc := jwt.RegisteredClaims{
		ExpiresAt: getJWTTime(aTokenExpiredDuration),
		Issuer:    tokenIssuer,
	}
	at := MyClaim{
		UserID,
		Username,
		rc,
	}
	atoken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, at).SignedString(key)
	if err != nil {
		return "", "", err
	}

	// refresh token 不需要保存任何用户信息
	rt := rc
	rt.ExpiresAt = getJWTTime(rTokenExpiredDuration)
	rtoken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, rt).SignedString(key)
	if err != nil {
		return "", "", err
	}
	return
}

// verifyWithKeys 使用全部已注入密钥依次尝试解析 token。
func verifyWithKeys(tokenID string, claims jwt.Claims) error {
	if len(secretKeys) == 0 {
		return ErrSecretNotConfigured
	}
	var lastErr error
	for _, key := range secretKeys {
		if _, err := jwt.ParseWithClaims(tokenID, claims, keyFuncFor(key), jwt.WithValidMethods([]string{"HS256"})); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// VerifyToken 验证Token
func VerifyToken(tokenID string) (*MyClaim, error) {
	if len(secretKeys) == 0 {
		return nil, ErrSecretNotConfigured
	}
	myc := new(MyClaim)
	if err := verifyWithKeys(tokenID, myc); err != nil {
		return nil, err
	}
	if myc.Issuer != tokenIssuer {
		return nil, fmt.Errorf("JWT issuer mismatch: %q", myc.Issuer)
	}
	return myc, nil
}

// RefreshToken 通过 refresh token 刷新 atoken
// 先完整校验 refresh token（算法、签发者、有效期），再尝试从旧 access token 恢复用户信息。
func RefreshToken(atoken, rtoken string) (newAtoken, newRtoken string, err error) {
	if len(secretKeys) == 0 {
		return "", "", ErrSecretNotConfigured
	}
	// refresh token 无效或过期直接返回错误
	var refreshClaim MyClaim
	if err = verifyWithKeys(rtoken, &refreshClaim); err != nil {
		return "", "", fmt.Errorf("verify refresh token: %w", err)
	}
	if refreshClaim.Issuer != tokenIssuer {
		return "", "", fmt.Errorf("refresh token issuer mismatch: %q", refreshClaim.Issuer)
	}
	// 从旧 access token 中解析出 claims 数据
	var claim MyClaim
	err = verifyWithKeys(atoken, &claim)
	if err == nil {
		// access token 仍然有效，直接重新颁发
		return GenToken(claim.UserID, claim.UserEmail)
	}
	// 仅当 access token 是因为正常过期时才允许刷新
	var validationErr *jwt.ValidationError
	if errors.As(err, &validationErr) && validationErr.Errors&jwt.ValidationErrorExpired != 0 {
		return GenToken(claim.UserID, claim.UserEmail)
	}
	return "", "", fmt.Errorf("parse access token for refresh: %w", err)
}
