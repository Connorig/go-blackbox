package etc

import (
	"context"
	"github.com/Domingor/go-blackbox/server/cache"
	"gorm.io/gorm"
	"reflect"
)

/**
* @Author: Connor
* @Date:   23.3.23 11:39
* @Description:
 */

var beanMap map[reflect.Type]reflect.Value

type GlobalContext struct {
	Ctx context.Context
}

func init() {
	beanMap = make(map[reflect.Type]reflect.Value)
	background := context.Background()
	Set(&GlobalContext{Ctx: background})
}

func Set(beans ...any) {
	for i := range beans {
		_type := reflect.TypeOf(beans[i])
		if !(_type.Kind() == reflect.Ptr && _type.Elem().Kind() == reflect.Struct) {
			panic("it is not struct pointer")
		}
		if _, ok := beanMap[reflect.ValueOf(beans[i]).Type()]; !ok {
			beanMap[reflect.ValueOf(beans[i]).Type()] = reflect.ValueOf(beans[i])
		}
	}
}

func Get[T any](bean T) T {
	if t := reflect.TypeOf(bean); !(t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct) {
		return bean
	}
	if beanPtr, ok := beanMap[reflect.TypeOf(bean)]; ok {
		return beanPtr.Interface().(T)
	}
	return bean
}

// 获取数据库实例
func GetDb() *gorm.DB {
	get := Get((*gorm.DB)(nil))
	return get
}

// 获取上下文
func GetContext() *GlobalContext {
	get := Get((*GlobalContext)(nil))
	return get
}

// 获取redis实例
func GetCache() cache.Rediser {
	get := Get((*cache.RedisCache)(nil))
	return get
}
