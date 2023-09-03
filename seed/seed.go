package seed

import (
	"context"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/Domingor/go-blackbox/simpleioc"
	"go.uber.org/zap"
)

// SeedFunc 自定义种子函数
type SeedFunc func(etc context.Context) (err error)

// Seed exec seed funcs
func Seed(SeedFunctions ...SeedFunc) error {
	if len(SeedFunctions) == 0 {
		zaplog.SugaredLogger.Debug("there is no seed func needed to run")
		return nil
	}
	// 批量执行种子函数（定时任务、初始化配置函数等）
	for _, v := range SeedFunctions {
		// 批量执行种子函数，传入上下文对象
		if err := v(simpleioc.GetContext().Ctx); err != nil {
			zaplog.Logger.Error("Seed func running fail.", zap.Any("err", err))
			return err
		}
	}

	zaplog.Logger.Info("all seed func have been running now")
	return nil
}
