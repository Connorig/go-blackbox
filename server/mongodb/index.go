package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo/options"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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

type Client struct {
	mc    *mongo.Client
	dbCig *MongoDBConfig
}

// GetClient ctx, cancel := context.WithTimeout(context.Background(), CONFIG.Timeout*time.Second)
func GetClient(dbCig *MongoDBConfig, ctx context.Context) (*Client, error) {
	uri := dbCig.GetApplyURI()
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

	if err != nil {
		return nil, err
	}
	client := &Client{mc: mc, dbCig: dbCig}
	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.mc.Ping(ctx, readpref.Primary())
}

// getCollection 表
func (c *Client) getCollection(tableName string) *mongo.Collection {
	return c.mc.Database(c.dbCig.DB).Collection(tableName)
}

func (c *Client) Aggregate(ctx context.Context, tableName string, groupStage mongo.Pipeline) ([]bson.M, error) {
	// pass the stage into a pipeline
	// pass the pipeline as the second paramter in the Aggregate() method
	cursor, err := c.getCollection(tableName).Aggregate(ctx, groupStage)
	if err != nil {
		return nil, err
	}
	// display the results
	results := []bson.M{}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	return results, nil
}

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
	var results []bson.M
	for cursor.Next(ctx) {
		b := bson.M{}
		err := cursor.Decode(b)
		if err != nil {
			return nil, err
		}
		results = append(results, b)
	}
	return results, nil
}

func (c *Client) FindOne(ctx context.Context, tableName string, filter interface{}) *mongo.SingleResult {
	return c.getCollection(tableName).FindOne(ctx, filter)
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.mc.Disconnect(ctx)
}

func (c *Client) InsertOne(ctx context.Context, tableName string, filter interface{}) (interface{}, error) {
	cur, err := c.getCollection(tableName).InsertOne(ctx, filter)
	if err != nil {
		return nil, err
	}
	return cur.InsertedID, nil
}

func (c *Client) DeleteOne(ctx context.Context, tableName string, filter interface{}) error {
	_, err := c.getCollection(tableName).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) UpdateOne(ctx context.Context, tableName string, filter, update interface{}) (*mongo.UpdateResult, error) {
	return c.getCollection(tableName).UpdateOne(ctx, filter, update)
}
