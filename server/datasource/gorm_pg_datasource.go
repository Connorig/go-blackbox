package datasource

import (
	"fmt"
	"github.com/Domingor/go-blackbox/apputils/appassert"
	"github.com/Domingor/go-blackbox/server/zaplog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
	"log"
	"os"
	"sync"
	"time"
)

/**
* @Author: Connor
* @Date:   23.2.23 10:04
* @Description:
 */

var (
	once     sync.Once
	_db      *gorm.DB //全局db操作对象
	_error   error
	pgConfig *PostgresConfig // pg配置文件
	tables   []interface{}   // 初始化model
)

// PostgresConfig 配置文件对象
type PostgresConfig struct {
	UserName     string
	Password     string
	Host         string
	Port         int
	DbName       string
	InitDb       bool
	AliasName    string
	SSL          string
	MaxIdleConns int // 最大闲置连接数
	MaxOpenConns int // 最大连接数
}

// GormInit 初始化配置 pg连接信息、初始化model表信息
func GormInit(pg *PostgresConfig, models []interface{}) (err error) {
	pgConfig = pg
	tables = models
	// 初始化
	_, err = GetDbInstance()
	return
}

// GetDbInstance 多个协程在使用公用_db调用其他方法时，会从连接池中获取连接
func GetDbInstance() (*gorm.DB, error) {
	// 只执行一次，用于初始化_db
	once.Do(func() {
		zaplog.SugaredLogger.Info("db starting initializing...")
		_error = gormPgSql(pgConfig)
	})
	return _db, _error
}

// 初始化数据库连接
func gormPgSql(pgConfig *PostgresConfig) (err error) {
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, //Slow SQL threshold
			LogLevel:                  logger.Info, //Log level
			IgnoreRecordNotFoundError: true,        //Ignore ErrRecordNotFound error for logger
			Colorful:                  false,       //Disable color
		},
	)
	// DSN连接
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		pgConfig.Host, pgConfig.UserName, pgConfig.Password, pgConfig.DbName, pgConfig.Port, pgConfig.SSL)

	// 表名规则
	namingStrategy := schema.NamingStrategy{
		//TablePrefix:   "", // table name prefix, table for `User` would be `t_users`
		SingularTable: true, // use singular table name, table for `User` would be `user` with this option enabled
		//NoLowerCase:   true,                              // skip the snake_casing of names
		//NameReplacer:  strings.NewReplacer("CID", "Cid"), // use name replacer to change struct/field name before convert it to db name
	}

	// 打开数据库会话
	if _db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newLogger, NamingStrategy: namingStrategy}); err != nil {
		zaplog.SugaredLogger.Debugf("open datasource failed %v", err)
		return
	}

	// 过滤 nil结构体
	for i, item := range tables {
		if appassert.IsNilFixed(item) {
			tables = append(tables[:i], tables[i+1:]...)
		}
	}

	// 自动创建表
	if len(tables) > 0 {
		err = _db.AutoMigrate(tables...) // 初始化model 数据表
		if err != nil {
			zaplog.SugaredLogger.Debugf("AutoMigrate tables failed %v", err)
			return err
		}
	}

	sqlDB, _ := _db.DB() //设置数据库连接池参数

	sqlDB.SetMaxOpenConns(pgConfig.MaxOpenConns) // 设置数据库连接池最大连接数
	sqlDB.SetMaxIdleConns(pgConfig.MaxIdleConns) // 连接池最大允许的空闲连接数，如果没有sql任务需要执行的连接数大于20，超过的连接会被连接池关闭
	sqlDB.SetConnMaxLifetime(time.Hour)          // SetConnMaxLifetime 设置了连接可复用的最大时间。
	return
}
