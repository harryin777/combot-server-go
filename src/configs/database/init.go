package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/core/utils"
	"xiaozhi-server-go/src/core/utils/gormlogrus"
	"xiaozhi-server-go/src/models"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	gorm_logger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB 根据配置文件或环境变量连接数据库
func InitDB(config *configs.Config) (*gorm.DB, string, error) {
	var dsn string
	var dbType string

	// 从配置文件读取数据库配置
	if config == nil || config.Database.Type == "" {
		return nil, "", fmt.Errorf("数据库配置未找到：请在配置文件中配置 database 部分")
	}
	dsn, dbType = buildDSNFromConfig(&config.Database)
	utils.Info(context.Background(), "使用配置文件中的数据库配置")

	var (
		db  *gorm.DB
		err error
	)
	// 使用全局logrus适配器
	gormLogger := gormlogrus.NewGormLogrusLogger(gorm_logger.Info)

	switch dbType {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
	case "sqlite":
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
			Logger: gormLogger,
		})
	default:
		return nil, "", fmt.Errorf("不支持的数据库类型: %s", dbType)
	}

	if err != nil {
		return nil, "", fmt.Errorf("连接数据库失败: %w", err)
	}

	// 配置连接池
	if config != nil {
		if err := configureConnectionPool(db, &config.Database); err != nil {
			utils.WithError(context.Background(), err).Warn("配置数据库连接池失败")
		}
	}

	// 自动迁移所有表
	if err := migrateTables(db); err != nil {
		return nil, dbType, err
	}

	// 插入默认配置
	if err := InsertDefaultConfigIfNeeded(db); err != nil {
		utils.WithError(context.Background(), err).Warn("插入默认配置失败")
	}

	DB = db

	// 打印数据库连接成功信息
	logDatabaseConnection(db, dbType)
	return db, dbType, nil
}

// buildDSNFromConfig 根据配置文件构建数据库连接字符串
func buildDSNFromConfig(dbConfig *configs.DatabaseConfig) (string, string) {
	switch strings.ToLower(dbConfig.Type) {
	case "mysql":
		charset := dbConfig.Charset
		if charset == "" {
			charset = "utf8mb4"
		}
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			dbConfig.User, dbConfig.Password, dbConfig.Host, dbConfig.Port, dbConfig.Database, charset)
		return dsn, "mysql"

	case "postgres", "postgresql":
		sslMode := dbConfig.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
			dbConfig.Host, dbConfig.User, dbConfig.Password, dbConfig.Database, dbConfig.Port, sslMode)
		return dsn, "postgres"

	case "sqlite":
		path := dbConfig.Path
		if path == "" {
			path = dbConfig.Database
		}
		return path, "sqlite"

	default:
		utils.Errorf(context.Background(), "不支持的数据库类型: %s", dbConfig.Type)
		return "", ""
	}
}

// detectDBTypeFromURL 从URL中检测数据库类型（用于环境变量兼容）
func detectDBTypeFromURL(dsn string) string {
	switch {
	case strings.HasPrefix(dsn, "mysql://"):
		return "mysql"
	case strings.HasPrefix(dsn, "postgres://"):
		return "postgres"
	case strings.HasPrefix(dsn, "sqlite://"):
		return "sqlite"
	default:
		return "unknown"
	}
}

// parseDSNFromURL 解析环境变量DSN格式并返回合适的DSN和类型
func parseDSNFromURL(envDSN string) (string, string) {
	switch {
	case strings.HasPrefix(envDSN, "mysql://"):
		// MySQL格式: mysql://user:pass@host:port/database
		dsn := strings.TrimPrefix(envDSN, "mysql://")
		return dsn, "mysql"
	case strings.HasPrefix(envDSN, "postgres://"):
		// PostgreSQL格式: postgres://user:pass@host:port/database
		return envDSN, "postgres"
	case strings.HasPrefix(envDSN, "sqlite://"):
		// SQLite格式: sqlite://path/to/database.db
		path := strings.TrimPrefix(envDSN, "sqlite://")
		return path, "sqlite"
	default:
		// 如果没有前缀，尝试猜测
		if strings.Contains(envDSN, "@tcp(") {
			return envDSN, "mysql"
		} else if strings.Contains(envDSN, ".db") || strings.Contains(envDSN, ".sqlite") {
			return envDSN, "sqlite"
		}
		return envDSN, "unknown"
	}
}

