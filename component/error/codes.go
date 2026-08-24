package apperr

// 错误码遵循阿里巴巴《Java开发手册(泰山版)》附3 错误码体系：
//   A 系列：用户端错误（参数、认证、权限、资源等）
//   B 系列：系统端错误（超时、限流、资源耗尽等）
//   C 系列：第三方服务错误（数据库、缓存、消息、通知等）
// 业务可在此基础上按 一级/二级 结构扩展自定义码，保持前缀语义。

const (
	// CodeOK 一切正常（手册：00000）。
	CodeOK Code = "00000"
)

// ===== A 系列：用户端错误 =====

const (
	// CodeUserError 用户端错误（一级）。
	CodeUserError Code = "A0001"
	// CodeUserRegisterError 用户注册错误。
	CodeUserRegisterError Code = "A0100"
	// CodeUserNameInvalid 用户名校验失败。
	CodeUserNameInvalid Code = "A0110"
	// CodeUserNameExists 用户名已存在。
	CodeUserNameExists Code = "A0111"
	// CodePasswordInvalid 密码校验失败。
	CodePasswordInvalid Code = "A0120"
	// CodeVerifyCodeInvalid 校验码输入错误。
	CodeVerifyCodeInvalid Code = "A0130"
	// CodeUserInfoInvalid 用户基本信息校验失败。
	CodeUserInfoInvalid Code = "A0150"
	// CodePhoneFormatInvalid 手机格式校验失败。
	CodePhoneFormatInvalid Code = "A0151"
	// CodeEmailFormatInvalid 邮箱格式校验失败。
	CodeEmailFormatInvalid Code = "A0153"
	// CodeUserLoginError 用户登录异常。
	CodeUserLoginError Code = "A0200"
	// CodeUserAccountNotExists 用户账户不存在。
	CodeUserAccountNotExists Code = "A0201"
	// CodeUserAccountFrozen 用户账户被冻结。
	CodeUserAccountFrozen Code = "A0202"
	// CodeUserPasswordWrong 用户密码错误。
	CodeUserPasswordWrong Code = "A0210"
	// CodeUserLoginExpired 用户登录已过期。
	CodeUserLoginExpired Code = "A0230"
	// CodeUserVerifyCodeWrong 用户验证码错误。
	CodeUserVerifyCodeWrong Code = "A0240"
	// CodeAccessPermissionError 访问权限异常。
	CodeAccessPermissionError Code = "A0300"
	// CodeAccessUnauthorized 访问未授权。
	CodeAccessUnauthorized Code = "A0301"
	// CodeAPINoPermission 无权限使用 API。
	CodeAPINoPermission Code = "A0312"
	// CodeUserAccessBlocked 用户访问被拦截。
	CodeUserAccessBlocked Code = "A0320"
	// CodeUserSignatureError 用户签名异常。
	CodeUserSignatureError Code = "A0340"
	// CodeRSASignatureError RSA 签名错误。
	CodeRSASignatureError Code = "A0341"
	// CodeRequestParamError 用户请求参数错误。
	CodeRequestParamError Code = "A0400"
	// CodeInvalidUserInput 无效的用户输入。
	CodeInvalidUserInput Code = "A0402"
	// CodeRequiredParamEmpty 请求必填参数为空。
	CodeRequiredParamEmpty Code = "A0410"
	// CodeParamOutOfRange 请求参数值超出允许的范围。
	CodeParamOutOfRange Code = "A0420"
	// CodeParamFormatMismatch 参数格式不匹配。
	CodeParamFormatMismatch Code = "A0421"
	// CodeAmountExceedsLimit 金额超出限制。
	CodeAmountExceedsLimit Code = "A0424"
	// CodeJSONParseFailed 请求 JSON 解析失败。
	CodeJSONParseFailed Code = "A0427"
	// CodeUserOperationError 用户操作异常。
	CodeUserOperationError Code = "A0440"
	// CodeRequestServiceError 用户请求服务异常。
	CodeRequestServiceError Code = "A0500"
	// CodeRequestRateLimited 请求次数超出限制。
	CodeRequestRateLimited Code = "A0501"
	// CodeRequestConcurrencyLimited 请求并发数超出限制。
	CodeRequestConcurrencyLimited Code = "A0502"
	// CodeDuplicateRequest 用户重复请求。
	CodeDuplicateRequest Code = "A0506"
	// CodeUploadFileError 用户上传文件异常。
	CodeUploadFileError Code = "A0700"
	// CodeUploadFileTypeMismatch 用户上传文件类型不匹配。
	CodeUploadFileTypeMismatch Code = "A0701"
	// CodeUploadFileTooLarge 用户上传文件太大。
	CodeUploadFileTooLarge Code = "A0702"
)

// ===== B 系列：系统端错误 =====

