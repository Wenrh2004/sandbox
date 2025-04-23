package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	
	"github.com/Wenrh2004/sandbox/internal/task/infrastructure/repository/query"
	"github.com/Wenrh2004/sandbox/pkg/log"
	"github.com/Wenrh2004/sandbox/pkg/transaction"
	"github.com/Wenrh2004/sandbox/pkg/zapgorm2"
)

const ctxTxKey = "TxKey"

type Repository struct {
	db *gorm.DB
	// rdb    *redis.Client
	logger *log.Logger
}

func NewRepository(
	logger *log.Logger,
	db *gorm.DB,
// rdb *redis.Client,
) *Repository {
	query.SetDefault(db)
	return &Repository{
		db: db,
		// rdb:    rdb,
		logger: logger,
	}
}

func NewTransaction(r *Repository) transaction.Transaction {
	return r
}

func (r *Repository) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		ctx = context.WithValue(ctx, ctxTxKey, tx)
		return fn(ctx)
	})
}

func NewDB(conf *viper.Viper, l *log.Logger) *gorm.DB {
	var (
		db  *gorm.DB
		err error
	)
	
	logger := zapgorm2.New(l.Logger)
	driver := conf.GetString("app.data.db.driver")
	dsn := conf.GetString("app.data.db.dsn")
	
	// GORM doc: https://gorm.io/docs/connecting_to_the_database.html
	switch driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger,
		})
	case "postgres":
		db, err = gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		}), &gorm.Config{})
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	default:
		panic("unknown db driver")
	}
	if err != nil {
		panic(err)
	}
	db = db.Debug()
	
	// Connection Pool config
	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	
	return db
}

func NewRedis(conf *viper.Viper) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     conf.GetString("data.redis.addr"),
		Password: conf.GetString("data.redis.password"),
		DB:       conf.GetInt("data.redis.db"),
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(fmt.Sprintf("redis error: %s", err.Error()))
	}
	
	return rdb
}