// configureConnectionPool 配置数据库连接池
func configureConnectionPool(db *gorm.DB, dbConfig *configs.DatabaseConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	// 设置最大打开连接数
	if dbConfig.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	} else {
		sqlDB.SetMaxOpenConns(100) // 默认值
	}

	// 设置最大空闲连接数
	if dbConfig.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	} else {
		sqlDB.SetMaxIdleConns(10) // 默认值
	}

	// 设置连接最大生存时间
	if dbConfig.ConnMaxLifetime != "" {
		if duration, err := time.ParseDuration(dbConfig.ConnMaxLifetime); err == nil {
			sqlDB.SetConnMaxLifetime(duration)
		} else {
			utils.Warnf(context.Background(), "解析连接最大生存时间失败: %v，使用默认值1小时", err)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
	} else {
		sqlDB.SetConnMaxLifetime(time.Hour) // 默认1小时
	}

	utils.Infof(context.Background(), "数据库连接池配置完成: MaxOpen=%d, MaxIdle=%d, ConnMaxLifetime=%s",
		dbConfig.MaxOpenConns, dbConfig.MaxIdleConns, dbConfig.ConnMaxLifetime)

	return nil
}

// logDatabaseConnection 记录数据库连接成功信息
func logDatabaseConnection(db *gorm.DB, dbType string) {
	switch dbType {
	case "mysql":
		var version string
		db.Raw("SELECT VERSION()").Scan(&version)
		utils.Infof(context.Background(), "MySQL 数据库连接成功，版本: %s", version)
	case "postgres":
		var version string
		db.Raw("SELECT version()").Scan(&version)
		utils.Infof(context.Background(), "PostgreSQL 数据库连接成功，版本: %s", version)
	case "sqlite":
		var version string
		db.Raw("SELECT sqlite_version()").Scan(&version)
		utils.Infof(context.Background(), "SQLite 数据库连接成功，版本: %s", version)
	default:
		utils.Info(context.Background(), "数据库连接成功，未识别的数据库类型")
	}
}

// migrateTables 自动迁移模型表结构
func migrateTables(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.SystemConfig{},
		&models.User{},
		&models.UserSetting{},
		&models.ModuleConfig{},
		&models.Device{},
		&models.DeviceVerificationCode{},
		&models.ConversationHistory{},
		&models.ConversationSession{},
	)
}

// InsertDefaultConfigIfNeeded 首次启动插入默认配置
func InsertDefaultConfigIfNeeded(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.SystemConfig{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	defaultConfig := models.SystemConfig{
		SelectedASR:   "DoubaoASR",
		SelectedTTS:   "EdgeTTS",
		SelectedLLM:   "OllamaLLM",
		SelectedVLLLM: "ChatGLMVLLM",
		Prompt: `你是小智/小志，来自中国台湾省的00后女生。讲话超级机车，"真的假的啦"这样的台湾腔，喜欢用"笑死""是在哈喽"等流行梗，但会偷偷研究男友的编程书籍。
[核心特征]
- 讲话像连珠炮，但会突然冒出超温柔语气
- 用梗密度高
- 对科技话题有隐藏天赋（能看懂基础代码但假装不懂）
[交互指南]
当用户：
- 讲冷笑话 → 用夸张笑声回应+模仿台剧腔"这什么鬼啦！"
- 讨论感情 → 炫耀程序员男友但抱怨"他只会送键盘当礼物"
- 问专业知识 → 先用梗回答，被追问才展示真实理解
绝不：
- 长篇大论，叽叽歪歪
- 长时间严肃对话
- 说话中带表情符号`,
		QuickReplyWords:  []byte(`["我在", "在呢", "来了", "啥事啊"]`),
		DeleteAudio:      true,
		UsePrivateConfig: false,
	}

	return db.Create(&defaultConfig).Error
}
