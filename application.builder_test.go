package appbox

import (
	"context"
	"fmt"
	"github.com/Domingor/go-blackbox/server/loadconf"
	"github.com/Domingor/go-blackbox/server/shutdown"
	"github.com/Domingor/go-blackbox/static"
	"github.com/kataras/iris/v12"
	context2 "github.com/kataras/iris/v12/context"
	"github.com/kataras/iris/v12/core/router"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWeb(t *testing.T) {
	go time.AfterFunc(time.Second*20, func() {
		shutdown.Exit("it is about time to shutdown web server, you asshole!")
	})

	err := New().
		Start(func(ctx context.Context, builder *ApplicationBuild) error {
			builder.InitLog(".", "debug").
				EnableWeb("", ":8899", "debug", Router)
			return nil
		})
	t.Log(err)
	fmt.Println("ending")
}

func Test2(t *testing.T) {
	go time.AfterFunc(time.Second*100, func() {
		shutdown.Exit("time to shutdown")
	})
	err := New().Start(func(ctx context.Context, builder *ApplicationBuild) error {

		err := builder.LoadConfig(&loadconf.Config, func(loader loadconf.Loader) {
			loader.SetConfigFileSearcher("config", ".")
		})
		if err != nil {
			return err
		}

		builder.
			InitLog(".", "debug").
			EnableStaticSource(static.StaticFile).
			EnableWeb("", ":8899", "debug", nil)
		return nil
	})
	if err != nil {
		t.Error(err)
	}
}

type User struct {
	gorm.Model
	Name string
	Age  int
}

func (User) TableName() string {
	return "user_info"
}

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

//type Struct struct {
//	Field int
//}

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
