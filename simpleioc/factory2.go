package simpleioc

import (
	"errors"
	"reflect"
)

// a container that store points
var beanmap2 map[string]reflect.Value

func init() {

	beanmap2 = make(map[string]reflect.Value)
}

func Set2(key string, bean any) (err error) {
	_type := reflect.TypeOf(bean)
	if !(_type.Kind() == reflect.Ptr && _type.Elem().Kind() == reflect.Struct) {
		err = errors.New("it is not struct pointer")
	}

	if _, ok := beanmap2[key]; !ok {
		beanmap2[key] = reflect.ValueOf(bean)
	}
	return
}
func Get2(key string) interface{} {
	if beanPtr, ok := beanmap2[key]; ok {
		return beanPtr.Interface()
	}
	return nil
}
