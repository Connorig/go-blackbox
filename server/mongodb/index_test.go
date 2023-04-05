package mongodb

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"testing"
	"time"
)

var cong = MongoDBConfig{
	Timeout: 10,
	DB:      "admin",
	Addr:    "admin:admin@10.211.55.5:27017/",
}

func TestGetClient(t *testing.T) {

	//addr := "travis:test@127.0.0.1:27017/mongo_test"

	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb getClient", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
	})
}

func TestPing(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb ping", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		err = client.Ping(ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
	})
}

func TestInsertOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb InsertOne", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		res, err := client.InsertOne(ctx, "testing", bson.D{
			{Key: "name", Value: "pi"},
			{Key: "value", Value: 3.14159},
		})
		if err != nil {
			t.Error(err.Error())
			return
		}
		if res == "travis:test@127.0.0.1:27017/mongo_test" {
			t.Error("inserted id is empty")
		}
	})
}

func TestGetCollection(t *testing.T) {

	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb GetCollection", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		res := client.getCollection("testing")
		if res == nil {
			t.Error("Collection return empty")
		}
	})
}

func TestGetAggregate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb Aggregate", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
			return
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		pipeline := mongo.Pipeline{
			{
				{"$match", bson.D{
					{"items.fruit", "banana"},
				}},
			},
			{
				{"$sort", bson.D{
					{"date", 1},
				}},
			},
		}
		res, err := client.Aggregate(ctx, "testing", pipeline)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if res == nil {
			t.Error("Collection return empty")
		}
	})
}

func TestFind(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb Find", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
			return
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		res, err := client.Find(ctx, "testing", bson.D{{"end", nil}})
		if err != nil {
			t.Error(err.Error())
			return
		}
		if res == nil {
			t.Error("Collection return empty")
		}
		t.Log(res)
	})
}

func TestFindOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb FindOne", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
			return
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		var result bson.M

		res := client.FindOne(ctx, "testing", bson.D{{"end", nil}})
		if res == nil {
			t.Error("Collection return empty")
		}
		res.Decode(&result)
		t.Log(result)
	})
}

func TestDeleteOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb DeleteOne", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
			return
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		err = client.DeleteOne(ctx, "testing", bson.D{{"end", nil}})
		if err != nil {
			t.Error(err.Error())
			return
		}
	})
}

func TestUpdateOne(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), cong.Timeout*time.Second)

	defer cancel()
	t.Run("test mongodb UpdateOne", func(t *testing.T) {
		client, err := GetClient(&cong, ctx)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if client == nil {
			t.Error("mongodb clinet is nil")
			return
		}
		defer func() {
			if err = client.Disconnect(ctx); err != nil {
				t.Error(err)
			}
		}()
		id, err := client.InsertOne(ctx, "testing", bson.D{
			{Key: "name", Value: "pi"}, {Key: "value", Value: 3.14159},
		})
		if err != nil {
			t.Error(err.Error())
			return
		}
		b := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "name", Value: "pi"},
				{Key: "value", Value: 3.1415926},
			}},
		}
		res, err := client.UpdateOne(ctx, "testing", bson.D{{"_id", id}}, b)
		if err != nil {
			t.Error(err.Error())
			return
		}
		if res == nil {
			t.Error("Collection return empty")
		}
	})
}
