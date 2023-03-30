package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
	mc    *mongo.Client
	dbCig *MongoDBConfig
}

// GetClient ctx, cancel := context.WithTimeout(context.Background(), CONFIG.Timeout*time.Second)
func GetClient(dbCig *MongoDBConfig, ctx context.Context) (*Client, error) {
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(dbCig.GetApplyURI()))
	if err != nil {
		return nil, err
	}
	client := &Client{mc: mc, dbCig: dbCig}
	return client, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.mc.Ping(ctx, readpref.Primary())
}

// getCollection
func (c *Client) getCollection(name string) *mongo.Collection {
	return c.mc.Database(c.dbCig.DB).Collection(name)
}

func (c *Client) Aggregate(ctx context.Context, name string, groupStage mongo.Pipeline) ([]bson.M, error) {
	// pass the stage into a pipeline
	// pass the pipeline as the second paramter in the Aggregate() method
	cursor, err := c.getCollection(name).Aggregate(ctx, groupStage)
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

func (c *Client) Find(ctx context.Context, name string, filters ...interface{}) ([]bson.M, error) {
	var cursor *mongo.Cursor
	var err error
	if len(filters) == 0 {
		cursor, err = c.getCollection(name).Find(ctx, bson.D{})
		if err != nil {
			return nil, err
		}
	} else {
		cursor, err = c.getCollection(name).Find(ctx, filters[0])
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

func (c *Client) FindOne(ctx context.Context, name string, filter interface{}) *mongo.SingleResult {
	return c.getCollection(name).FindOne(ctx, filter)
}

func (c *Client) Disconnect(ctx context.Context) error {
	return c.mc.Disconnect(ctx)
}

func (c *Client) InsertOne(ctx context.Context, name string, filter interface{}) (interface{}, error) {
	cur, err := c.getCollection(name).InsertOne(ctx, filter)
	if err != nil {
		return nil, err
	}
	return cur.InsertedID, nil
}

func (c *Client) DeleteOne(ctx context.Context, name string, filter interface{}) error {
	_, err := c.getCollection(name).DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) UpdateOne(ctx context.Context, name string, filter, update interface{}) (*mongo.UpdateResult, error) {
	return c.getCollection(name).UpdateOne(ctx, filter, update)
}
