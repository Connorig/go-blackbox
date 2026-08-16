package cronjobs

import (
	"github.com/robfig/cron/v3"
)

// 全局便捷入口:InitCronJob 时自动设置,业务直接 cron.GetCron() 获取调度器。

var globalCron *cron.Cron

// SetCron 设置全局调度器(InitCronJob 时自动调用)。
func SetCron(instance *cron.Cron) { globalCron = instance }

// GetCron 获取全局调度器;未启用定时任务时返回 nil。
func GetCron() *cron.Cron { return globalCron }
