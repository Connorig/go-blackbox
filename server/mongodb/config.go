package mongodb

import (
	"time"

	"github.com/snowlyg/helper/str"
)

type MongoDBConfig struct {
	Timeout time.Duration `mapstructure:"timeout" json:"timeout" yaml:"timeout"`
	DB      string        `mapstructure:"db" json:"db" yaml:"db"`
	Addr    string        `mapstructure:"addr" json:"addr" yaml:"addr"`
}

func (md *MongoDBConfig) GetApplyURI() string {
	return str.Join("mongodb://", md.Addr)
}
