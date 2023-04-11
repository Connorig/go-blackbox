package appioc

import (
	"fmt"
	"github.com/Domingor/go-blackbox/server/cronjobs"
	"testing"
	"time"
)

/**
* @Author: Connor
* @Date:   23.3.29 11:05
* @Description:
 */

func TestDoOnce(t *testing.T) {
	instance := GetCronJobInstance()

	cronInstance := cronjobs.CronInstance()
	if instance != cronInstance {
		t.Error("err")
	}
	instance.AddFunc("@every 1s", func() {
		fmt.Println("func running...")
	})
	instance.Start()
	time.Sleep(time.Second * 10)
}
