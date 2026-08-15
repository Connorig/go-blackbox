package apptoken

import (
	"github.com/golang-jwt/jwt/v4"
)

// 组织身份声明扩展(增量,兼容):
// MyClaim 增加 OrgID/DeptID,旧 token 无该字段时解析为零值(不限制)。
// 配套数据权限体系:webiris.Auth 认证后通过 webiris.DataScope(ctx) 读取,
// 业务查询自动按组织/部门隔离(见 framework/database 的 DataScope)。

// MyClaim 是 Access Token 携带的业务声明。
// Scope 是权限标识列表(逗号分隔);OrgID/DeptID 是数据权限范围,
// 由 GenTokenFull 写入(GenTokenWithScope 签发时为零值)。
type MyClaim struct {
	UserID    int64  `json:"userId"`
	UserEmail string `json:"userEmail"`
	Scope     string `json:"scope,omitempty"`  // 权限标识,逗号分隔
	OrgID     int64  `json:"orgId,omitempty"`  // 组织 ID(数据权限)
	DeptID    int64  `json:"deptId,omitempty"` // 部门 ID(数据权限)
	jwt.RegisteredClaims
}

// GenTokenFull 颁发携带权限声明与组织身份的 token。
// scope 为逗号分隔权限标识;orgID/deptID 为数据权限范围
// (业务无组织概念时传 0,行为与 GenTokenWithScope 一致)。
func GenTokenFull(UserID int64, Username, scope string, orgID, deptID int64) (atoken, rtoken string, err error) {
	key, err := signingKey()
	if err != nil {
		return "", "", err
	}
	rc := jwt.RegisteredClaims{
		ExpiresAt: getJWTTime(aTokenExpiredDuration),
		Issuer:    tokenIssuer,
	}
	at := MyClaim{
		UserID:           UserID,
		UserEmail:        Username,
		Scope:            scope,
		OrgID:            orgID,
		DeptID:           deptID,
		RegisteredClaims: rc,
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

// GenTokenWithScope 颁发携带权限声明的 token(组织身份为零值)。
// 需要组织身份时使用 GenTokenFull。
func GenTokenWithScope(UserID int64, Username, scope string) (atoken, rtoken string, err error) {
	return GenTokenFull(UserID, Username, scope, 0, 0)
}
