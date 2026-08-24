package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

// MongoTemplate 高频操作层(对标 Spring Data MongoDB 的 MongoTemplate):
// 在 Client 现有封装(Find/InsertOne/UpdateOne/DeleteOne/Aggregate)之上,
// 补充批量/计数操作,并暴露原生 *mongo.Client / *mongo.Collection
// 供开发者手动调用原生 API。
//
// 用法:
//
//	client, _ := mongodb.GetClient(config, ctx)
//	client.InsertMany(ctx, "orders", []interface{}{...})
//	count, _ := client.Count(ctx, "orders", bson.M{"status": "paid"})
//	// 需要原生 API 时:
//	raw := client.Client()                     // *mongo.Client
//	raw.Database("db").RunCommand(ctx, ...)
type MongoTemplate interface {
	// Client 返回底层原生 MongoDB 客户端。
	Client() *mongo.Client
	// Collection 返回原生集合句柄(手动操作入口)。
	Collection(name string) *mongo.Collection

	// InsertMany 批量插入文档;返回插入结果(含 IDs)。
	InsertMany(ctx context.Context, tableName string, documents []interface{}) (*mongo.InsertManyResult, error)
	// Count 统计匹配 filter 的文档数。
	Count(ctx context.Context, tableName string, filter interface{}) (int64, error)
	// UpdateMany 批量更新匹配 filter 的文档;返回更新结果。
	UpdateMany(ctx context.Context, tableName string, filter, update interface{}) (*mongo.UpdateResult, error)
}

// ---- 实现 ----

// Client 返回底层原生 MongoDB 客户端。
func (c *Client) Client() *mongo.Client {
	if c == nil {
		return nil
	}
	return c.mc
}

// Collection 返回原生集合句柄。
func (c *Client) Collection(name string) *mongo.Collection {
	return c.getCollection(name)
}

// InsertMany 批量插入。
func (c *Client) InsertMany(ctx context.Context, tableName string, documents []interface{}) (*mongo.InsertManyResult, error) {
	if len(documents) == 0 {
		return &mongo.InsertManyResult{}, nil
	}
	return c.getCollection(tableName).InsertMany(ctx, documents)
}

// Count 计数。
func (c *Client) Count(ctx context.Context, tableName string, filter interface{}) (int64, error) {
	if filter == nil {
		filter = map[string]interface{}{}
	}
	return c.getCollection(tableName).CountDocuments(ctx, filter)
}

// UpdateMany 批量更新。
func (c *Client) UpdateMany(ctx context.Context, tableName string, filter, update interface{}) (*mongo.UpdateResult, error) {
	return c.getCollection(tableName).UpdateMany(ctx, filter, update)
}
