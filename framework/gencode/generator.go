package gencode

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
)

// GeneratedFile 生成结果文件。
type GeneratedFile struct {
	Path    string `json:"path"`    // 相对项目路径(如 internal/model/test_mycat.go)
	Content string `json:"content"` // 文件内容
	Hash    string `json:"hash"`    // 内容哈希(覆盖比对用)
}

// GenResult 一次生成的输出。
type GenResult struct {
	Table     string          `json:"table"`
	Files     []GeneratedFile `json:"files"`
	RouteCode string          `json:"route_code"` // 路由注册代码段(复制进 main.go)
}

// modelName 表名转模型名(test_mycat → TestMycat)。
func modelName(tableName string) string {
	parts := strings.Split(strings.ToLower(tableName), "_")
	var builder strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		builder.WriteString(part[1:])
	}
	return builder.String()
}

// varName 模型名转变量名(TestMycat → testMycat)。
func varName(model string) string {
	if model == "" {
		return ""
	}
	return strings.ToLower(model[:1]) + model[1:]
}

// goType 数据库类型 → Go 类型。
func goType(column ColumnMeta) string {
	upper := strings.ToUpper(column.DataType)
	switch {
	case strings.Contains(upper, "INT"):
		if column.AutoIncr || column.PrimaryKey {
			return "int64"
		}
		return "int"
	case strings.Contains(upper, "BIGINT"):
		return "int64"
	case strings.Contains(upper, "DECIMAL"), strings.Contains(upper, "NUMERIC"),
		strings.Contains(upper, "FLOAT"), strings.Contains(upper, "DOUBLE"), strings.Contains(upper, "REAL"):
		return "float64"
	case strings.Contains(upper, "BOOL"):
		return "bool"
	case strings.Contains(upper, "TIME"), strings.Contains(upper, "DATE"):
		return "time.Time"
	case strings.Contains(upper, "BLOB"), strings.Contains(upper, "BYTEA"):
		return "[]byte"
	default:
		return "string"
	}
}

// goColumnTag 生成 GORM tag。
func goColumnTag(column ColumnMeta) string {
	var parts []string
	parts = append(parts, "column:"+column.Name)
	if column.PrimaryKey {
		parts = append(parts, "primarykey")
	}
	if column.AutoIncr {
		parts = append(parts, "autoIncrement")
	}
	if column.Length > 0 {
		parts = append(parts, fmt.Sprintf("size:%d", column.Length))
	}
	if !column.Nullable {
		parts = append(parts, "not null")
	}
	if column.Default != "" && !column.AutoIncr {
		parts = append(parts, "default:"+column.Default)
	}
	return "`gorm:\"" + strings.Join(parts, ";") + "\"`"
}

// genData 模板数据。
type genData struct {
	Table      TableMeta
	ModelName  string
	VarName    string
	RoutePath  string
	ModulePath string
	GoFields   []genField
}

// genField 渲染用字段。
type genField struct {
	Name    string // Go 字段名
	Column  string // 列名
	GoType  string // Go 类型
	Tag     string // GORM tag
	Comment string
}

// buildFields 构建渲染字段(排除基础字段 id/gmt_create/gmt_modified/is_deleted)。
func buildFields(table TableMeta) []genField {
	base := map[string]bool{"id": true, "gmt_create": true, "gmt_modified": true, "is_deleted": true}
	var fields []genField
	for _, column := range table.Columns {
		if base[strings.ToLower(column.Name)] {
			continue
		}
		fields = append(fields, genField{
			Name:    modelName(column.Name),
			Column:  column.Name,
			GoType:  goType(column),
			Tag:     goColumnTag(column),
			Comment: column.Comment,
		})
	}
	return fields
}

// Generate 从表元数据生成 DDD 代码(五件套 + 路由片段)。
// modulePath 用于内部包 import 路径(如 github.com/example/app)。
func Generate(table TableMeta, modulePath string) (*GenResult, error) {
	data := genData{
		Table:      table,
		ModelName:  modelName(table.Name),
		VarName:    varName(modelName(table.Name)),
		RoutePath:  "/api/v1/" + strings.ReplaceAll(table.Name, "_", "-"),
		ModulePath: modulePath,
		GoFields:   buildFields(table),
	}

	render := func(tmplText, name string) (string, error) {
		tmpl, err := template.New(name).Parse(tmplText)
		if err != nil {
			return "", fmt.Errorf("gencode: parse template %s: %w", name, err)
		}
		var builder strings.Builder
		if err := tmpl.Execute(&builder, data); err != nil {
			return "", fmt.Errorf("gencode: render %s: %w", name, err)
		}
		return builder.String(), nil
	}

	result := &GenResult{Table: table.Name}
	files := map[string]string{
		"internal/model/" + table.Name + ".go":      tmplModel,
		"internal/filter/" + table.Name + ".go":     tmplFilter,
		"internal/repository/" + table.Name + ".go": tmplRepository,
		"internal/service/" + table.Name + ".go":    tmplService,
		"internal/handler/" + table.Name + ".go":    tmplHandler,
	}
	for path, tmplText := range files {
		content, err := render(tmplText, path)
		if err != nil {
			return nil, err
		}
		result.Files = append(result.Files, GeneratedFile{
			Path:    path,
			Content: content,
			Hash:    hashContent(content),
		})
	}
	route, err := render(tmplRoute, "route")
	if err != nil {
		return nil, err
	}
	result.RouteCode = route
	return result, nil
}

// hashContent 内容哈希。
func hashContent(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}
