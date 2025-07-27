package main

import (
	"fmt"
	"os"
	"xiaozhi-server-go/src/configs"
	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"
)

func main() {
	// 初始化日志系统
	config := &configs.Config{}
	config.Log.LogLevel = "info"
	config.Log.LogDir = "./logs"
	config.Log.LogFile = "test.log"

	if err := utils.InitGlobalLogger(config); err != nil {
		fmt.Printf("初始化日志失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== 数据库配置测试 ===")

	// 测试1: 使用配置文件配置（SQLite）
	fmt.Println("\n1. 测试配置文件数据库配置 (SQLite)")
	config.Database = configs.DatabaseConfig{
		Type: "sqlite",
		Path: "./test.db",
	}

	db, dbType, err := database.InitDB(config)
	if err != nil {
		fmt.Printf("❌ 配置文件数据库连接失败: %v\n", err)
	} else {
		fmt.Printf("✅ 使用配置文件连接数据库成功: %s\n", dbType)
		db.Exec("DROP TABLE IF EXISTS test_table")
		if err := db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT)").Error; err == nil {
			fmt.Println("✅ 数据库操作测试成功")
		} else {
			fmt.Printf("❌ 数据库操作测试失败: %v\n", err)
		}
	}

	// 测试2: 使用环境变量配置
	fmt.Println("\n2. 测试环境变量数据库配置")
	os.Setenv("DATABASE_URL", "sqlite://./test_env.db")

	emptyConfig := &configs.Config{}
	db2, dbType2, err := database.InitDB(emptyConfig)
	if err != nil {
		fmt.Printf("❌ 环境变量数据库连接失败: %v\n", err)
	} else {
		fmt.Printf("✅ 使用环境变量连接数据库成功: %s\n", dbType2)
		if err := db2.Exec("CREATE TABLE IF NOT EXISTS env_test (id INTEGER PRIMARY KEY, name TEXT)").Error; err == nil {
			fmt.Println("✅ 环境变量数据库操作测试成功")
		} else {
			fmt.Printf("❌ 环境变量数据库操作测试失败: %v\n", err)
		}
	}

	// 测试3: 无配置错误测试
	fmt.Println("\n3. 测试无配置错误处理")
	os.Unsetenv("DATABASE_URL")
	emptyConfig2 := &configs.Config{}

	_, _, err = database.InitDB(emptyConfig2)
	if err != nil {
		fmt.Printf("✅ 无配置错误处理正确: %v\n", err)
	} else {
		fmt.Println("❌ 应该返回错误但没有")
	}

	// 清理测试文件
	os.Remove("./test.db")
	os.Remove("./test_env.db")

	fmt.Println("\n=== 测试完成 ===")

	// 关闭日志系统
	utils.CloseGlobalLogger()
}