const (
	// CodeSystemError 系统执行出错（一级）。
	CodeSystemError Code = "B0001"
	// CodeSystemTimeout 系统执行超时。
	CodeSystemTimeout Code = "B0100"
	// CodeSystemDisasterTriggered 系统容灾功能被触发。
	CodeSystemDisasterTriggered Code = "B0200"
	// CodeSystemRateLimited 系统限流。
	CodeSystemRateLimited Code = "B0210"
	// CodeSystemDegraded 系统功能降级。
	CodeSystemDegraded Code = "B0220"
	// CodeSystemResourceError 系统资源异常。
	CodeSystemResourceError Code = "B0300"
	// CodeSystemResourceExhausted 系统资源耗尽。
	CodeSystemResourceExhausted Code = "B0310"
	// CodeConnectionPoolExhausted 系统连接池耗尽。
	CodeConnectionPoolExhausted Code = "B0314"
	// CodeThreadPoolExhausted 系统线程池耗尽。
	CodeThreadPoolExhausted Code = "B0315"
)

// ===== C 系列：第三方服务错误 =====

const (
	// CodeThirdPartyError 调用第三方服务出错（一级）。
	CodeThirdPartyError Code = "C0001"
	// CodeMiddlewareError 中间件服务出错。
	CodeMiddlewareError Code = "C0100"
	// CodeMessageServiceError 消息服务出错。
	CodeMessageServiceError Code = "C0120"
	// CodeMessagePublishFailed 消息投递出错。
	CodeMessagePublishFailed Code = "C0121"
	// CodeMessageConsumeFailed 消息消费出错。
	CodeMessageConsumeFailed Code = "C0122"
	// CodeCacheServiceError 缓存服务出错。
	CodeCacheServiceError Code = "C0130"
	// CodeConfigServiceError 配置服务出错。
	CodeConfigServiceError Code = "C0140"
	// CodeThirdPartyTimeout 第三方系统执行超时。
	CodeThirdPartyTimeout Code = "C0200"
	// CodeDatabaseTimeout 数据库服务超时。
	CodeDatabaseTimeout Code = "C0250"
	// CodeDatabaseError 数据库服务出错。
	CodeDatabaseError Code = "C0300"
	// CodeTableNotExists 表不存在。
	CodeTableNotExists Code = "C0311"
	// CodePrimaryKeyConflict 主键冲突。
	CodePrimaryKeyConflict Code = "C0341"
	// CodeNotifyServiceError 通知服务出错。
	CodeNotifyServiceError Code = "C0500"
	// CodeMailNotifyFailed 邮件提醒服务失败。
	CodeMailNotifyFailed Code = "C0503"
)

// 常用错误码的 HTTP 状态映射（供 webiris.Fail 默认使用）。
var defaultHTTPStatus = map[Code]int{
	CodeOK:                        200,
	CodeUserError:                 400,
	CodeUserRegisterError:         400,
	CodeUserNameInvalid:           400,
	CodeUserNameExists:            400,
	CodePasswordInvalid:           400,
	CodeVerifyCodeInvalid:         400,
	CodeUserInfoInvalid:           400,
	CodePhoneFormatInvalid:        400,
	CodeEmailFormatInvalid:        400,
	CodeUserLoginError:            401,
	CodeUserAccountNotExists:      401,
	CodeUserAccountFrozen:         403,
	CodeUserPasswordWrong:         401,
	CodeUserLoginExpired:          401,
	CodeUserVerifyCodeWrong:       401,
	CodeAccessPermissionError:     403,
	CodeAccessUnauthorized:        401,
	CodeAPINoPermission:           403,
	CodeUserAccessBlocked:         403,
	CodeUserSignatureError:        401,
	CodeRSASignatureError:         401,
	CodeRequestParamError:         400,
	CodeInvalidUserInput:          400,
	CodeRequiredParamEmpty:        400,
	CodeParamOutOfRange:           400,
	CodeParamFormatMismatch:       400,
	CodeAmountExceedsLimit:        400,
	CodeJSONParseFailed:           400,
	CodeUserOperationError:        400,
	CodeRequestServiceError:       500,
	CodeRequestRateLimited:        429,
	CodeRequestConcurrencyLimited: 429,
	CodeDuplicateRequest:          409,
	CodeUploadFileError:           400,
	CodeUploadFileTypeMismatch:    400,
	CodeUploadFileTooLarge:        413,
	CodeSystemError:               500,
	CodeSystemTimeout:             504,
	CodeSystemDisasterTriggered:   503,
	CodeSystemRateLimited:         429,
	CodeSystemDegraded:            503,
	CodeSystemResourceError:       503,
	CodeSystemResourceExhausted:   503,
	CodeConnectionPoolExhausted:   503,
	CodeThreadPoolExhausted:       503,
	CodeThirdPartyError:           502,
	CodeMiddlewareError:           502,
	CodeMessageServiceError:       502,
	CodeMessagePublishFailed:      502,
	CodeMessageConsumeFailed:      502,
	CodeCacheServiceError:         502,
	CodeConfigServiceError:        502,
	CodeThirdPartyTimeout:         504,
	CodeDatabaseTimeout:           504,
	CodeDatabaseError:             500,
	CodeTableNotExists:            500,
	CodePrimaryKeyConflict:        409,
	CodeNotifyServiceError:        502,
	CodeMailNotifyFailed:          502,
}

// HTTPStatus 返回错误码对应的默认 HTTP 状态；未注册时返回 500。
func HTTPStatus(code Code) int {
	if status, ok := defaultHTTPStatus[code]; ok {
		return status
	}
	return 500
}
