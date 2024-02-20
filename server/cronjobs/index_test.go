package cronjobs

import (
	"fmt"
	"testing"
	"time"
)

type Jobs struct {
}

func (j Jobs) Run() {
	fmt.Println("job is running....")

}

func TestCronInstance(t *testing.T) {
	instance := CronInstance()
	t.Run("Test cron instance init", func(t *testing.T) {
		//if instance == nil {
		//	t.Error("Cron Instance init fail.")
		//}
		//instance1 := CronInstance()
		//if instance != instance1 {
		//	t.Error("Cron Instance is change.")
		//}

		// 使用对象调用定时任务
		jobs := Jobs{}
		//err := DoOnce(jobs)
		//t.Error(err)

		//CronInstance().AddJob("@every 3s", jobs)

		// 添加func调用定时任务
		//CronInstance().AddFunc("@every 1s", func() {
		//	//t.Log("func running in 1 sec")
		//	fmt.Println("func running in 1 sec....")
		//})

		DoOnce(jobs)
	})

	// 必须调用start方法
	instance.Start()
	time.Sleep(time.Second * 10)
}
