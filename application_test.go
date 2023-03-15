package awesomeProject1

import (
	"awesomeProject1/server/datasource"
	"awesomeProject1/server/loadConf"
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/kataras/iris/v12"
	irisContext "github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/core/router"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestLoader(t *testing.T) {

	app := New()

	go func() {
		app.Start(func(ctx context.Context, builder *applicationStart) error {

			builder.loadConfig(&loadConf.Config, func(loader loadConf.Loader) {
				loader.SetConfigFileSearcher("config", ".")
			})

			postConfig := datasource.PostgresConfig{
				UserName: loadConf.Config.Db.User,
				Password: loadConf.Config.Db.Password,
				Host:     loadConf.Config.Db.Host,
				Port:     loadConf.Config.Db.Port,
				DbName:   loadConf.Config.Db.DbName,
				InitDb:   false,
				//AliasName:    "",
				SSL:          loadConf.Config.Db.Ssl,
				MaxIdleConns: loadConf.Config.Db.MaxIdleConns,
				MaxOpenConns: loadConf.Config.Db.MaxOpenConns,
			}

			redConfig := redis.UniversalOptions{
				Addrs:    strings.Split(loadConf.Config.Redis.Addrs, ","),
				Password: loadConf.Config.Redis.Password,
				PoolSize: loadConf.Config.Redis.PoolSize,
				DB:       loadConf.Config.Redis.Db,
			}

			builder.
				enableDb(&postConfig).
				enableCache(redConfig).
				enableWeb(loadConf.Config.Web.Listen, loadConf.Config.Web.DebugLevel, Router)
			return nil
		}, func(s string) {

		})
	}()
	// 等待初始化完成。
	time.Sleep(time.Second * 3)
	t.Log(app.builder.redisDb)
	t.Log(app.builder.gormDb)

	t.Run("test", func(t *testing.T) {
		resp, err := http.Get("http://localhost:9527/v1/one")
		if err != nil {
			t.Errorf("test web start get %v", err)
		}
		defer resp.Body.Close()
		s, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("test web start get %v", err)
		}
		t.Logf("%s", s)
	})
}

//func TestApplication(t *testing.T) {
//
//	go func() { Start().enableWeb(":8989", "error", Router) }()
//
//	time.Sleep(time.Second * 3)
//
//	t.Run("test", func(t *testing.T) {
//		resp, err := http.Get("http://localhost:8989/v1/one")
//		if err != nil {
//			t.Errorf("test web start get %v", err)
//		}
//		defer resp.Body.Close()
//		s, err := ioutil.ReadAll(resp.Body)
//		if err != nil {
//			t.Errorf("test web start get %v", err)
//		}
//		t.Logf("%s", s)
//	})
//}

func Router(application *iris.Application) {
	application.PartyFunc("/v1", func(p router.Party) {
		p.Get("/one", func(c *irisContext.Context) {
			c.WriteString("Here you are!")
		})
	})
}
