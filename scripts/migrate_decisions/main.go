package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"nofx/logger"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run main.go <数据库路径>")
		fmt.Println("示例: go run main.go ../config.db")
		os.Exit(1)
	}

	dbPath := os.Args[1]
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("数据库文件不存在: %s", dbPath)
	}

	// 打开数据库
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer db.Close()

	// 检查 decisions 字段是否存在
	var columnExists bool
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('decisions') 
		WHERE name = 'decisions'
	`).Scan(&columnExists)
	if err != nil {
		log.Fatalf("检查字段失败: %v", err)
	}

	if !columnExists {
		log.Println("添加 decisions 字段...")
		_, err = db.Exec(`ALTER TABLE decisions ADD COLUMN decisions TEXT DEFAULT ''`)
		if err != nil {
			log.Fatalf("添加字段失败: %v", err)
		}
		log.Println("✅ decisions 字段已添加")
	}

	// 查询所有需要迁移的记录（content 不为空且 decisions 为空）
	rows, err := db.Query(`
		SELECT id, content 
		FROM decisions 
		WHERE content != '' AND (decisions IS NULL OR decisions = '')
	`)
	if err != nil {
		log.Fatalf("查询记录失败: %v", err)
	}
	defer rows.Close()

	var migratedCount int
	var failedCount int

	for rows.Next() {
		var id int64
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Printf("⚠️  扫描记录失败: %v", err)
			failedCount++
			continue
		}

		// 解析 Content 中的 decisions
		var fullRecord logger.DecisionRecord
		if err := json.Unmarshal([]byte(content), &fullRecord); err != nil {
			log.Printf("⚠️  解析 Content 失败 (ID: %d): %v", id, err)
			failedCount++
			continue
		}

		// 序列化 decisions
		decisionsJSON, err := json.Marshal(fullRecord.Decisions)
		if err != nil {
			log.Printf("⚠️  序列化 decisions 失败 (ID: %d): %v", id, err)
			failedCount++
			continue
		}

		// 更新数据库
		_, err = db.Exec(`
			UPDATE decisions 
			SET decisions = ? 
			WHERE id = ?
		`, string(decisionsJSON), id)
		if err != nil {
			log.Printf("⚠️  更新记录失败 (ID: %d): %v", id, err)
			failedCount++
			continue
		}

		migratedCount++
		if migratedCount%100 == 0 {
			log.Printf("已迁移 %d 条记录...", migratedCount)
		}
	}

	log.Printf("✅ 迁移完成: 成功 %d 条, 失败 %d 条", migratedCount, failedCount)

	// 询问是否删除 content 字段（可选）
	fmt.Print("\n是否删除 content 字段？(y/N): ")
	var answer string
	fmt.Scanln(&answer)
	if answer == "y" || answer == "Y" {
		log.Println("⚠️  注意: SQLite 不支持直接删除列，需要重建表")
		log.Println("   建议保留 content 字段作为备份，或手动执行以下步骤：")
		log.Println("   1. 创建新表（不包含 content 字段）")
		log.Println("   2. 复制数据到新表")
		log.Println("   3. 删除旧表")
		log.Println("   4. 重命名新表")
		log.Println("   由于操作风险较高，请手动执行或使用数据库管理工具")
	}
}
