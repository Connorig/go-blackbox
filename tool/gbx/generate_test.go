package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGeneratesProject 默认 code 风格:生成完整骨架并断言关键内容。
func TestRunGeneratesProject(t *testing.T) {
	workDir := t.TempDir()
	err := run(options{name: "demo-app", dir: workDir, module: "github.com/example/demo-app"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	root := filepath.Join(workDir, "demo-app")

	// 文件齐全
	expected := []string{
		"main.go",
		"config.toml",
		"go.mod",
		"README.md",
		".gitignore",
		"internal/model/user.go",
		"internal/handler/user.go",
	}
	for _, rel := range expected {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing generated file: %s", rel)
		}
	}

	// code 风格:显式 Enable* 装配
	mainSrc, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	mainText := string(mainSrc)
	for _, want := range []string{
		"EnableWeb(appbox.TimeFormat, \":8080\"",
		"EnableAdmin(\":6060\"",
		"webiris.SQLGuard()",
		"monitor.Register(app, \"/monitor\"",
		"apptoken.SetSecretKey",
		"datasource.DriverSQLite",
	} {
		if !strings.Contains(mainText, want) {
			t.Errorf("main.go missing %q", want)
		}
	}

	// go.mod
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "module github.com/example/demo-app") {
		t.Errorf("go.mod module wrong:\n%s", goMod)
	}

	// model/handler 模板内容
	modelSrc, _ := os.ReadFile(filepath.Join(root, "internal/model/user.go"))
	if !strings.Contains(string(modelSrc), "model.StandardModel") ||
		!strings.Contains(string(modelSrc), "model.OrgFields") {
		t.Error("model template must include StandardModel and OrgFields")
	}
	handlerSrc, _ := os.ReadFile(filepath.Join(root, "internal/handler/user.go"))
	if !strings.Contains(string(handlerSrc), "webiris.DataScope(ctx)") {
		t.Error("handler template must include data scope")
	}
	if !strings.Contains(string(handlerSrc), "apptoken.GenTokenFull") {
		t.Error("handler template must include GenTokenFull")
	}
}

// TestRunConfigStyle config 风格:AutoConfigure + 模块开关配置。
func TestRunConfigStyle(t *testing.T) {
	workDir := t.TempDir()
	err := run(options{name: "cfg-app", dir: workDir, module: "github.com/example/cfg-app", style: "config"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	root := filepath.Join(workDir, "cfg-app")
	mainSrc, _ := os.ReadFile(filepath.Join(root, "main.go"))
	mainText := string(mainSrc)
	for _, want := range []string{
		"AutoConfigure(&cfg.Modules",
		"apploader.Modules",
		"LoadConfig(&cfg",
	} {
		if !strings.Contains(mainText, want) {
			t.Errorf("config style main.go missing %q", want)
		}
	}
	configSrc, _ := os.ReadFile(filepath.Join(root, "config.toml"))
	configText := string(configSrc)
	for _, want := range []string{"[modules]", "[modules.web]", "enabled = true"} {
		if !strings.Contains(configText, want) {
			t.Errorf("config style config.toml missing %q", want)
		}
	}
}

// TestRunInvalidStyle 非法风格拒绝。
func TestRunInvalidStyle(t *testing.T) {
	if err := run(options{name: "demo", dir: t.TempDir(), style: "weird"}); err == nil {
		t.Fatal("invalid style must fail")
	}
}

// TestRunInvalidName 非法项目名拒绝。
func TestRunInvalidName(t *testing.T) {
	if err := run(options{name: "bad name!", dir: t.TempDir()}); err == nil {
		t.Fatal("invalid name must fail")
	}
	if err := run(options{name: "", dir: t.TempDir()}); err == nil {
		t.Fatal("empty name must fail")
	}
}

// TestValidName 命名校验。
func TestValidName(t *testing.T) {
	valid := []string{"demo", "demo-app", "demo_app", "Demo2"}
	invalid := []string{"", ".", "..", "a b", "a/b", "a.b"}
	for _, name := range valid {
		if !validName(name) {
			t.Errorf("%q should be valid", name)
		}
	}
	for _, name := range invalid {
		if validName(name) {
			t.Errorf("%q should be invalid", name)
		}
	}
}


// TestRunGenStyle gen 风格:CRUD 全栈(DDD 分层 + 测试表)。
func TestRunGenStyle(t *testing.T) {
	workDir := t.TempDir()
	err := run(options{name: "crud-app", dir: workDir, module: "github.com/example/crud-app", style: "gen"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	root := filepath.Join(workDir, "crud-app")

	expected := []string{
		"main.go",
		"internal/model/test_mycat.go",
		"internal/filter/test_mycat.go",
		"internal/repository/test_mycat.go",
		"internal/service/test_mycat.go",
		"internal/handler/test_mycat.go",
	}
	for _, rel := range expected {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("missing generated file: %s", rel)
		}
	}

	mainSrc, _ := os.ReadFile(filepath.Join(root, "main.go"))
	mainText := string(mainSrc)
	for _, want := range []string{
		"/api/v1/test-mycat",
		"ListTestMycat(svc)",
		"CreateTestMycat(svc)",
		"UpdateTestMycat(svc)",
		"DeleteTestMycat(svc)",
		"repository.NewTestMycatRepository",
		"service.NewTestMycatService",
	} {
		if !strings.Contains(mainText, want) {
			t.Errorf("main.go missing %q", want)
		}
	}

	modelSrc, _ := os.ReadFile(filepath.Join(root, "internal/model/test_mycat.go"))
	if !strings.Contains(string(modelSrc), "model.StandardModel") {
		t.Error("model template must include StandardModel")
	}
	repoSrc, _ := os.ReadFile(filepath.Join(root, "internal/repository/test_mycat.go"))
	if !strings.Contains(string(repoSrc), "datasource.WithTx") {
		t.Error("repository template must include WithTx transaction sample")
	}
	handlerSrc, _ := os.ReadFile(filepath.Join(root, "internal/handler/test_mycat.go"))
	handlerText := string(handlerSrc)
	for _, want := range []string{"webiris.OK", "webiris.Fail", "apperr.CodeRequestParamError"} {
		if !strings.Contains(handlerText, want) {
			t.Errorf("handler missing %q", want)
		}
	}
}