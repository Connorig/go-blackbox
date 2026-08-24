package datasource

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

// TestGetDBUninitialized 验证未初始化时 GetDB 返回错误(而非 nil 传导)。
func TestGetDBUninitialized(t *testing.T) {
	// 保存现场
	previous := defaultInstance
	instancesMu.Lock()
	previousInstances := instances
	instances = map[string]*Instance{}
	instancesMu.Unlock()
	defer func() {
		instancesMu.Lock()
		instances = previousInstances
		defaultInstance = previous
		instancesMu.Unlock()
	}()

	db, err := GetDB()
	if err == nil {
		t.Fatal("GetDB must return error when default instance is not initialized")
	}
	if db != nil {
		t.Fatalf("GetDB must return nil db with error, got %v", db)
	}
}

// TestGetDBClosed 验证实例已关闭时 GetDB 返回错误(而非静默 nil → GORM panic)。
func TestGetDBClosed(t *testing.T) {
	previous := defaultInstance
	closedInstance := &Instance{closed: true}
	defaultInstance = closedInstance
	instancesMu.Lock()
	previousInstances := instances
	instances = map[string]*Instance{"": closedInstance}
	instancesMu.Unlock()
	defer func() {
		instancesMu.Lock()
		instances = previousInstances
		defaultInstance = previous
		instancesMu.Unlock()
	}()

	db, err := GetDB()
	if err == nil {
		t.Fatal("GetDB must return error when instance is closed")
	}
	if db != nil {
		t.Fatalf("GetDB must return nil db with error, got %v", db)
	}
}

// TestGetNamedDBUnknown 验证未知名实例返回明确错误。
func TestGetNamedDBUnknown(t *testing.T) {
	db, err := GetNamedDB("no-such-instance")
	if err == nil {
		t.Fatal("GetNamedDB must return error for unknown instance")
	}
	if db != nil {
		t.Fatalf("GetNamedDB must return nil db with error, got %v", db)
	}
}

// TestGetDBErrorCarriesInstanceError 验证 GetDB 包装 Get 的错误(可被 errors.Is 识别)。
func TestGetDBErrorCarriesInstanceError(t *testing.T) {
	previous := defaultInstance
	defaultInstance = nil
	instancesMu.Lock()
	previousInstances := instances
	instances = map[string]*Instance{}
	instancesMu.Unlock()
	defer func() {
		instancesMu.Lock()
		instances = previousInstances
		defaultInstance = previous
		instancesMu.Unlock()
	}()

	_, err := GetDB()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errNotInitialized) {
		t.Fatalf("error must wrap sentinel errNotInitialized, got %v", err)
	}
}

// TestGetDBHealthyInstance 验证正常实例返回 GORM 句柄。
func TestGetDBHealthyInstance(t *testing.T) {
	previous := defaultInstance
	healthy := &Instance{db: &gorm.DB{}}
	defaultInstance = healthy
	instancesMu.Lock()
	previousInstances := instances
	instances = map[string]*Instance{"": healthy}
	instancesMu.Unlock()
	defer func() {
		instancesMu.Lock()
		instances = previousInstances
		defaultInstance = previous
		instancesMu.Unlock()
	}()

	db, err := GetDB()
	if err != nil {
		t.Fatalf("GetDB must succeed on healthy instance: %v", err)
	}
	if db == nil {
		t.Fatal("GetDB must return non-nil db")
	}
}
