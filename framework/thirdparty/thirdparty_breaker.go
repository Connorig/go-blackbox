package thirdparty

import (
	"context"
	"errors"
	"fmt"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/Connorig/go-blackbox/framework/circuit"
)

// breakerClassify 熔断错误分类:网络错误/5xx 计失败,4xx 业务错误不计。
func breakerClassify(err error) bool {
	if err == nil {
		return false
	}
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		// 4xx(客户端/业务错误)不触发熔断
		if appErr.HTTPStatus >= 400 && appErr.HTTPStatus < 500 {
			return false
		}
	}
	return true
}

// doWithBreaker 执行受熔断保护的单次请求(含状态码判定)。
// 语义:
//   - 网络错误 / 5xx → 返回错误并计入熔断失败
//   - 4xx → 返回错误但不计入熔断失败(业务错误)
//   - 2xx/3xx → 返回 nil
//   - 熔断打开 → 快速返回 B0200(系统容灾被触发),不发起真实请求
func (c *Client) doWithBreaker(ctx context.Context, method, url string, body []byte) (int, []byte, error) {
	var status int
	var respBody []byte
	run := func() error {
		var callErr error
		status, respBody, callErr = c.roundTrip(ctx, method, url, body)
		if callErr != nil {
			return callErr // 网络层错误:计入失败
		}
		if status >= 400 {
			return apperr.NewWithStatus(status, apperr.CodeThirdPartyError,
				fmt.Sprintf("thirdparty: unexpected status %d: %s", status, truncate(respBody, 200)))
		}
		return nil
	}

	if c.breaker == nil {
		err := run()
		return status, respBody, err
	}
	err := c.breaker.Execute(run, breakerClassify)
	if errors.Is(err, circuit.ErrOpen) {
		return 0, nil, apperr.New(apperr.CodeSystemDisasterTriggered,
			"thirdparty: circuit breaker is open, request rejected")
	}
	return status, respBody, err
}
