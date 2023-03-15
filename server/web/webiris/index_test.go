package web_iris

import (
	"awesomeProject1/server/cache"
	"awesomeProject1/server/datasource"
	"awesomeProject1/server/web"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	go func() {

		init := Init(":18085", "2006-01-02 15:04:05")
		init.parties = append(init.parties, Party{
			Perfix: "/v0/version",
			PartyFunc: func(index iris.Party) {
				index.Get("/", func(context *context.Context) {
					context.WriteString("IRIS-ADMIN is running!!!")
				})
			},
		})

		datasource.GormInit(&datasource.PostgresConfig{
			UserName:     "postgres",
			Password:     "postgres",
			Host:         "127.0.0.1",
			Port:         5432,
			DbName:       "demo",
			AliasName:    "default",
			SSL:          "false",
			MaxOpenConns: 100,
			MaxIdleConns: 10,
		}, datasource.RegisterTables()...)

		cache.Instance()

		web.Start(init)

	}()

	time.Sleep(2 * time.Second)

	//t.Run("test web run", func(t *testing.T) {
	//	resp, err := http.Get("http://127.0.0.1:18085/v0/version/")
	//	if err != nil {
	//		t.Errorf("test web start get %v", err)
	//	}
	//	defer resp.Body.Close()
	//	s, err := ioutil.ReadAll(resp.Body)
	//	if err != nil {
	//		t.Errorf("test web start get %v", err)
	//	}
	//	if string(s) != "IRIS-ADMIN is running!!!" {
	//		t.Errorf("test web start want %s but get %s", "Not Found", string(s))
	//	}
	//})
}
