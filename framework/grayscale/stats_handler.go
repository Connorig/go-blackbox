package grayscale

import (
	apperr "github.com/Connorig/go-blackbox/component/error"
	web "github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
)

// StatsHandler 返回灰度命中统计 JSON(便于接入监控路由/面板):
//
//	GET /gray/stats → {code, message, data: {total, new_hits, old_hits, ratio}}
//
// 用法:app.Get("/gray/stats", strategy.StatsHandler())
func (s *Strategy) StatsHandler() iris.Handler {
	return func(ctx iris.Context) {
		if s == nil {
			web.Fail(ctx, 500, apperr.CodeSystemError, "grayscale strategy is nil")
			return
		}
		stats := s.Stats()
		web.OK(ctx, map[string]interface{}{
			"total":        stats.Total,
			"new_hits":     stats.NewHits,
			"old_hits":     stats.OldHits,
			"ratio":        stats.Ratio,
			"config_ratio": s.Ratio,
		})
	}
}
