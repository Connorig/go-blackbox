// Package upload 提供 multipart 文件上传封装:
// 大小/扩展名校验 + 存储联动(对象存储或本地目录)+ 统一文件信息返回。
// 场景:直播封面、头像、附件上传。
package upload

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/kataras/iris/v12"
)

// Config 上传配置。
type Config struct {
	// MaxSize 单文件大小上限(字节),0 表示不限制。
	MaxSize int64
	// AllowedExt 允许的扩展名(小写,如 ".jpg" ".png");空表示不限制。
	AllowedExt []string
}

// FileInfo 上传文件信息。
type FileInfo struct {
	// Name 原始文件名。
	Name string
	// Ext 扩展名(小写,含点)。
	Ext string
	// Size 文件大小(字节)。
	Size int64
	// Content 文件内容(小文件场景直接使用;大文件建议先落盘)。
	Content []byte
	// Header 原始 multipart 头。
	Header *multipart.FileHeader
}

// Parse 解析并校验 multipart 文件(formField 为表单字段名)。
// 校验失败返回错误(大小超限/扩展名不允许/字段缺失)。
func Parse(ctx iris.Context, formField string, config Config) (*FileInfo, error) {
	if ctx == nil {
		return nil, errors.New("upload: context is nil")
	}
	file, header, err := ctx.FormFile(formField)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, fmt.Errorf("upload: field %q is missing", formField)
		}
		return nil, fmt.Errorf("upload: read form field %q: %w", formField, err)
	}
	defer file.Close()
	if config.MaxSize > 0 && header.Size > config.MaxSize {
		return nil, fmt.Errorf("upload: file size %d exceeds limit %d", header.Size, config.MaxSize)
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if len(config.AllowedExt) > 0 && !containsString(config.AllowedExt, ext) {
		return nil, fmt.Errorf("upload: extension %q is not allowed", ext)
	}
	content, err := io.ReadAll(io.LimitReader(file, config.MaxSize+1))
	if err != nil {
		return nil, fmt.Errorf("upload: read file: %w", err)
	}
	if config.MaxSize > 0 && int64(len(content)) > config.MaxSize {
		return nil, fmt.Errorf("upload: file size %d exceeds limit %d", len(content), config.MaxSize)
	}
	return &FileInfo{
		Name:    header.Filename,
		Ext:     ext,
		Size:    int64(len(content)),
		Content: content,
		Header:  header,
	}, nil
}

// ParseRequest 从标准 net/http 请求解析 multipart 文件。
func ParseRequest(r *http.Request, formField string, config Config) (*FileInfo, error) {
	if r == nil {
		return nil, errors.New("upload: request is nil")
	}
	file, header, err := r.FormFile(formField)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, fmt.Errorf("upload: field %q is missing", formField)
		}
		return nil, fmt.Errorf("upload: read form field %q: %w", formField, err)
	}
	defer file.Close()
	if config.MaxSize > 0 && header.Size > config.MaxSize {
		return nil, fmt.Errorf("upload: file size %d exceeds limit %d", header.Size, config.MaxSize)
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if len(config.AllowedExt) > 0 && !containsString(config.AllowedExt, ext) {
		return nil, fmt.Errorf("upload: extension %q is not allowed", ext)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("upload: read file: %w", err)
	}
	if config.MaxSize > 0 && int64(len(content)) > config.MaxSize {
		return nil, fmt.Errorf("upload: file size %d exceeds limit %d", len(content), config.MaxSize)
	}
	return &FileInfo{
		Name:    header.Filename,
		Ext:     ext,
		Size:    int64(len(content)),
		Content: content,
		Header:  header,
	}, nil
}

// containsString 判断切片是否包含字符串。
func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
