package appbox

import (
	"context"
	"github.com/Domingor/go-blackbox/server/cache"
	"github.com/Domingor/go-blackbox/server/datasource"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
	"testing"
	"time"
)

type User struct {
	gorm.Model
	Name string
	Age  int
}

func (User) TableName() string {
	return "user_info"
}

func TestLoader(t *testing.T) {

	app := New()

	go func() {

		app.Start(func(ctx context.Context, builder *ApplicationBuild) error {

			builder.LoadConfig(&loadconf.Config, func(loader loadconf.Loader) {
				loader.SetConfigFileSearcher("config", ".")
			})

			postConfig := datasource.PostgresConfig{
				UserName: loadconf.Config.Db.User,
				Password: loadconf.Config.Db.Password,
				Host:     loadconf.Config.Db.Host,
				Port:     loadconf.Config.Db.Port,
				DbName:   loadconf.Config.Db.DbName,
				InitDb:   false,
				//AliasName:    "",
				SSL:          loadconf.Config.Db.Ssl,
				MaxIdleConns: loadconf.Config.Db.MaxIdleConns,
				MaxOpenConns: loadconf.Config.Db.MaxOpenConns,
			}

			cacheOptins := cache.RedisOptions{
				Addr:     loadconf.Config.Redis.Addrs,
				Password: loadconf.Config.Redis.Password,
				PoolSize: loadconf.Config.Redis.PoolSize,
				DB:       loadconf.Config.Redis.Db,
			}

			tables := RegisterTables()

			builder.
				EnableDb(&postConfig, tables).
				EnableCache(ctx, cacheOptins)
			//EnableWeb(webiris.TimeFormat,
			//	loadconf.Config.Web.Listen,
			//	loadconf.Config.Web.DebugLevel,
			//	Router)
			return nil
		})
	}()

	//等待初始化完成。
	time.Sleep(time.Second * 2)
	//t.Log(etc.GetContext())
	//t.Log(etc.GetDb())
	//t.Log(etc.GetCache())

	t.Log(GormDb())
	t.Log(GlobalCtx())
	t.Log(RedisCache())

	//t.Run("test", func(t *testing.T) {
	//	resp, err := http.Get("http://localhost:9000/v1/one")
	//	if err != nil {
	//		t.Errorf("test web start get %v", err)
	//	}
	//	defer resp.Body.Close()
	//	s, err := ioutil.ReadAll(resp.Body)
	//	if err != nil {
	//		t.Errorf("test web start get %v", err)
	//	}
	//	t.Logf("%s", s)
	//})
}

func Router(application *iris.Application) {
	//application.PartyFunc("/v1", func(p router.Party) {
	//	p.Get("/one", func(c *context2.Context) {
	//		c.WriteString("Here you are!")
	//	})
	//})
}

func RegisterTables() (tables []interface{}) {
	tables = append(tables,
		new(User),
	)
	return
}
