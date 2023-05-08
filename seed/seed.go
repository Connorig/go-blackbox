package seed

import (
	"context"
	"errors"
	"github.com/Domingor/go-blackbox/appioc"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"go.uber.org/zap"
)

// SeedFunc 自定义种子函数
type SeedFunc func(etc context.Context) (err error)

// Seed exec seed funcs
func Seed(SeedFunctions ...SeedFunc) error {
	zaplog.ZAPLOG.Debug("Seed funcs are running now.")

	if len(SeedFunctions) == 0 {
		return errors.New("there is no seed func needed to run")
	}
	for _, v := range SeedFunctions {
		// 批量执行种子函数，传入上下文对象
		err := v(appioc.GetContext().Ctx)
		if err != nil {
			zaplog.ZAPLOG.Error("Seed func running fail.", zap.Any("err", err))
			return err
		}
	}
	zaplog.ZAPLOG.Info("all seed func are run now")
	return nil
}
