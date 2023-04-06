package mongdbdemo

import (
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"time"
)

// ClientOpts mongoClient 连接客户端参数
var ClientOpts = options.Client().
	SetAuth(options.Credential{
		AuthMechanism: "SCRAM-SHA-1",
		//AuthSource:        "anquan",    // 用于身份验证的数据库的名称
		Username: "admin",
		Password: "admin",
	}).
	SetConnectTimeout(time.Second * 10).
	SetHosts([]string{"10.211.55.5:27017"}).
	SetMaxPoolSize(20).
	SetMinPoolSize(5).
	SetReadPreference(readpref.Primary()). // 默认值是readpref.Primary()（https://www.mongodb.com/docs/manual/core/read-preference/#read-preference）
	SetReplicaSet("")                      // SetReplicaSet指定集群的副本集名称。（默认为空）
