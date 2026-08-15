package cronjobs

import (
	"testing"

	"github.com/Connorig/go-blackbox/framework/log"
)

type Jobs struct {
	//..
}

func (j Jobs) Run() {
	zaplog.Logger.Info("job is now running....")
}

// TestCronInstance 验证 Cron 单例与一次性任务注册。
// 不启动调度器、不固定 Sleep，避免测试依赖真实时间。
func TestCronInstance(t *testing.T) {
	instance := CronInstance()

	jobs := Jobs{}
	if err := DoOnce(jobs); err != nil {
		t.Fatalf("register one-time job failed: %v", err)
	}

	entries := instance.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 cron entry after DoOnce, got %d", len(entries))
	}
}
