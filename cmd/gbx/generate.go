package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// templateData 模板变量。
type templateData struct {
	ProjectName string // 项目名(目录名)
	ModulePath  string // Go module 路径
	Port        string // 业务端口
	AdminPort   string // Admin 端口
}

// styleTemplates 返回指定风格的模板集合:
//   - 风格模板:文件名以 <style>__ 开头(如 code__main.go.tmpl)
//   - 公共模板:不以 code__/config__ 开头(如 go.mod.tmpl、internal__model__user.go.tmpl)
func styleTemplates(style string) []string {
	if style == "" {
		style = "code"
	}
	prefix := style + "__"
	var result []string
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) || !hasStylePrefix(name) {
			result = append(result, name)
		}
	}
	return result
}

// hasStylePrefix 判断模板是否属于某风格专属(code__/config__/gen__)。
func hasStylePrefix(name string) bool {
	for _, prefix := range []string{"code__", "config__", "gen__"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// run 执行生成。
func run(opts options) error {
	if strings.TrimSpace(opts.name) == "" {
		return fmt.Errorf("project name is required: gbx new -name <name>")
	}
	if !validName(opts.name) {
		return fmt.Errorf("invalid project name %q: only letters, digits, '-' and '_' are allowed", opts.name)
	}
	style := opts.style
	if style == "" {
		style = "code"
	}
	if style != "code" && style != "config" && style != "gen" {
		return fmt.Errorf("invalid style %q: must be 'code', 'config' or 'gen'", style)
	}
	module := opts.module
	if module == "" {
		module = "github.com/" + opts.name + "/" + opts.name
	}
	targetDir := filepath.Join(opts.dir, opts.name)
	data := templateData{
		ProjectName: opts.name,
		ModulePath:  module,
		Port:        "8080",
		AdminPort:   "6060",
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	templates := styleTemplates(style)
	if len(templates) == 0 {
		return fmt.Errorf("no templates found for style %q", style)
	}
	generated := 0
	for _, sourceName := range templates {
		// 去掉 <style>__ 前缀,其余 __ 替换为路径分隔符
		destRel := sourceName
		for _, prefix := range []string{"code__", "config__", "gen__"} {
			if strings.HasPrefix(destRel, prefix) {
				destRel = strings.TrimPrefix(destRel, prefix)
				break
			}
		}
		destRel = strings.ReplaceAll(strings.TrimSuffix(destRel, ".tmpl"), "__", "/")
		if err := renderTemplate(sourceName, destRel, targetDir, data); err != nil {
			return err
		}
		generated++
	}

	fmt.Printf("✅ 项目骨架已生成:%s\n", targetDir)
	fmt.Printf("   项目名:  %s\n", data.ProjectName)
	fmt.Printf("   Module:  %s\n", data.ModulePath)
	fmt.Printf("   风格:    %s(%s)\n", style, styleDescription(style))
	fmt.Printf("   端口:    业务 %s / Admin %s\n", data.Port, data.AdminPort)
	fmt.Printf("   下一步:\n")
	fmt.Printf("     cd %s\n", targetDir)
	fmt.Printf("     go mod tidy\n")
	fmt.Printf("     go run .   → http://localhost:%s/health/live, 监控页 /monitor\n", data.Port)
	fmt.Printf("   (%d 个文件)\n", generated)
	return nil
}

// styleDescription 风格说明。
func styleDescription(style string) string {
	if style == "config" {
		return "配置驱动:config.toml [modules] 开关模块"
	}
	if style == "gen" {
		return "CRUD 全栈:测试表 test_mycat 完整增删改查(DDD 分层)"
	}
	return "代码式显式装配:builder.Enable* 链"
}

// renderTemplate 渲染单个模板到目标路径。
func renderTemplate(sourceName, destRel, targetDir string, data templateData) error {
	content, err := templateFS.ReadFile("templates/" + sourceName)
	if err != nil {
		return fmt.Errorf("read template %s: %w", sourceName, err)
	}
	tmpl, err := template.New(sourceName).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parse template %s: %w", sourceName, err)
	}
	dest := filepath.Join(targetDir, destRel)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", destRel, err)
	}
	file, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", destRel, err)
	}
	defer file.Close()
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("render %s: %w", destRel, err)
	}
	fmt.Printf("   ✓ %s\n", destRel)
	return nil
}

// validName 校验项目名(字母/数字/-/_ 开头为字母)。
func validName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for i, r := range name {
		ok := r == '-' || r == '_' ||
			(r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(i > 0 && r >= '0' && r <= '9')
		if !ok {
			return false
		}
	}
	return true
}
