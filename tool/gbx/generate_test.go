package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunGeneratesProject 生成完整骨架并断言关键内容。
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

	// 关键内容:module 路径、项目名、端口
	mainSrc, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	mainText := string(mainSrc)
	for _, want := range []string{
		`EnableWeb(appbox.TimeFormat, ":8080"`,
		`EnableAdmin(":6060"`,
		"webiris.SQLGuard()",
		"monitor.Register(app, \"/monitor\"",
		"apptoken.SetSecretKey",
		"datasource.DriverSQLite",
	} {
		if !strings.Contains(mainText, want) {
			t.Errorf("main.go missing %q", want)
		}
	}

	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(goMod), "module github.com/example/demo-app") {
		t.Errorf("go.mod module wrong:\n%s", goMod)
	}
	if !strings.Contains(string(goMod), "github.com/Connorig/go-blackbox v1.13.0") {
		t.Errorf("go.mod missing go-blackbox dependency:\n%s", goMod)
	}

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
