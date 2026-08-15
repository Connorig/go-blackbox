package mongodb

import (
	"context"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// testMongoConfig 返回集成测试使用的 MongoDB 配置。
// 未设置 GO_BLACKBOX_MONGO_ADDR 时跳过测试，禁止在仓库中硬编码任何地址或凭据。
func testMongoConfig(t *testing.T) *MongoDBConfig {
	t.Helper()
	addr := os.Getenv("GO_BLACKBOX_MONGO_ADDR")
	if addr == "" {
		t.Skip("MongoDB integration test requires GO_BLACKBOX_MONGO_ADDR environment variable")
	}
	database := os.Getenv("GO_BLACKBOX_MONGO_DB")
	if database == "" {
		database = "admin"
	}
	return &MongoDBConfig{
		Timeout: 10,
		DB:      database,
		Addr:    addr,
	}
}

// testClient 建立带超时的测试客户端，并在测试结束时断开连接。
func testClient(t *testing.T) (*Client, context.Context, context.CancelFunc) {
	t.Helper()
	config := testMongoConfig(t)
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout*time.Second)
	client, err := GetClient(config, ctx)
	if err != nil {
		cancel()
		t.Fatalf("connect MongoDB failed: %v", err)
	}
	if client == nil {
		cancel()
		t.Fatal("MongoDB client is nil")
	}
	t.Cleanup(func() {
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer disconnectCancel()
		if err := client.Disconnect(disconnectCtx); err != nil {
			t.Errorf("disconnect MongoDB failed: %v", err)
		}
	})
	return client, ctx, cancel
}

func TestGetClient(t *testing.T) {
	client, _, cancel := testClient(t)
	defer cancel()

	if client.mc == nil {
		t.Fatal("underlying mongo client is nil")
	}
}

func TestPing(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping MongoDB failed: %v", err)
	}
}

func TestInsertOne(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	res, err := client.InsertOne(ctx, "testing", bson.D{
		{Key: "name", Value: "pi"},
		{Key: "value", Value: 3.14159},
	})
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	if res == nil {
		t.Fatal("inserted id is nil")
	}
}

func TestGetCollection(t *testing.T) {
	client, _, cancel := testClient(t)
	defer cancel()

	res := client.getCollection("testing")
	if res == nil {
		t.Fatal("collection is nil")
	}
}

func TestGetAggregate(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	pipeline := mongo.Pipeline{
		{
			{Key: "$match", Value: bson.D{
				{Key: "items.fruit", Value: "banana"},
			}},
		},
		{
			{Key: "$sort", Value: bson.D{
				{Key: "date", Value: 1},
			}},
		},
	}
	res, err := client.Aggregate(ctx, "testing", pipeline)
	if err != nil {
		t.Fatalf("aggregate failed: %v", err)
	}
	if res == nil {
		t.Fatal("aggregate result is nil")
	}
}

func TestFind(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	res, err := client.Find(ctx, "testing", bson.D{{Key: "end", Value: nil}})
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if res == nil {
		t.Fatal("find result is nil")
	}
	t.Log(res)
}

func TestFindOne(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	var result bson.M
	res := client.FindOne(ctx, "testing", bson.D{{Key: "end", Value: nil}})
	if res == nil {
		t.Fatal("find one result is nil")
	}
	if err := res.Decode(&result); err != nil {
		t.Fatalf("decode find one result failed: %v", err)
	}
	t.Log(result)
}

func TestDeleteOne(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	if err := client.DeleteOne(ctx, "testing", bson.D{{Key: "end", Value: nil}}); err != nil {
		t.Fatalf("delete one failed: %v", err)
	}
}

func TestUpdateOne(t *testing.T) {
	client, ctx, cancel := testClient(t)
	defer cancel()

	id, err := client.InsertOne(ctx, "testing", bson.D{
		{Key: "name", Value: "pi"}, {Key: "value", Value: 3.14159},
	})
	if err != nil {
		t.Fatalf("insert document failed: %v", err)
	}
	b := bson.D{
		{Key: "$set", Value: bson.D{
			{Key: "name", Value: "pi"},
			{Key: "value", Value: 3.1415926},
		}},
	}
	res, err := client.UpdateOne(ctx, "testing", bson.D{{Key: "_id", Value: id}}, b)
	if err != nil {
		t.Fatalf("update one failed: %v", err)
	}
	if res == nil {
		t.Fatal("update result is nil")
	}
}
