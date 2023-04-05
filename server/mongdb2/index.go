package mongdb2

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

func GetClient() {

	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	db := client.Database("admin") // 获取名为test的db
	collectionNames, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("collectionNames: ", collectionNames)
}

func GetCollection() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()

	db := client.Database("admin") // 获取名为test的db
	collection := db.Collection("person")
	fmt.Println("collection:", collection.Name())
}

func InsertOne() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	one, err := c.InsertOne(ctx, bson.M{"name": "祖国人", "gender": "男", "ranking": 1})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("id:%s", one.InsertedID)
	fmt.Println(one)
}

func InsertMany() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	// InsertMany
	docs := []interface{}{
		bson.M{"name": "二次元刀哥", "gender": "男", "level": 23},
		bson.M{"name": "小亮", "gender": "男", "level": 2},
	}
	// Ordered 设置为false表示其中一条插入失败不会影响其他文档的插入，默认为true，一条失败其他都不会被写入
	insertManyOptions := options.InsertMany().SetOrdered(false)
	insertManyResult, err := c.InsertMany(ctx, docs, insertManyOptions)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ids:", insertManyResult.InsertedIDs)

}

func Find() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	// SetSort 设置排序字段（1表示升序；-1表示降序）
	findOptions := options.Find().SetSort(bson.D{{"level", 1}})

	findCursor, err := c.Find(ctx, bson.M{"gender": "男"}, findOptions)

	if err != nil {
		log.Fatal(err)
	}
	var results []bson.M
	err = findCursor.All(ctx, &results)
	if err != nil {
		log.Fatal(err)
	}
	for _, result := range results {
		fmt.Println(result)
	}
}

func FindOne() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	var result bson.M
	// 按照level排序并跳过第一个, 且只需返回name、gender字段（SetProjection：1表示包含某些字段；0表示不包含某些字段）
	findOneOptions := options.FindOne().
		//SetSkip(1).
		SetSort(bson.D{{"level", 1}}).SetProjection(bson.D{{"name", 1}, {"gender", 1}})
	singleResult := c.FindOne(ctx, bson.M{"ranking": 1}, findOneOptions)
	err = singleResult.Decode(&result)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result)
}

func FindOneAndDelete() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	// FindOneAndDelete
	findOneAndDeleteOptions := options.FindOneAndDelete().SetProjection(bson.D{{"name", 1}, {"gender", 1}})
	var deleteDocs bson.M
	singleResult := c.FindOneAndDelete(ctx, bson.M{"name": "祖国人"}, findOneAndDeleteOptions)
	err = singleResult.Decode(&deleteDocs)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(deleteDocs)
}

func FindOneAndUpdate() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")
	// FindOneAndUpdate
	// 注意：返回的结果仍然是更新前的document
	_id, err := primitive.ObjectIDFromHex("642d453b3565020e6b6ba609") // 从一个十六进制字符串创建一个新的ObjectID
	if err != nil {
		log.Fatal(err)
	}
	findOneAndUpdateOptions := options.FindOneAndUpdate().SetUpsert(true)
	// "$set"：Field Update Operators之一。表示更新字段
	update := bson.M{"$set": bson.M{
		"level":  0,
		"gender": "男",
		"msg":    "新的field",
	}}
	var toUpdateDoc bson.M
	err = c.FindOneAndUpdate(ctx, bson.M{"_id": _id}, update, findOneAndUpdateOptions).Decode(&toUpdateDoc)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(toUpdateDoc)
}

func UpdateMany() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")
	// UpdateMany
	filter := bson.M{"level": 0}
	update := bson.M{"$set": bson.M{"level": 10}}
	updateResult, err := c.UpdateMany(ctx, filter, update)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("matched: %v  modified: %v  upserted: %v  upsertedID: %v\n",
		updateResult.MatchedCount,
		updateResult.ModifiedCount,
		updateResult.UpsertedCount,
		updateResult.UpsertedID)
	fmt.Println(updateResult)
}

func DeleteOne() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")
	// DeleteOne
	deleteOptions := options.Delete().SetCollation(&options.Collation{
		CaseLevel: false, // 忽略大小写
	})
	filter := bson.M{"name": "虎哥"}
	deleteResult, err := c.DeleteOne(ctx, filter, deleteOptions)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("delete count:", deleteResult.DeletedCount)
	fmt.Println(deleteResult)

}

func DeleteMany() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")

	// DeleteMany
	deleteOptions := options.Delete().SetCollation(&options.Collation{
		CaseLevel: false, // 忽略大小写
	})
	filter := bson.M{"level": 0}
	deleteResult, err := c.DeleteMany(ctx, filter, deleteOptions)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("delete count:", deleteResult.DeletedCount)
}

func Distinct() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")
	// Distinct
	distinctOptions := options.Distinct().SetMaxTime(time.Second * 2)
	// 返回所有的level值（注意是不同的结果，重复的level值不会重复输出）
	fieldName := "level"
	filter := bson.M{}
	distinctValues, err := c.Distinct(ctx, fieldName, filter, distinctOptions)
	for _, value := range distinctValues {
		fmt.Println(value)
	}
}

func Count() {
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelFunc()

	client, err := mongo.Connect(ctx, ClientOpts)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = client.Disconnect(context.TODO()); err != nil {
			log.Fatal(err)
		}
	}()
	c := client.Database("admin").Collection("person")
	// Count
	// EstimatedDocumentCount
	totalCount, err := c.EstimatedDocumentCount(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(totalCount)
	// CountDocuments
	filter := bson.M{"gender": "男"}
	count, err := c.CountDocuments(ctx, filter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(count)
}
