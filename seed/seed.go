package seed

import (
	"context"
	"github.com/Domingor/go-blackbox/etc"
	"github.com/Domingor/go-blackbox/zap_server"
)

// SeedFunc 用户启动项目时的一些回调、初始化工作
//type SeedFunc interface {
//	Init(etc context.Context) (err error)
//}

type SeedFunc func(etc context.Context) (err error)

// Seed exec seed funcs
func Seed(SeedFunctions ...SeedFunc) error {
	zap_server.ZAPLOG.Error("error ========>")
	zap_server.ZAPLOG.Debug("debug ========>")
	if len(SeedFunctions) == 0 {
		return nil
	}
	for _, v := range SeedFunctions {
		err := v(etc.GetContext().Ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
