package simpleioc

import (
	"fmt"
	"testing"

	"github.com/Connorig/go-blackbox/framework/cron"
)

/**
* @Author: Connor
* @Date:   23.3.28 14:37
* @Description: 兼容层测试
 */

// Stu 是兼容测试使用的结构体。
type Stu struct {
	Name string
	Age  int
}

// TestLegacySetGet 验证旧 Set/Get API 的指针一致性与缺失返回语义。
func TestLegacySetGet(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	stu1 := Stu{
		Name: "Homelander",
		Age:  18,
	}
	Set(&stu1)

	get := Get((*Stu)(nil))
	if get == nil {
		t.Fatal("legacy Get must return registered instance")
	}
	if get != &stu1 {
		t.Fatal("legacy Get must return the same pointer")
	}
	if get.Name != "Homelander" || get.Age != 18 {
		t.Fatalf("unexpected instance: %+v", get)
	}
}

// TestLegacyGetMissingReturnsOriginal 验证旧 Get 对未注册类型返回原参数值（旧语义）。
func TestLegacyGetMissingReturnsOriginal(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	var zero *Stu
	if got := Get(zero); got != nil {
		t.Fatalf("legacy Get must return nil for unregistered type, got %v", got)
	}
}

// TestLegacySetPanicsOnNonStruct 验证旧 Set 对非结构体指针保持 panic 语义。
func TestLegacySetPanicsOnNonStruct(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	defer func() {
		if recover() == nil {
			t.Fatal("legacy Set must panic for non-struct pointer")
		}
	}()
	value := 1
	Set(&value)
}

// TestLegacyGetDbReturnsNilWhenMissing 验证旧 GetDb 在未注册时返回 nil。
func TestLegacyGetDbReturnsNilWhenMissing(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	if db := GetDb(); db != nil {
		t.Fatalf("GetDb must return nil when database is not registered, got %v", db)
	}
}

// TestDoOnce 验证 IOC 容器与 cronjobs 包返回同一个 Cron 单例，且能注册任务。
// 不启动调度器、不固定 Sleep，避免测试依赖真实时间。
func TestDoOnce(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	cronInstance := cronjobs.CronInstance()
	if err := RegisterInstance(cronInstance); err != nil {
		t.Fatalf("register cron instance failed: %v", err)
	}

	instance := GetCronJobInstance()
	if instance != cronInstance {
		t.Fatal("cron instance mismatch between IOC container and cronjobs package")
	}
	if _, err := instance.AddFunc("@every 1s", func() {
		fmt.Println("func running...")
	}); err != nil {
		t.Fatalf("register cron func failed: %v", err)
	}
}
