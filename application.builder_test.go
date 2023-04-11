package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"github.com/kataras/iris/v12"
	context2 "github.com/kataras/iris/v12/context"

	"github.com/kataras/iris/v12/core/router"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"testing"
	"time"
)

type User struct {
	gorm.Model
	Name string
	Age  int
}

type Struct struct {
	Field int
}

/*
(* T)(nil) 它返回nil指针或没有指针，但仍然为struct的所有字段分配内存。
new（T）和& T {} 完全等价：分配一个零T并返回一个指向这个分配的内存的指针。唯一的区别是，& T {} 不适用于内置类型，如 int ;你只能做 new（int）。
*/
//func TestNil(t *testing.T) {
//	test1 := &Struct{}
//	test2 := new(Struct)
//	test3 := (*Struct)(nil)
//	fmt.Printf("%#v, %#v, %#v \n", test1, test2, test3) //&main.Struct{Field:0}, &main.Struct{Field:0}, (*main.Struct)(nil)
//
//	fmt.Printf("%T, %T, %T \n", test1, test2, test3) // *main.Struct, *main.Struct, *main.Struct
//
//	test1.Field = 1
//	fmt.Println(test1.Field) // 1
//
//	test2.Field = 2
//	fmt.Println(test2.Field) // 2
//
//	//test3.Field = 3 // test3分配内存，返回一个nil指针，不能使用
//	// fmt.Println(test3.Field)
//
//	configType := reflect.TypeOf(test3)
//	println(configType.String())
//
//	field := test3.Field
//	t.Log(field)
//
//}

func (User) TableName() string {
	return "user_info"
}

func TestLoader(t *testing.T) {

	app := New()

	go func() {

		app.Start(func(ctx context.Context, builder *ApplicationBuild) error {

			err := builder.LoadConfig(&loadconf.Config, func(loader loadconf.Loader) {
				loader.SetConfigFileSearcher("config", ".")
			})

			//postConfig := datasource.PostgresConfig{
			//	UserName: loadconf.Config.Db.user,
			//	Password: loadconf.Config.Db.Password,
			//	host:     loadconf.Config.Db.host,
			//	Port:     loadconf.Config.Db.Port,
			//	DbName:   loadconf.Config.Db.DbName,
			//	InitDb:   false,
			//	//AliasName:    "",
			//	SSL:          loadconf.Config.Db.Ssl,
			//	MaxIdleConns: loadconf.Config.Db.MaxIdleConns,
			//	MaxOpenConns: loadconf.Config.Db.MaxOpenConns,
			//}
			//
			//cacheOptins := cache.RedisOptions{
			//	Addr:     loadconf.Config.Redis.Addrs,
			//	Password: loadconf.Config.Redis.Password,
			//	PoolSize: loadconf.Config.Redis.PoolSize,
			//	DB:       loadconf.Config.Redis.Db,
			//}
			//
			//tables := RegisterTables()

			builder.
				//EnableDb(&postConfig, tables).
				//EnableCache(ctx, cacheOptins).
				InitLog(loadconf.Config.LogConf.OutDirPath, loadconf.Config.LogConf.LogLevel).
				SetSeeds(Setup)
			//EnableWeb(webiris.TimeFormat,
			//	loadconf.Config.Web.Listen,
			//	loadconf.Config.Web.DebugLevel,
			//	Router)
			//fmt.Println("builder finished...")
			zaplog.ZAPLOG.Info("builder finished...", zap.Any("Err", 1))
			return err
		})
	}()

	//等待初始化完成。
	time.Sleep(time.Second * 2)
	//t.Log(appioc.GetContext())
	//t.Log(appioc.GetDb())
	//t.Log(appioc.GetCache())

	//t.Log(GormDb())
	//t.Log(GlobalCtx().Ctx)
	//t.Log(RedisCache())

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
	application.PartyFunc("/v1", func(p router.Party) {
		p.Get("/one", func(c *context2.Context) {
			c.WriteString("Here you are!")
		})
	})
}

func RegisterTables() (tables []interface{}) {
	tables = append(tables,
		new(User),
	)
	return
}

func Setup(ctx context.Context) (err error) {
	modules := []func(context.Context) error{func(ctx context.Context) error {
		fmt.Println("hello it's a module func.")
		return nil
	}}

	for _, m := range modules {
		err = m(ctx)
		if err != nil {
			break
		}
	}
	return
}

//type user struct {
//	gorm.Model
//	Name string
//	Age  int
//}
//
//func (user) TableName() string {
//	return "user_info"
//}
//
//func TestLoader(t *testing.T) {
//
//	app := New()
//
//	go func() {
//
//		app.Start(func(ctx context.Context, builder *ApplicationBuild) error {
//
//			err := builder.LoadConfig(&loadconf.Config, func(loader loadconf.Loader) {
//				loader.SetConfigFileSearcher("config", ".")
//			})
//
//			postConfig := datasource.PostgresConfig{
//				UserName: loadconf.Config.Db.user,
//				Password: loadconf.Config.Db.Password,
//				host:     loadconf.Config.Db.host,
//				Port:     loadconf.Config.Db.Port,
//				DbName:   loadconf.Config.Db.DbName,
//				InitDb:   false,
//				//AliasName:    "",
//				SSL:          loadconf.Config.Db.Ssl,
//				MaxIdleConns: loadconf.Config.Db.MaxIdleConns,
//				MaxOpenConns: loadconf.Config.Db.MaxOpenConns,
//			}
//
//			cacheOptins := cache.RedisOptions{
//				Addr:     loadconf.Config.Redis.Addrs,
//				Password: loadconf.Config.Redis.Password,
//				PoolSize: loadconf.Config.Redis.PoolSize,
//				DB:       loadconf.Config.Redis.Db,
//			}
//
//			tables := RegisterTables()
//
//			builder.
//				EnableDb(&postConfig, tables).
//				EnableCache(ctx, cacheOptins)
//			//EnableWeb(webiris.TimeFormat,
//			//	loadconf.Config.Web.Listen,
//			//	loadconf.Config.Web.DebugLevel,
//			//	Router)
//
//			return err
//		})
//	}()
//
//	//等待初始化完成。
//	time.Sleep(time.Second * 2)
//	//t.Log(appioc.GetContext())
//	//t.Log(appioc.GetDb())
//	//t.Log(appioc.GetCache())
//
//	t.Log(GormDb())
//	t.Log(GlobalCtx().Ctx)
//	t.Log(RedisCache())
//
//	//t.Run("test", func(t *testing.T) {
//	//	resp, err := http.Get("http://localhost:9000/v1/one")
//	//	if err != nil {
//	//		t.Errorf("test web start get %v", err)
//	//	}
//	//	defer resp.Body.Close()
//	//	s, err := ioutil.ReadAll(resp.Body)
//	//	if err != nil {
//	//		t.Errorf("test web start get %v", err)
//	//	}
//	//	t.Logf("%s", s)
//	//})
//}
//
//func Router(application *iris.Application) {
//	//application.PartyFunc("/v1", func(p router.Party) {
//	//	p.Get("/one", func(c *context2.Context) {
//	//		c.WriteString("Here you are!")
//	//	})
//	//})
//}
//
//func RegisterTables() (tables []interface{}) {
//	tables = append(tables,
//		new(user),
//	)
//	return
//}
