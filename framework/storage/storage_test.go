package storage

import (
	"context"
	"os"
	"testing"
	"time"
)

// storageEndpoint 返回测试对象存储地址;未配置时跳过。
func storageEndpoint(t *testing.T) (string, string, string) {
	t.Helper()
	endpoint := os.Getenv("GO_BLACKBOX_STORAGE_ENDPOINT")
	accessKey := os.Getenv("GO_BLACKBOX_STORAGE_ACCESS_KEY")
	secretKey := os.Getenv("GO_BLACKBOX_STORAGE_SECRET_KEY")
	if endpoint == "" {
		t.Skip("storage not configured: set GO_BLACKBOX_STORAGE_ENDPOINT to run")
	}
	if accessKey == "" {
		accessKey = "minioadmin"
	}
	if secretKey == "" {
		secretKey = "minioadmin"
	}
	return endpoint, accessKey, secretKey
}

// testClient 连接测试对象存储。
func testClient(t *testing.T) *Client {
	t.Helper()
	endpoint, accessKey, secretKey := storageEndpoint(t)
	client, err := NewClient(Config{Endpoint: endpoint, AccessKey: accessKey, SecretKey: secretKey})
	if err != nil {
		t.Fatalf("new storage client failed: %v", err)
	}
	return client
}

// TestNewClientInvalid 非法配置报错。
func TestNewClientInvalid(t *testing.T) {
	if _, err := NewClient(Config{}); err == nil {
		t.Fatal("empty endpoint must fail")
	}
}

// TestPutGetDelete 上传/下载/删除闭环(需真实对象存储)。
func TestPutGetDelete(t *testing.T) {
	client := testClient(t)
	ctx := context.Background()

	bucket := "gbx-test"
	objectName := "dir/hello.txt"
	if err := client.EnsureBucket(ctx, bucket); err != nil {
		t.Fatalf("ensure bucket failed: %v", err)
	}

	content := []byte("hello go-blackbox storage")
	if err := client.PutBytes(ctx, bucket, objectName, content, "text/plain"); err != nil {
		t.Fatalf("put failed: %v", err)
	}
	downloaded, err := client.GetBytes(ctx, bucket, objectName)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if string(downloaded) != string(content) {
		t.Fatalf("content mismatch: %q", downloaded)
	}
	stat, err := client.Stat(ctx, bucket, objectName)
	if err != nil || stat.Size != int64(len(content)) {
		t.Fatalf("stat = %+v, %v", stat, err)
	}
	// 预签名
	url, err := client.PresignedGet(ctx, bucket, objectName, time.Minute)
	if err != nil || url == "" {
		t.Fatalf("presigned get = %q, %v", url, err)
	}
	// 列举
	objects, err := client.List(ctx, bucket, "dir/", 10)
	if err != nil || len(objects) < 1 {
		t.Fatalf("list = %+v, %v", objects, err)
	}
	// 删除
	if err := client.Delete(ctx, bucket, objectName); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, err := client.Stat(ctx, bucket, objectName); err == nil {
		t.Fatal("stat after delete must fail")
	}
}
