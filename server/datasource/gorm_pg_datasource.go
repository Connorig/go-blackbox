package datasource

import (
	"fmt"
	"github.com/Domingor/go-blackbox/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
	_db      *gorm.DB        //全局db操作对象
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
	MaxIdleConns int
	MaxOpenConns int
}

// GormInit 初始化配置 pg连接信息、初始化model表信息
func GormInit(pg *PostgresConfig, models ...interface{}) {
	pgConfig = pg
	tables = models
	// 初始化
	GetDbInstance()
}

//GetDbInstance 多个协程在使用公用_db调用其他方法时，会从连接池中获取连接
func GetDbInstance() *gorm.DB {
	// 只执行一次，用于初始化_db
	once.Do(func() {
		fmt.Println("starting initializing...")
		err := gormPgSql(pgConfig)
		if err != nil {
			fmt.Printf("starting initialize failed %s \n", err)
		}
	})
	return _db
}

// 获取数据库连接
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

	// 打开数据库会话
	if _db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newLogger}); err != nil {
		fmt.Printf("open datasource is failed %v \n", err)
	}

	for i, item := range tables {
		if utils.IsNilFixed(item) {
			tables = append(tables[:i], tables[i+1:]...)
		}
	}

	if len(tables) > 0 {
		err = _db.AutoMigrate(tables...) // 初始化model 数据表
		if err != nil {
			fmt.Println("AutoMigrate tables failed ", err)
		}
	}

	sqlDB, _ := _db.DB() //设置数据库连接池参数

	sqlDB.SetMaxOpenConns(pgConfig.MaxOpenConns) // 设置数据库连接池最大连接数
	sqlDB.SetMaxIdleConns(pgConfig.MaxIdleConns) // 连接池最大允许的空闲连接数，如果没有sql任务需要执行的连接数大于20，超过的连接会被连接池关闭
	sqlDB.SetConnMaxLifetime(time.Hour)          // SetConnMaxLifetime 设置了连接可复用的最大时间。
	return
}
