package mongodb

import (
	"context"
	"errors"
	"log"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

/*
MongoDB术语/概念说明对比SQL术语/概念
database	数据库		database
collection  集合			table
document	文档			row
field		字段			column
index		index		索引
primarykey	主键 MongoDB自动将_id字段设置为主键		primary key
*/

// Client 封装 MongoDB 客户端与默认数据库配置。
type Client struct {
	mc    *mongo.Client
	dbCig *MongoDBConfig
}

// GetClient 创建 MongoDB 客户端；配置或 Context 为空时返回明确错误。
// 调用方应使用带超时的 Context，并在使用完成后调用 Disconnect。
func GetClient(dbCig *MongoDBConfig, ctx context.Context) (*Client, error) {
	if dbCig == nil {
		return nil, errors.New("MongoDB config is nil")
	}
	if ctx == nil {
		return nil, errors.New("MongoDB context is nil")
	}
	uri := dbCig.GetApplyURI()
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

	if err != nil {
		return nil, err
	}
	client := &Client{mc: mc, dbCig: dbCig}
	return client, nil
}

// Ping 检查 MongoDB 主节点连通性。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.mc == nil {
		return errors.New("ping MongoDB: client is nil")
	}
	return c.mc.Ping(ctx, readpref.Primary())
}

// getCollection 表
func (c *Client) getCollection(tableName string) *mongo.Collection {
	return c.mc.Database(c.dbCig.DB).Collection(tableName)
}

// Aggregate 执行聚合管道并返回全部结果，游标统一关闭并处理关闭错误。
func (c *Client) Aggregate(ctx context.Context, tableName string, groupStage mongo.Pipeline) ([]bson.M, error) {
	cursor, err := c.getCollection(tableName).Aggregate(ctx, groupStage)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Printf("close MongoDB aggregate cursor failed: %v", closeErr)
		}
	}()
	// display the results
	results := []bson.M{}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// Find 按过滤条件查询文档集合；游标统一关闭并处理解码与遍历错误。
// 新代码建议使用类型安全的 FindTyped。
func (c *Client) Find(ctx context.Context, tableName string, filters ...interface{}) ([]bson.M, error) {
	var cursor *mongo.Cursor
	var err error
	if len(filters) == 0 {
		cursor, err = c.getCollection(tableName).Find(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
	} else {
		cursor, err = c.getCollection(tableName).Find(ctx, filters[0])
		if err != nil {
			return nil, err
		}
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Printf("close MongoDB find cursor failed: %v", closeErr)
		}
	}()
	var results []bson.M
	for cursor.Next(ctx) {
		b := bson.M{}
		if err := cursor.Decode(&b); err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// FindTyped 按过滤条件查询并直接解码为 []T，类型安全。
// filter 为 nil 时查询全部文档。
func FindTyped[T any](c *Client, ctx context.Context, tableName string, filter interface{}) ([]T, error) {
	if c == nil || c.mc == nil {
		return nil, errors.New("find typed: client is nil")
	}
	if ctx == nil {
		return nil, errors.New("find typed: context is nil")
	}
	if filter == nil {
		filter = bson.D{}
	}
	cursor, err := c.getCollection(tableName).Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			log.Printf("close MongoDB find typed cursor failed: %v", closeErr)
		}
	}()
	var results []T
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// FindOne 查询单条文档。
func (c *Client) FindOne(ctx context.Context, tableName string, filter interface{}) *mongo.SingleResult {
	return c.getCollection(tableName).FindOne(ctx, filter)
}

// Disconnect 关闭 MongoDB 客户端连接；重复调用安全。
func (c *Client) Disconnect(ctx context.Context) error {
	if c == nil || c.mc == nil {
		return errors.New("disconnect MongoDB: client is nil")
	}
	return c.mc.Disconnect(ctx)
}

// InsertOne 插入单条文档并返回插入 ID。
func (c *Client) InsertOne(ctx context.Context, tableName string, document interface{}) (interface{}, error) {
	cur, err := c.getCollection(tableName).InsertOne(ctx, document)
	if err != nil {
		return nil, err
	}
	return cur.InsertedID, nil
}

// DeleteOne 删除单条匹配文档。
func (c *Client) DeleteOne(ctx context.Context, tableName string, filter interface{}) error {
	_, err := c.getCollection(tableName).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

// UpdateOne 更新单条匹配文档。
func (c *Client) UpdateOne(ctx context.Context, tableName string, filter, update interface{}) (*mongo.UpdateResult, error) {
	return c.getCollection(tableName).UpdateOne(ctx, filter, update)
}
