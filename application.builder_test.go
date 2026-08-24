package appbox

import (
	"context"
	"fmt"
	"github.com/Connorig/go-blackbox/framework/cache"
	"github.com/Connorig/go-blackbox/framework/config"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/Connorig/go-blackbox/framework/lifecycle"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Connorig/go-blackbox/framework/log"
	"github.com/Connorig/go-blackbox/framework/web"
	"github.com/kataras/iris/v12"
	"gorm.io/gorm"
)

// TestWeb 验证依赖项目可以分别注册 Web 启动前、启动后和 Cron Seed 回调。
// 该测试会启动真实 TCP Listener，因此默认跳过，仅在显式开启集成测试时执行。
func TestWeb(t *testing.T) {
	if os.Getenv("GO_BLACKBOX_WEB_INTEGRATION") != "1" {
		t.Skip("set GO_BLACKBOX_WEB_INTEGRATION=1 to run the Web integration test")
	}

	logDirectory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(logDirectory, "zap"), 0o755); err != nil {
		t.Fatalf("create integration test log directory failed: %v", err)
	}

	exitTimer := time.AfterFunc(500*time.Millisecond, func() {
		shutdown.Exit("Web integration test completed")
	})
	t.Cleanup(func() {
		exitTimer.Stop()
	})

	err := New().Start(func(ctx context.Context, builder *ApplicationBuild) error {

		// 加载项目配置文件
		if err := builder.LoadConfig(&apploader.Config, func(loader apploader.Loader) {
			loader.SetConfigFileSearcher("config", ".")
		}); err != nil {
			return err
		}

		dbConfig := &datasource.PostgresConfig{
			UserName:     apploader.Config.Db.User,
			Password:     apploader.Config.Db.Password,
			Host:         apploader.Config.Db.Host,
			Port:         apploader.Config.Db.Port,
			DbName:       apploader.Config.Db.DbName,
			AliasName:    "",
			SSL:          apploader.Config.Db.Ssl,
			MaxIdleConns: 20,
			MaxOpenConns: 10,
		}
		redConfig := cache.RedisOptions{
			Addr:     apploader.Config.Redis.Host,
			Password: apploader.Config.Redis.Password,
			DB:       apploader.Config.Redis.Db,
		}
		t.Log(dbConfig, redConfig)
		builder.
			InitLog(logDirectory, "debug").
			BeforeSetup(BeforeWebSetup).
			EnableWebWithConfig(webiris.Config{
				Address:         apploader.Config.Web.Listen,
				TimeFormat:      TimeFormat,
				LogLevel:        "debug",
				ShutdownTimeout: time.Second,
			}, Router).
			AfterSetup(AfterWebSetup).
			SetSeeds(RegisterCronSeeds)
		return nil
	})
	if err != nil {
		t.Fatalf("start Web integration application failed: %v", err)
	}
}

// User 是 GORM Model 注册示例，供依赖项目参考数据表声明方式。
type User struct {
	gorm.Model
	Name string
	Age  int
}

// Router 注册 Web 集成测试使用的健康响应路由。
// 写入响应失败时使用 Iris Logger 记录错误，避免测试 Handler 静默忽略异常。
func Router(application *iris.Application) {
	application.Get("/v1/one", func(ctx iris.Context) {
		println("s")
		if _, err := ctx.WriteString("go-blackbox Web service is running"); err != nil {
			application.Logger().Errorf("write integration test response failed: %v", err)
		}
	})
}

// RegisterTables 返回依赖项目需要交给 GORM AutoMigrate 的 Model 列表。
func RegisterTables() []interface{} {
	return []interface{}{new(User)}
}

// BeforeWebSetup 展示 Web 启动前回调。
// 该阶段适合执行配置检查和内存预热；Context 取消时必须立即返回。
func BeforeWebSetup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("before Web setup canceled: %w", err)
	}
	zaplog.SugaredLogger.Info("before Web setup completed")
	return nil
}

// AfterWebSetup 展示 Web Ready 后回调。
// 该阶段可以执行依赖 Web 已可监听的初始化操作。
func AfterWebSetup(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("after Web setup canceled: %w", err)
	}
	zaplog.SugaredLogger.Info("after Web setup completed")
	return nil
}

// RegisterCronSeeds 注册脚手架启动后需要运行的 Cron 任务。
// SetSeeds 会在内部调用该函数并自动启用 Cron，无需额外调用 InitCronJob。
func RegisterCronSeeds(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("register Cron seeds canceled: %w", err)
	}

	if _, err := CronJobSingle().AddFunc("@every 1s", func() {
		zaplog.SugaredLogger.Info("Cron seed task is running")
	}); err != nil {
		return fmt.Errorf("add Cron seed task: %w", err)
	}
	return nil
}
