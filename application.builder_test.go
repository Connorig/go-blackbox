package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/server/shutdown"
	"github.com/kataras/iris/v12"
	context2 "github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/core/router"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWeb(t *testing.T) {
	go time.AfterFunc(time.Second*50, func() {
		shutdown.Exit("it is about time to shutdown web server, you asshole!")
	})
	err2 := New().
		Start(func(ctx context.Context, builder *ApplicationBuild) error {
			// 加载项目配置文件
			//if err := builder.LoadConfig(&loader.Config, func(loader loader.Loader) {
			//	loader.SetConfigFileSearcher("config", ".")
			//}); err != nil {
			//	return err
			//}

			//dbConfig := &datasource.PostgresConfig{
			//	UserName:     "ows",
			//	Password:     "thingple",
			//	Host:         "127.0.0.1",
			//	Port:         5442,
			//	DbName:       "test",
			//	AliasName:    "",
			//	SSL:          "disable",
			//	MaxIdleConns: 20,
			//	MaxOpenConns: 10,
			//}

			//redConfig := cache.RedisOptions{
			//	Addr:     "127.0.0.1:6380",
			//	Password: "123456",
			//	DB:       0,
			//}
			builder.
				InitLog(".", "debug").                          // 初始化日志
				EnableWeb(TimeFormat, ":8899", "debug", Router) // 开启webServer
			//EnableDb(dbConfig, RegisterTables()...)  // 开启数据库操作
			//SetSeeds(Setup).InitCronJob().           // 启动服务3s后的一些后置函数、定时任务执行
			//EnableCache(redConfig)                   // 开启redis
			return nil
		})

	t.Log(err2)
}

type User struct {
	gorm.Model
	Name string
	Age  int
}

//func (User) TableName() string {
//	return "user_test"
//}

func Router(application *iris.Application) {
	application.PartyFunc("/v1", func(p router.Party) {
		p.Get("/one", func(c *context2.Context) {
			_, err := c.WriteString("For those of you who are fucking busy, this is what happened last week on shameless.")
			if err != nil {
				return
			}
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
	modules := []func(context.Context) error{
		// 普通后置函数
		func(ctx context.Context) error {
			fmt.Println("hello it's a module func.")
			return nil
		},
		// 定时函数，必须InitCronJob() 才会执行定时任务
		func(ctx context.Context) error {
			if _, err2 := CronJobSingle().AddFunc("@every 1s", func() {
				fmt.Println("func running in 1 sec....")
			}); err2 != nil {
				return err2
			}
			return nil
		}}

	// 批量执行
	for _, m := range modules {
		err = m(ctx)
		if err != nil {
			break
		}
	}
	return
}

//type Student struct {
//	Field int
//}

/*
(* T)(nil) 它返回nil指针或没有指针，但仍然为struct的所有字段分配内存。
new（T）和& T {} 完全等价：分配一个零T并返回一个指向这个分配的内存的指针。唯一的区别是，& T {} 不适用于内置类型，如 int ;你只能做 new（int）。
*/
//func TestNil(t *testing.T) {
//	test1 := &Student{}
//	test2 := new(Student)
//	test3 := (*Student)(nil)
//	fmt.Printf("%#v, %#v, %#v \n", test1, test2, test3) //&main.Student{Field:0}, &main.Student{Field:0}, (*main.Student)(nil)
//
//	fmt.Printf("%T, %T, %T \n", test1, test2, test3) // *main.Student, *main.Student, *main.Student
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
