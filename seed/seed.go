package seed

import (
	"context"
	"github.com/Domingor/go-blackbox/appioc"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"go.uber.org/zap"
)

// SeedFunc 启动项目时的一些回调、初始化工作
//type SeedFunc interface {
//	Init(appioc context.Context) (err error)
//}

type SeedFunc func(etc context.Context) (err error)

// Seed exec seed funcs
func Seed(SeedFunctions ...SeedFunc) error {
	zaplog.ZAPLOG.Debug("Seed funcs are running now.")

	if len(SeedFunctions) == 0 {
		return nil
	}
	for _, v := range SeedFunctions {
		err := v(appioc.GetContext().Ctx)
		if err != nil {
			zaplog.ZAPLOG.Error("Seed func running fail.", zap.Any("err", err))
			return err
		}
	}
	return nil
}
