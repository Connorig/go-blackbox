// Package storage 提供对象存储集成(对标 Spring Cloud 的 OSS/对象存储抽象,
// 兼容 S3 协议的 MinIO/阿里云 OSS/腾讯云 COS 等):常用操作封装 + 原生客户端暴露。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Config 对象存储配置(兼容 S3 协议:MinIO/OSS/COS 等)。
type Config struct {
	// Endpoint 服务地址(如 127.0.0.1:9000 或 oss-cn-hangzhou.aliyuncs.com)。
	Endpoint string
	// AccessKey/SecretKey 访问密钥。
	AccessKey string
	SecretKey string
	// UseSSL 是否启用 HTTPS。
	UseSSL bool
	// Region 区域(OSS 必需,如 oss-cn-hangzhou)。
	Region string
	// Bucket 默认桶(可空,操作时显式指定)。
	Bucket string
}

// ObjectInfo 对象信息。
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
}

// StorageTemplate 高频操作层:上传/下载/删除/列举/签名 + 原生客户端。
type StorageTemplate interface {
	// Ping 连通性检查(列举默认桶或根)。
	Ping(ctx context.Context) error
	// EnsureBucket 确保桶存在(不存在则创建)。
	EnsureBucket(ctx context.Context, bucket string) error
	// Put 上传对象(reader 流式上传)。
	Put(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) error
	// PutBytes 上传字节数据。
	PutBytes(ctx context.Context, bucket, objectName string, data []byte, contentType string) error
	// Get 下载对象(返回读取流,调用方负责 Close)。
	Get(ctx context.Context, bucket, objectName string) (io.ReadCloser, *ObjectInfo, error)
	// GetBytes 下载对象为字节(小文件场景)。
	GetBytes(ctx context.Context, bucket, objectName string) ([]byte, error)
	// Delete 删除对象(不存在不报错)。
	Delete(ctx context.Context, bucket, objectName string) error
	// Stat 对象信息(不存在返回错误)。
	Stat(ctx context.Context, bucket, objectName string) (*ObjectInfo, error)
	// List 列举前缀下的对象(最多 limit 个,limit<=0 时全部)。
	List(ctx context.Context, bucket, prefix string, limit int) ([]ObjectInfo, error)
	// PresignedPut 生成预签名上传 URL(临时直传,防暴露密钥)。
	PresignedPut(ctx context.Context, bucket, objectName string, ttl time.Duration) (string, error)
	// PresignedGet 生成预签名下载 URL。
	PresignedGet(ctx context.Context, bucket, objectName string, ttl time.Duration) (string, error)
	// Client 返回原生 MinIO 客户端(高级操作入口)。
	Client() *minio.Client
}

// Client 是对象存储模板实现。
type Client struct {
	minio *minio.Client
	bucket string
}

// NewClient 创建对象存储客户端(兼容 S3 协议)。
// Endpoint 为空时返回错误。
func NewClient(config Config) (*Client, error) {
	if config.Endpoint == "" {
		return nil, errors.New("storage: endpoint is required")
	}
	minioClient, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: create client: %w", err)
	}
	client := &Client{minio: minioClient, bucket: config.Bucket}
	SetGlobal(client)
	return client, nil
}

// Ping 连通性检查。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.minio == nil {
		return errors.New("storage: client is nil")
	}
	bucket := c.bucket
	if bucket == "" {
		bucket = "gbx-ping-check"
	}
	_, err := c.minio.BucketExists(ctx, bucket)
	return err
}

// EnsureBucket 确保桶存在。
func (c *Client) EnsureBucket(ctx context.Context, bucket string) error {
	if bucket == "" {
		return errors.New("storage: bucket is required")
	}
	exists, err := c.minio.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		return c.minio.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// Put 上传对象。
func (c *Client) Put(ctx context.Context, bucket, objectName string, reader io.Reader, size int64, contentType string) error {
	if c == nil || c.minio == nil {
		return errors.New("storage: client is nil")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := c.minio.PutObject(ctx, bucket, objectName, reader, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// PutBytes 上传字节数据。
func (c *Client) PutBytes(ctx context.Context, bucket, objectName string, data []byte, contentType string) error {
	return c.Put(ctx, bucket, objectName, bytesReader(data), int64(len(data)), contentType)
}

// Get 下载对象。
func (c *Client) Get(ctx context.Context, bucket, objectName string) (io.ReadCloser, *ObjectInfo, error) {
	if c == nil || c.minio == nil {
		return nil, nil, errors.New("storage: client is nil")
	}
	object, err := c.minio.GetObject(ctx, bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, nil, err
	}
	stat, err := object.Stat()
	if err != nil {
		object.Close()
		return nil, nil, err
	}
	return object, toObjectInfo(stat), nil
}

// GetBytes 下载为字节(小文件)。
func (c *Client) GetBytes(ctx context.Context, bucket, objectName string) ([]byte, error) {
	object, _, err := c.Get(ctx, bucket, objectName)
	if err != nil {
		return nil, err
	}
	defer object.Close()
	return io.ReadAll(object)
}

// Delete 删除对象。
func (c *Client) Delete(ctx context.Context, bucket, objectName string) error {
	if c == nil || c.minio == nil {
		return errors.New("storage: client is nil")
	}
	return c.minio.RemoveObject(ctx, bucket, objectName, minio.RemoveObjectOptions{})
}

// Stat 对象信息。
func (c *Client) Stat(ctx context.Context, bucket, objectName string) (*ObjectInfo, error) {
	if c == nil || c.minio == nil {
		return nil, errors.New("storage: client is nil")
	}
	stat, err := c.minio.StatObject(ctx, bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}
	return toObjectInfo(stat), nil
}

// List 列举对象。
func (c *Client) List(ctx context.Context, bucket, prefix string, limit int) ([]ObjectInfo, error) {
	if c == nil || c.minio == nil {
		return nil, errors.New("storage: client is nil")
	}
	var result []ObjectInfo
	for object := range c.minio.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix}) {
		if object.Err != nil {
			return nil, object.Err
		}
		result = append(result, *toObjectInfo(object))
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

// PresignedPut 预签名上传 URL。
func (c *Client) PresignedPut(ctx context.Context, bucket, objectName string, ttl time.Duration) (string, error) {
	if c == nil || c.minio == nil {
		return "", errors.New("storage: client is nil")
	}
	url, err := c.minio.PresignedPutObject(ctx, bucket, objectName, ttl)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// PresignedGet 预签名下载 URL。
func (c *Client) PresignedGet(ctx context.Context, bucket, objectName string, ttl time.Duration) (string, error) {
	if c == nil || c.minio == nil {
		return "", errors.New("storage: client is nil")
	}
	url, err := c.minio.PresignedGetObject(ctx, bucket, objectName, ttl, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// Client 返回原生 MinIO 客户端。
func (c *Client) Client() *minio.Client {
	if c == nil {
		return nil
	}
	return c.minio
}

// toObjectInfo 转换 MinIO 对象信息。
func toObjectInfo(object minio.ObjectInfo) *ObjectInfo {
	return &ObjectInfo{
		Key:          object.Key,
		Size:         object.Size,
		ContentType:  object.ContentType,
		LastModified: object.LastModified,
		ETag:         object.ETag,
	}
}

// bytesReader 字节转 reader。
func bytesReader(data []byte) io.Reader {
	return &sliceReader{data: data}
}

// sliceReader 轻量字节读取器(避免 io.NopCloser 每次分配)。
type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) Read(buffer []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(buffer, r.data[r.pos:])
	r.pos += n
	return n, nil
}
