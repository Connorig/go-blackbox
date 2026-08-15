package gencode

import (
	"errors"
	"net/http"

	apperr "github.com/Connorig/go-blackbox/component/error"
	"github.com/kataras/iris/v12"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// Config 组件配置。
type Config struct {
	// DB 数据库实例(元数据与生成记录使用;必填)。
	DB *gorm.DB
	// ModulePath 业务项目 module 路径(生成 import 用;必填)。
	ModulePath string
	// OutputDir 生成文件输出根目录(默认当前目录)。
	OutputDir string
	// Auth 管理接口认证中间件(推荐 webiris.Auth;nil 时仅限流)。
	Auth iris.Handler
	// RatePerSecond 管理接口限流(默认 5 QPS)。
	RatePerSecond float64
}

// Register 注册低代码生成平台:
//
//	GET  <prefix>                 管理页面(表列表/字段编辑/生成)
//	GET  <prefix>/api/tables      表列表(+已生成标记)
//	GET  <prefix>/api/tables/{t}  表详情(字段)
//	POST <prefix>/api/tables/{t}/columns       新增字段
//	DELETE <prefix>/api/tables/{t}/columns/{c} 删除字段
//	POST <prefix>/api/tables/{t}/sync          同步表结构
//	GET  <prefix>/api/tables/{t}/preview       代码预览
//	POST <prefix>/api/tables/{t}/generate      生成代码(force=true 覆盖)
func Register(app *iris.Application, prefix string, cfg Config) (iris.Party, error) {
	if app == nil {
		return nil, errors.New("gencode: app is nil")
	}
	if prefix == "" {
		prefix = "/gencode"
	}
	service, err := NewService(cfg.DB, cfg.ModulePath, cfg.OutputDir)
	if err != nil {
		return nil, err
	}

	party := app.Party(prefix)
	party.Get("/", pageHandler)
	party.Get("", pageHandler)

	// API:限流 + 可选认证
	handlers := []iris.Handler{}
	rps := cfg.RatePerSecond
	if rps <= 0 {
		rps = 5
	}
	limiter := rate.NewLimiter(rate.Limit(rps), int(rps))
	handlers = append(handlers, func(ctx iris.Context) {
		if !limiter.Allow() {
			failJSON(ctx, http.StatusTooManyRequests, apperr.CodeSystemRateLimited, "gencode api rate limited")
			ctx.StopExecution()
			return
		}
		ctx.Next()
	})
	if cfg.Auth != nil {
		handlers = append(handlers, cfg.Auth)
	}
	api := party.Party("/api", handlers...)

	api.Get("/tables", func(ctx iris.Context) {
		tables, err := service.ListTables(ctx.Request().Context())
		if err != nil {
			failJSON(ctx, http.StatusInternalServerError, apperr.CodeSystemError, err.Error())
			return
		}
		okJSON(ctx, tables)
	})
	api.Get("/tables/{name}", func(ctx iris.Context) {
		table, err := service.ReadTable(ctx.Request().Context(), ctx.Params().Get("name"))
		if err != nil {
			failJSON(ctx, http.StatusNotFound, apperr.CodeRequestParamError, err.Error())
			return
		}
		okJSON(ctx, table)
	})
	api.Post("/tables/{name}/columns", func(ctx iris.Context) {
		var column ColumnMeta
		if err := ctx.ReadJSON(&column); err != nil {
			failJSON(ctx, http.StatusBadRequest, apperr.CodeJSONParseFailed, "invalid json body")
			return
		}
		if err := service.AddColumn(ctx.Request().Context(), ctx.Params().Get("name"), column); err != nil {
			failJSON(ctx, http.StatusInternalServerError, apperr.CodeSystemError, err.Error())
			return
		}
		okJSON(ctx, map[string]string{"added": column.Name})
	})
	api.Delete("/tables/{name}/columns/{column}", func(ctx iris.Context) {
		if err := service.DropColumn(ctx.Request().Context(),
			ctx.Params().Get("name"), ctx.Params().Get("column")); err != nil {
			failJSON(ctx, http.StatusInternalServerError, apperr.CodeSystemError, err.Error())
			return
		}
		okJSON(ctx, map[string]string{"dropped": ctx.Params().Get("column")})
	})
	api.Post("/tables/{name}/sync", func(ctx iris.Context) {
		table, err := service.SyncTable(ctx.Request().Context(), ctx.Params().Get("name"))
		if err != nil {
			failJSON(ctx, http.StatusNotFound, apperr.CodeRequestParamError, err.Error())
			return
		}
		okJSON(ctx, table)
	})
	api.Get("/tables/{name}/preview", func(ctx iris.Context) {
		result, err := service.Preview(ctx.Request().Context(), ctx.Params().Get("name"))
		if err != nil {
			failJSON(ctx, http.StatusInternalServerError, apperr.CodeSystemError, err.Error())
			return
		}
		okJSON(ctx, result)
	})
	api.Post("/tables/{name}/generate", func(ctx iris.Context) {
		force := ctx.URLParamDefault("force", "false") == "true"
		result, overwritten, err := service.Generate(ctx.Request().Context(), ctx.Params().Get("name"), force)
		if err != nil {
			failJSON(ctx, http.StatusInternalServerError, apperr.CodeSystemError, err.Error())
			return
		}
		if !force && len(overwritten) > 0 {
			// 需要覆盖确认
			okJSON(ctx, map[string]interface{}{
				"need_confirm": true,
				"overwritten":  overwritten,
				"files":        len(result.Files),
			})
			return
		}
		okJSON(ctx, map[string]interface{}{
			"generated":   true,
			"files":       len(result.Files),
			"overwritten": overwritten,
			"route_code":  result.RouteCode,
		})
	})

	return party, nil
}

// pageHandler 管理页面。
func pageHandler(ctx iris.Context) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.Header("Cache-Control", "no-store")
	_, _ = ctx.WriteString(adminPageHTML)
}

// okJSON 统一成功响应。
func okJSON(ctx iris.Context, data interface{}) {
	_ = ctx.JSON(map[string]interface{}{"code": "00000", "message": "ok", "data": data})
}

// failJSON 统一失败响应。
func failJSON(ctx iris.Context, status int, code apperr.Code, message string) {
	ctx.StatusCode(status)
	_ = ctx.JSON(map[string]interface{}{"code": code, "message": message})
}
