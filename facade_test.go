package appbox

import (
	"testing"

	"github.com/Connorig/go-blackbox/framework/cache"
	"github.com/Connorig/go-blackbox/framework/mongo"
)

// TestFacadeNilSafety 未启用模块时门面返回 nil 不 panic。
func TestFacadeNilSafety(t *testing.T) {
	// 全新环境(无任何装配,容器未注册)
	if DB() != nil {
		t.Fatal("DB must be nil before EnableDatabase")
	}
	if Cache() != nil {
		t.Fatal("Cache must be nil before EnableCache")
	}
	if MongoDB() != nil {
		t.Fatal("MongoDB must be nil before EnableMongoDB")
	}
	if MQ() != nil {
		t.Fatal("MQ must be nil")
	}
	if KafkaProducer() != nil || KafkaConsumer() != nil {
		t.Fatal("Kafka must be nil")
	}
	if Cron() != nil {
		t.Fatal("Cron must be nil")
	}
	if ES() != nil || Storage() != nil || Influx() != nil || SMS() != nil || Mail() != nil {
		t.Fatal("optional clients must be nil")
	}
	if Datasource() != nil || NamedDatasource("x") != nil {
		t.Fatal("datasource must be nil")
	}
}

// TestFacadeAfterSet 模块全局指针设置后门面可取到实例。
func TestFacadeAfterSet(t *testing.T) {
	cache.SetGlobal(&cache.RedisCache{})
	mongodb.SetGlobal(&mongodb.Client{})

	if Cache() == nil {
		t.Fatal("Cache must be available after SetGlobal")
	}
	if MongoDB() == nil {
		t.Fatal("MongoDB must be available after SetGlobal")
	}
	// 清理,避免影响其他测试
	cache.SetGlobal(nil)
	mongodb.SetGlobal(nil)
}
