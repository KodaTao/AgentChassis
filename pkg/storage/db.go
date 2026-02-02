// Package storage 提供数据存储功能
package storage

import (
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/KodaTao/AgentChassis/pkg/observability"
)

// DB 全局数据库实例
var DB *gorm.DB

// Config 数据库配置
type Config struct {
	Path string // 数据库文件路径
}

// InitDB 初始化数据库连接
func InitDB(cfg Config) error {
	// 处理路径中的 ~
	dbPath := expandPath(cfg.Path)

	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 配置 GORM 日志
	gormLogger := logger.Default.LogMode(logger.Silent)

	// 打开数据库连接
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return err
	}

	DB = db
	observability.Info("Database initialized", "path", dbPath)

	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	return DB
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(models ...any) error {
	if DB == nil {
		return ErrDBNotInitialized
	}
	return DB.AutoMigrate(models...)
}

// Close 关闭数据库连接
func Close() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// expandPath 展开路径中的 ~ 为用户主目录
func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// 错误定义
var (
	ErrDBNotInitialized = &DBError{Message: "database not initialized"}
)

// DBError 数据库错误
type DBError struct {
	Message string
}

func (e *DBError) Error() string {
	return e.Message
}
