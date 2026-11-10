package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"patreonoj/internal/config"
	"patreonoj/internal/models"
	"patreonoj/internal/oss"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type seedUser struct {
	Username    string
	Password    string
	UUID        string
	Nickname    string
	Email       string
	Permissions string
	Status      string
	Roles       []string
}

type seedQuestion struct {
	QuestionID  string
	Title       string
	Content     string
	Difficulty  string
	TimeLimit   int
	MemoryLimit int
	Tags        []string
	Status      string
}

type seedTestCase struct {
	Input    string
	Output   string
	IsHidden bool
}

type seedSubmissionDef struct {
	ID         string
	User       string
	QuestionID string
	Language   string
	Status     string
	Correct    bool
	AgeHours   int
	RuntimeMs  int64
	MemoryKB   int64
	ErrorCode  string
	ErrorMsg   string
}

type seedSolvedDef struct {
	User       string
	QuestionID string
	AgeHours   int
}

type seedMasteryDef struct {
	User     string
	SkillKey string
	Mastery  float64
}

var seedBaseTime = time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

func main() {
	if err := config.InitConfig("config.yaml"); err != nil {
		log.Fatalf("init config failed: %v", err)
	}
	if err := models.InitDB(); err != nil {
		log.Fatalf("init db failed: %v", err)
	}
	if err := resetDatabase(models.DB); err != nil {
		log.Fatalf("reset db failed: %v", err)
	}
	if err := models.AutoMigrate(); err != nil {
		log.Fatalf("auto migrate failed: %v", err)
	}
	if err := models.InitOjOverView(); err != nil {
		log.Fatalf("init overview failed: %v", err)
	}

	ossClient, bucketName, err := initSeedOSS()
	if err != nil {
		log.Fatalf("init oss failed: %v", err)
	}

	if err := runSeed(models.DB, ossClient, bucketName); err != nil {
		log.Fatalf("seed failed: %v", err)
	}

	log.Println("seed finished")
}

func resetDatabase(db *gorm.DB) error {
	switch db.Dialector.Name() {
	case "mysql":
		return resetMySQLDatabase(db)
	case "sqlite":
		return resetSQLiteDatabase(db)
	default:
		return fmt.Errorf("unsupported database type: %s", db.Dialector.Name())
	}
}

func resetMySQLDatabase(db *gorm.DB) error {
	rows, err := db.Raw("SHOW TABLES").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		tables = append(tables, table)
	}
	if len(tables) == 0 {
		return nil
	}

	if err := db.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return err
	}
	defer db.Exec("SET FOREIGN_KEY_CHECKS = 1")

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return err
		}
	}
	return nil
}

func resetSQLiteDatabase(db *gorm.DB) error {
	rows, err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		tables = append(tables, table)
	}
	if len(tables) == 0 {
		return nil
	}

	if err := db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return err
	}
	defer db.Exec("PRAGMA foreign_keys = ON")

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return err
		}
	}
	return nil
}

func runSeed(db *gorm.DB, ossClient *oss.OSS, bucketName string) error {
	permissions, roles, err := seedAuthData(db)
	if err != nil {
		return err
	}
	if err := seedUsers(db, roles); err != nil {
		return err
	}

	tags, err := seedTags(db)
	if err != nil {
		return err
	}
	questions, err := seedQuestions(db, tags)
	if err != nil {
		return err
	}
	if err := seedTestCases(db, ossClient, bucketName, questions); err != nil {
		return err
	}
	if err := seedQuestionEdges(db, questions); err != nil {
		return err
	}
	if err := seedSubmissionsAndSolved(db, questions); err != nil {
		return err
	}
	if err := seedOverview(db); err != nil {
		return err
	}

	log.Printf("种子数据完成：权限=%d 角色=%d 用户=%d 标签=%d 题目=%d",
		len(permissions), len(roles), 4, len(tags), len(questions))
	log.Printf("演示账号：admin/admin123, alice/alice123, bob/bob123, carol/carol123")
	return nil
}
func initSeedOSS() (*oss.OSS, string, error) {
	client, err := oss.NewOSSClient(
		config.GlobalConfig.OSS.Address,
		config.GlobalConfig.OSS.PublicAddress,
		config.GlobalConfig.OSS.AccessKey,
		config.GlobalConfig.OSS.SecretKey,
	)
	if err != nil {
		return nil, "", err
	}

	bucketName := strings.TrimSpace(config.GlobalConfig.OSS.BucketName)
	if bucketName == "" {
		bucketName = "patreon-oj-cases"
	}
	if err := client.CreateBucket(context.Background(), bucketName); err != nil {
		return nil, "", err
	}

	return client, bucketName, nil
}

func seedAuthData(db *gorm.DB) (map[string]models.Permission, map[string]models.Role, error) {
	permDefs := []models.Permission{
		{Code: "admin", Description: "全局管理员权限"},
		{Code: "question.manage", Description: "题目管理权限"},
		{Code: "submission.manage", Description: "提交记录管理权限"},
	}
	perms := make(map[string]models.Permission, len(permDefs))
	for _, def := range permDefs {
		var p models.Permission
		err := db.Where("code = ?", def.Code).First(&p).Error
		switch {
		case err == nil:
			p.Description = def.Description
			if err := db.Save(&p).Error; err != nil {
				return nil, nil, err
			}
		case err == gorm.ErrRecordNotFound:
			p = def
			if err := db.Create(&p).Error; err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, err
		}
		perms[p.Code] = p
	}

	roleDefs := []struct {
		Name        string
		Description string
		PermCodes   []string
	}{
		{
			Name:        "admin",
			Description: "管理员",
			PermCodes:   []string{"admin", "question.manage", "submission.manage"},
		},
		{
			Name:        "student",
			Description: "普通学习者",
			PermCodes:   []string{},
		},
	}
	roles := make(map[string]models.Role, len(roleDefs))
	for _, def := range roleDefs {
		var role models.Role
		err := db.Where("name = ?", def.Name).First(&role).Error
		switch {
		case err == nil:
			role.Description = def.Description
			if err := db.Save(&role).Error; err != nil {
				return nil, nil, err
			}
		case err == gorm.ErrRecordNotFound:
			role = models.Role{Name: def.Name, Description: def.Description}
			if err := db.Create(&role).Error; err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, err
		}

		rolePerms := make([]models.Permission, 0, len(def.PermCodes))
		for _, code := range def.PermCodes {
			if p, ok := perms[code]; ok {
				rolePerms = append(rolePerms, p)
			}
		}
		if err := db.Model(&role).Association("Permissions").Replace(rolePerms); err != nil {
			return nil, nil, err
		}

		roles[role.Name] = role
	}
	return perms, roles, nil
}
func seedUsers(db *gorm.DB, roles map[string]models.Role) error {
	users := []seedUser{
		{
			Username:    "admin",
			Password:    "admin123",
			UUID:        "11111111-1111-1111-1111-111111111111",
			Nickname:    "系统管理员",
			Email:       "admin@patreonoj.local",
			Permissions: "admin",
			Status:      "active",
			Roles:       []string{"admin"},
		},
		{
			Username: "alice",
			Password: "alice123",
			UUID:     "22222222-2222-2222-2222-222222222222",
			Nickname: "小艾",
			Email:    "alice@patreonoj.local",
			Status:   "active",
			Roles:    []string{"student"},
		},
		{
			Username: "bob",
			Password: "bob123",
			UUID:     "33333333-3333-3333-3333-333333333333",
			Nickname: "小博",
			Email:    "bob@patreonoj.local",
			Status:   "active",
			Roles:    []string{"student"},
		},
		{
			Username: "carol",
			Password: "carol123",
			UUID:     "44444444-4444-4444-4444-444444444444",
			Nickname: "小卡",
			Email:    "carol@patreonoj.local",
			Status:   "active",
			Roles:    []string{"student"},
		},
	}

	for _, def := range users {
		var u models.User
		err := db.Where("username = ?", def.Username).First(&u).Error
		switch {
		case err == nil:
			u.Password = md5Hex(def.Password)
			u.UUID = def.UUID
			u.Nickname = def.Nickname
			u.Email = def.Email
			u.Permissions = def.Permissions
			u.Status = def.Status
			if err := db.Save(&u).Error; err != nil {
				return err
			}
		case err == gorm.ErrRecordNotFound:
			u = models.User{
				Username:    def.Username,
				Password:    md5Hex(def.Password),
				UUID:        def.UUID,
				Nickname:    def.Nickname,
				Email:       def.Email,
				Permissions: def.Permissions,
				Status:      def.Status,
			}
			if err := db.Create(&u).Error; err != nil {
				return err
			}
		default:
			return err
		}

		roleRecords := make([]models.Role, 0, len(def.Roles))
		for _, roleName := range def.Roles {
			if r, ok := roles[roleName]; ok {
				roleRecords = append(roleRecords, r)
			}
		}
		if err := db.Model(&u).Association("Roles").Replace(roleRecords); err != nil {
			return err
		}
	}

	return nil
}
func seedTags(db *gorm.DB) (map[string]models.Tag, error) {
	defs := []models.Tag{
		{Name: "array", Description: "数组"},
		{Name: "hash-table", Description: "哈希表"},
		{Name: "string", Description: "字符串"},
		{Name: "stack", Description: "栈"},
		{Name: "queue", Description: "队列"},
		{Name: "two-pointers", Description: "双指针"},
		{Name: "sliding-window", Description: "滑动窗口"},
		{Name: "sorting", Description: "排序"},
		{Name: "binary-search", Description: "二分查找"},
		{Name: "heap", Description: "堆 / 优先队列"},
		{Name: "quickselect", Description: "快速选择"},
		{Name: "linked-list", Description: "链表"},
		{Name: "divide-and-conquer", Description: "分治"},
		{Name: "dp", Description: "动态规划"},
		{Name: "state-transition", Description: "状态转移"},
		{Name: "linear-dp", Description: "线性 DP"},
		{Name: "string-dp", Description: "字符串 DP"},
		{Name: "knapsack", Description: "背包问题"},
		{Name: "sequence", Description: "序列问题"},
		{Name: "matrix", Description: "矩阵"},
		{Name: "graph", Description: "图论"},
		{Name: "graph-traversal", Description: "图遍历"},
		{Name: "topological-sort", Description: "拓扑排序"},
		{Name: "shortest-path", Description: "最短路径"},
		{Name: "union-find", Description: "并查集"},
		{Name: "mst", Description: "最小生成树"},
		{Name: "tree", Description: "树"},
		{Name: "binary-tree", Description: "二叉树"},
		{Name: "binary-search-tree", Description: "二叉搜索树"},
		{Name: "dfs", Description: "深度优先搜索"},
		{Name: "bfs", Description: "广度优先搜索"},
		{Name: "backtracking", Description: "回溯"},
		{Name: "bitmask", Description: "位运算 / 状态压缩"},
		{Name: "greedy", Description: "贪心"},
		{Name: "trie", Description: "字典树"},
		{Name: "design", Description: "数据结构设计"},
	}
	out := make(map[string]models.Tag, len(defs))
	for _, def := range defs {
		var tag models.Tag
		err := db.Where("name = ?", def.Name).First(&tag).Error
		switch {
		case err == nil:
			tag.Description = def.Description
			if err := db.Save(&tag).Error; err != nil {
				return nil, err
			}
		case err == gorm.ErrRecordNotFound:
			tag = def
			if err := db.Create(&tag).Error; err != nil {
				return nil, err
			}
		default:
			return nil, err
		}
		out[tag.Name] = tag
	}
	return out, nil
}

func seedQuestions(db *gorm.DB, tags map[string]models.Tag) (map[string]models.Question, error) {
	defs := []seedQuestion{
		{
			QuestionID:  "P1001",
			Title:       "数组-两数之和",
			Content:     "给定整数数组 nums 和目标值 target，请返回两个数的 1-based 下标，使它们的和等于 target。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "hash-table"},
			Status:      "published",
		},
		{
			QuestionID:  "P1002",
			Title:       "栈-有效括号",
			Content:     "给定只包含括号字符的字符串，判断括号是否按正确顺序闭合。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"string", "stack"},
			Status:      "published",
		},
		{
			QuestionID:  "P1003",
			Title:       "数组-合并区间",
			Content:     "给定若干闭区间，合并所有重叠区间并按起点升序输出。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "sorting"},
			Status:      "published",
		},
		{
			QuestionID:  "P1004",
			Title:       "字符串-无重复字符的最长子串",
			Content:     "给定字符串 s，求不含重复字符的最长连续子串长度。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"string", "sliding-window", "hash-table"},
			Status:      "published",
		},
		{
			QuestionID:  "P1005",
			Title:       "数组-接雨水",
			Content:     "给定柱状图高度数组，计算雨后能够接住的水量。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"array", "two-pointers", "stack"},
			Status:      "published",
		},
		{
			QuestionID:  "P1006",
			Title:       "字符串-最小覆盖子串",
			Content:     "给定字符串 s 和 t，返回 s 中覆盖 t 所有字符的最短子串。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"string", "sliding-window", "hash-table"},
			Status:      "published",
		},
		{
			QuestionID:  "P1007",
			Title:       "栈-字符串解码",
			Content:     "给定形如 k[encoded_string] 的编码字符串，返回解码后的字符串。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"string", "stack"},
			Status:      "published",
		},
		{
			QuestionID:  "P1101",
			Title:       "二分-二分查找",
			Content:     "给定升序数组 nums 和目标值 target，返回 target 的 0-based 下标，不存在返回 -1。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "binary-search"},
			Status:      "published",
		},
		{
			QuestionID:  "P1102",
			Title:       "二分-搜索插入位置",
			Content:     "给定升序数组和目标值，返回目标值应插入的位置。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "binary-search"},
			Status:      "published",
		},
		{
			QuestionID:  "P1103",
			Title:       "二分-搜索旋转排序数组",
			Content:     "给定旋转后的升序数组和目标值，返回目标值下标，不存在返回 -1。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "binary-search"},
			Status:      "published",
		},
		{
			QuestionID:  "P1104",
			Title:       "堆-前 K 个高频元素",
			Content:     "给定整数数组和整数 k，返回出现频率最高的 k 个元素，按频率从高到低输出。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "hash-table", "heap"},
			Status:      "published",
		},
		{
			QuestionID:  "P1105",
			Title:       "堆-数组中的第 K 大元素",
			Content:     "给定整数数组和整数 k，返回数组中第 k 大的元素。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"array", "heap", "quickselect"},
			Status:      "published",
		},
		{
			QuestionID:  "P1106",
			Title:       "链表-排序链表",
			Content:     "给定链表节点序列，请在 O(n log n) 时间内按升序排序并输出。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"linked-list", "sorting", "divide-and-conquer"},
			Status:      "published",
		},
		{
			QuestionID:  "P1107",
			Title:       "链表-合并 K 个有序链表",
			Content:     "给定 k 个升序链表，合并成一个升序链表并输出所有节点值。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"linked-list", "heap", "divide-and-conquer"},
			Status:      "published",
		},
		{
			QuestionID:  "P2201",
			Title:       "DP-最长递增子序列",
			Content:     "给定整数数组，返回最长严格递增子序列的长度。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "binary-search", "sequence"},
			Status:      "published",
		},
		{
			QuestionID:  "P3201",
			Title:       "图论-被围绕的区域",
			Content:     "给定由 X 和 O 组成的矩阵，将所有被 X 完全围绕的 O 修改为 X。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "graph-traversal", "matrix", "union-find"},
			Status:      "published",
		},
		{
			QuestionID:  "P4001",
			Title:       "树-二叉树最大深度",
			Content:     "给定二叉树的层序序列，返回二叉树的最大深度，空节点用 null 表示。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "dfs", "bfs"},
			Status:      "published",
		},
		{
			QuestionID:  "P4002",
			Title:       "树-二叉树层序遍历",
			Content:     "给定二叉树的层序序列，按层输出节点值。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "bfs", "queue"},
			Status:      "published",
		},
		{
			QuestionID:  "P4003",
			Title:       "树-验证二叉搜索树",
			Content:     "给定二叉树层序序列，判断它是否是一棵有效二叉搜索树。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "dfs", "binary-search-tree"},
			Status:      "published",
		},
		{
			QuestionID:  "P4004",
			Title:       "树-二叉树最近公共祖先",
			Content:     "给定二叉树层序序列和两个节点值，返回它们最近公共祖先的节点值。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "dfs", "binary-tree"},
			Status:      "published",
		},
		{
			QuestionID:  "P4005",
			Title:       "树-路径总和",
			Content:     "给定二叉树和目标和，判断是否存在一条从根到叶子的路径，路径和等于目标值。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "dfs", "backtracking"},
			Status:      "published",
		},
		{
			QuestionID:  "P4006",
			Title:       "树-二叉树序列化",
			Content:     "给定二叉树层序序列，请输出去除末尾多余 null 后的规范层序序列。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"tree", "bfs", "design"},
			Status:      "published",
		},
		{
			QuestionID:  "P4007",
			Title:       "设计-实现 Trie",
			Content:     "实现字典树的 insert、search 和 startsWith 操作，并输出查询结果。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"trie", "design", "string"},
			Status:      "published",
		},
		{
			QuestionID:  "P5001",
			Title:       "贪心-买卖股票的最佳时机",
			Content:     "给定每天股票价格，只允许买卖一次，返回最大利润。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"greedy", "array", "dp"},
			Status:      "published",
		},
		{
			QuestionID:  "P5002",
			Title:       "贪心-跳跃游戏",
			Content:     "给定非负整数数组，每个元素表示最大跳跃长度，判断能否到达最后一个位置。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"greedy", "array"},
			Status:      "published",
		},
		{
			QuestionID:  "P5003",
			Title:       "贪心-加油站",
			Content:     "给定每个加油站的油量和到下一站的耗油量，返回能绕行一圈的起点下标，不存在返回 -1。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"greedy", "array"},
			Status:      "published",
		},
		{
			QuestionID:  "P5004",
			Title:       "回溯-全排列",
			Content:     "给定不含重复数字的数组，返回所有可能全排列的数量。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"backtracking", "array"},
			Status:      "published",
		},
		{
			QuestionID:  "P5005",
			Title:       "回溯-子集",
			Content:     "给定不含重复数字的数组，返回所有子集的数量。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"backtracking", "bitmask", "array"},
			Status:      "published",
		},
		{
			QuestionID:  "P5006",
			Title:       "回溯-N 皇后",
			Content:     "给定整数 n，返回 n 皇后问题不同解决方案的数量。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"backtracking", "matrix"},
			Status:      "published",
		},
		{
			QuestionID:  "P5007",
			Title:       "贪心-任务调度器",
			Content:     "给定任务列表和冷却时间 n，返回完成所有任务所需的最短时间。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"greedy", "heap", "hash-table"},
			Status:      "published",
		},
		{
			QuestionID:  "P2001",
			Title:       "DP-爬楼梯",
			Content:     "给定整数 n，假设每次可以爬 1 或 2 级台阶，求到达第 n 级的方法总数。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "state-transition", "linear-dp"},
			Status:      "published",
		},
		{
			QuestionID:  "P2002",
			Title:       "DP-打家劫舍",
			Content:     "给定一排房屋金额，不能偷相邻房屋，求可偷得的最大金额。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "state-transition", "linear-dp"},
			Status:      "published",
		},
		{
			QuestionID:  "P2003",
			Title:       "DP-编辑距离",
			Content:     "给定两个字符串 word1 和 word2，返回将 word1 转换为 word2 所需的最少操作数。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "string-dp", "state-transition"},
			Status:      "published",
		},
		{
			QuestionID:  "P3001",
			Title:       "图论-岛屿数量",
			Content:     "给定 0/1 网格，统计由上下左右连通形成的岛屿数量。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "graph-traversal"},
			Status:      "published",
		},
		{
			QuestionID:  "P3002",
			Title:       "图论-课程表",
			Content:     "给定课程依赖关系，判断是否可以完成所有课程。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "topological-sort"},
			Status:      "published",
		},
		{
			QuestionID:  "P3003",
			Title:       "图论-网络延迟时间",
			Content:     "给定带权有向图，返回从起点到所有节点的最短到达时间最大值，不可达返回 -1。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "shortest-path"},
			Status:      "published",
		},
		{
			QuestionID:  "P2101",
			Title:       "DP-最大子数组和",
			Content:     "给定整数数组，求和最大的连续子数组的和。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "linear-dp"},
			Status:      "published",
		},
		{
			QuestionID:  "P2102",
			Title:       "DP-不同路径",
			Content:     "机器人从左上角走到右下角，每次只能向右或向下，求不同路径数量。",
			Difficulty:  "中等",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "state-transition"},
			Status:      "published",
		},
		{
			QuestionID:  "P2103",
			Title:       "DP-零钱兑换",
			Content:     "给定硬币面值和目标金额，求组成目标金额所需最少硬币数，不可达返回 -1。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"dp", "state-transition"},
			Status:      "published",
		},
		{
			QuestionID:  "P3101",
			Title:       "图论-省份数量",
			Content:     "给定城市连通矩阵，返回省份数量。",
			Difficulty:  "简单",
			TimeLimit:   2000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "union-find"},
			Status:      "published",
		},
		{
			QuestionID:  "P3102",
			Title:       "图论-连接所有点的最小费用",
			Content:     "给定平面点坐标，求连接所有点的最小总费用（最小生成树）。",
			Difficulty:  "中等",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "mst", "union-find"},
			Status:      "published",
		},
		{
			QuestionID:  "P3103",
			Title:       "图论-最短路径计数",
			Content:     "给定无向带权图，求从 1 到 n 的最短路径条数（对 1e9+7 取模）。",
			Difficulty:  "困难",
			TimeLimit:   3000,
			MemoryLimit: 256,
			Tags:        []string{"graph", "shortest-path"},
			Status:      "published",
		},
	}

	out := make(map[string]models.Question, len(defs))
	for _, def := range defs {
		tagTokens := append([]string{}, def.Tags...)
		sort.Strings(tagTokens)
		tagCSV := strings.Join(tagTokens, ",")

		var q models.Question
		err := db.Where("question_id = ?", def.QuestionID).First(&q).Error
		switch {
		case err == nil:
			q.Title = def.Title
			q.Content = def.Content
			q.Difficulty = def.Difficulty
			q.TimeLimit = def.TimeLimit
			q.MemoryLimit = def.MemoryLimit
			q.Tags = tagCSV
			q.Status = def.Status
			if err := db.Save(&q).Error; err != nil {
				return nil, err
			}
		case err == gorm.ErrRecordNotFound:
			q = models.Question{
				QuestionId:  def.QuestionID,
				Title:       def.Title,
				Content:     def.Content,
				Difficulty:  def.Difficulty,
				TimeLimit:   def.TimeLimit,
				MemoryLimit: def.MemoryLimit,
				Tags:        tagCSV,
				Status:      def.Status,
			}
			if err := db.Create(&q).Error; err != nil {
				return nil, err
			}
		default:
			return nil, err
		}

		questionTags := make([]models.Tag, 0, len(def.Tags))
		for _, name := range def.Tags {
			if t, ok := tags[name]; ok {
				questionTags = append(questionTags, t)
			}
		}
		if err := db.Model(&q).Association("Tag").Replace(questionTags); err != nil {
			return nil, err
		}

		out[q.QuestionId] = q
	}
	return out, nil
}

func seedTestCases(db *gorm.DB, ossClient *oss.OSS, bucketName string, questions map[string]models.Question) error {
	testDefs := map[string][]seedTestCase{
		"P1001": {
			{Input: "4 9\n2 7 11 15\n", Output: "1 2", IsHidden: false},
			{Input: "3 6\n3 2 4\n", Output: "2 3", IsHidden: true},
		},
		"P1002": {
			{Input: "()[]{}\n", Output: "true", IsHidden: false},
			{Input: "([)]\n", Output: "false", IsHidden: true},
		},
		"P1003": {
			{Input: "4\n1 3\n2 6\n8 10\n15 18\n", Output: "1 6\n8 10\n15 18", IsHidden: false},
			{Input: "2\n1 4\n4 5\n", Output: "1 5", IsHidden: true},
		},
		"P1004": {
			{Input: "abcabcbb\n", Output: "3", IsHidden: false},
			{Input: "bbbbb\n", Output: "1", IsHidden: true},
		},
		"P1005": {
			{Input: "12\n0 1 0 2 1 0 1 3 2 1 2 1\n", Output: "6", IsHidden: false},
			{Input: "6\n4 2 0 3 2 5\n", Output: "9", IsHidden: true},
		},
		"P1006": {
			{Input: "ADOBECODEBANC\nABC\n", Output: "BANC", IsHidden: false},
			{Input: "aa\naa\n", Output: "aa", IsHidden: true},
		},
		"P1007": {
			{Input: "3[a]2[bc]\n", Output: "aaabcbc", IsHidden: false},
			{Input: "3[a2[c]]\n", Output: "accaccacc", IsHidden: true},
		},
		"P1101": {
			{Input: "6 9\n-1 0 3 5 9 12\n", Output: "4", IsHidden: false},
			{Input: "6 2\n-1 0 3 5 9 12\n", Output: "-1", IsHidden: true},
		},
		"P1102": {
			{Input: "4 5\n1 3 5 6\n", Output: "2", IsHidden: false},
			{Input: "4 2\n1 3 5 6\n", Output: "1", IsHidden: true},
		},
		"P1103": {
			{Input: "7 0\n4 5 6 7 0 1 2\n", Output: "4", IsHidden: false},
			{Input: "7 3\n4 5 6 7 0 1 2\n", Output: "-1", IsHidden: true},
		},
		"P1104": {
			{Input: "6 2\n1 1 1 2 2 3\n", Output: "1 2", IsHidden: false},
			{Input: "1 1\n1\n", Output: "1", IsHidden: true},
		},
		"P1105": {
			{Input: "6 2\n3 2 1 5 6 4\n", Output: "5", IsHidden: false},
			{Input: "9 4\n3 2 3 1 2 4 5 5 6\n", Output: "4", IsHidden: true},
		},
		"P1106": {
			{Input: "4\n4 2 1 3\n", Output: "1 2 3 4", IsHidden: false},
			{Input: "5\n-1 5 3 4 0\n", Output: "-1 0 3 4 5", IsHidden: true},
		},
		"P1107": {
			{Input: "3\n3 1 4 5\n3 1 3 4\n2 2 6\n", Output: "1 1 2 3 4 4 5 6", IsHidden: false},
			{Input: "2\n1 1\n1 0\n", Output: "0 1", IsHidden: true},
		},
		"P2201": {
			{Input: "8\n10 9 2 5 3 7 101 18\n", Output: "4", IsHidden: false},
			{Input: "6\n0 1 0 3 2 3\n", Output: "4", IsHidden: true},
		},
		"P3201": {
			{Input: "4 4\nXXXX\nXOOX\nXXOX\nXOXX\n", Output: "XXXX\nXXXX\nXXXX\nXOXX", IsHidden: false},
			{Input: "1 1\nO\n", Output: "O", IsHidden: true},
		},
		"P4001": {
			{Input: "7\n3 9 20 null null 15 7\n", Output: "3", IsHidden: false},
			{Input: "0\n", Output: "0", IsHidden: true},
		},
		"P4002": {
			{Input: "7\n3 9 20 null null 15 7\n", Output: "3\n9 20\n15 7", IsHidden: false},
			{Input: "1\n1\n", Output: "1", IsHidden: true},
		},
		"P4003": {
			{Input: "3\n2 1 3\n", Output: "true", IsHidden: false},
			{Input: "7\n5 1 4 null null 3 6\n", Output: "false", IsHidden: true},
		},
		"P4004": {
			{Input: "7 5 1\n3 5 1 6 2 0 8\n", Output: "3", IsHidden: false},
			{Input: "7 6 2\n3 5 1 6 2 0 8\n", Output: "5", IsHidden: true},
		},
		"P4005": {
			{Input: "3 4\n1 2 3\n", Output: "true", IsHidden: false},
			{Input: "3 5\n1 2 3\n", Output: "false", IsHidden: true},
		},
		"P4006": {
			{Input: "7\n1 2 3 null null 4 5\n", Output: "1 2 3 null null 4 5", IsHidden: false},
			{Input: "1\n1\n", Output: "1", IsHidden: true},
		},
		"P4007": {
			{Input: "5\ninsert apple\nsearch apple\nsearch app\nstartsWith app\ninsert app\n", Output: "true\nfalse\ntrue", IsHidden: false},
			{Input: "2\ninsert code\nstartsWith co\n", Output: "true", IsHidden: true},
		},
		"P5001": {
			{Input: "6\n7 1 5 3 6 4\n", Output: "5", IsHidden: false},
			{Input: "5\n7 6 4 3 1\n", Output: "0", IsHidden: true},
		},
		"P5002": {
			{Input: "5\n2 3 1 1 4\n", Output: "true", IsHidden: false},
			{Input: "5\n3 2 1 0 4\n", Output: "false", IsHidden: true},
		},
		"P5003": {
			{Input: "5\n1 2 3 4 5\n3 4 5 1 2\n", Output: "3", IsHidden: false},
			{Input: "3\n2 3 4\n3 4 3\n", Output: "-1", IsHidden: true},
		},
		"P5004": {
			{Input: "3\n1 2 3\n", Output: "6", IsHidden: false},
			{Input: "1\n0\n", Output: "1", IsHidden: true},
		},
		"P5005": {
			{Input: "3\n1 2 3\n", Output: "8", IsHidden: false},
			{Input: "0\n", Output: "1", IsHidden: true},
		},
		"P5006": {
			{Input: "4\n", Output: "2", IsHidden: false},
			{Input: "1\n", Output: "1", IsHidden: true},
		},
		"P5007": {
			{Input: "6 2\nA A A B B B\n", Output: "8", IsHidden: false},
			{Input: "6 0\nA A A B B B\n", Output: "6", IsHidden: true},
		},
		"P2001": {
			{Input: "5\n", Output: "8", IsHidden: false},
			{Input: "3\n", Output: "3", IsHidden: true},
		},
		"P2002": {
			{Input: "5\n2 7 9 3 1\n", Output: "12", IsHidden: false},
			{Input: "4\n2 1 1 2\n", Output: "4", IsHidden: true},
		},
		"P2003": {
			{Input: "horse\nros\n", Output: "3", IsHidden: false},
			{Input: "intention\nexecution\n", Output: "5", IsHidden: true},
		},
		"P3001": {
			{Input: "4 5\n11000\n11000\n00100\n00011\n", Output: "3", IsHidden: false},
			{Input: "3 3\n111\n010\n111\n", Output: "1", IsHidden: true},
		},
		"P3002": {
			{Input: "2 1\n1 0\n", Output: "true", IsHidden: false},
			{Input: "2 2\n1 0\n0 1\n", Output: "false", IsHidden: true},
		},
		"P3003": {
			{Input: "4 3 2\n2 1 1\n2 3 1\n3 4 1\n", Output: "2", IsHidden: false},
			{Input: "3 1 1\n2 3 1\n", Output: "-1", IsHidden: true},
		},
		"P2101": {
			{Input: "9\n-2 1 -3 4 -1 2 1 -5 4\n", Output: "6", IsHidden: false},
			{Input: "1\n1\n", Output: "1", IsHidden: true},
		},
		"P2102": {
			{Input: "3 7\n", Output: "28", IsHidden: false},
			{Input: "3 2\n", Output: "3", IsHidden: true},
		},
		"P2103": {
			{Input: "3\n1 2 5\n11\n", Output: "3", IsHidden: false},
			{Input: "1\n2\n3\n", Output: "-1", IsHidden: true},
		},
		"P3101": {
			{Input: "3\n1 1 0\n1 1 0\n0 0 1\n", Output: "2", IsHidden: false},
			{Input: "3\n1 0 0\n0 1 0\n0 0 1\n", Output: "3", IsHidden: true},
		},
		"P3102": {
			{Input: "5\n0 0\n2 2\n3 10\n5 2\n7 0\n", Output: "20", IsHidden: false},
			{Input: "3\n3 12\n-2 5\n-4 1\n", Output: "18", IsHidden: true},
		},
		"P3103": {
			{Input: "7 10\n1 2 1\n1 3 1\n2 4 1\n3 4 1\n2 5 2\n3 5 2\n4 6 1\n5 6 1\n6 7 1\n5 7 3\n", Output: "2", IsHidden: false},
			{Input: "2 1\n1 2 5\n", Output: "1", IsHidden: true},
		},
	}

	for qid, defs := range testDefs {
		q, ok := questions[qid]
		if !ok {
			continue
		}
		if err := db.Where("question_id = ?", q.Id).Delete(&models.TestCase{}).Error; err != nil {
			return err
		}

		records := make([]models.TestCase, 0, len(defs))
		for idx, def := range defs {
			caseNo := idx + 1
			base := fmt.Sprintf("problems/%d/%d", q.Id, caseNo)
			inputKey := base + ".in"
			outputKey := base + ".out"

			inputBytes := []byte(def.Input)
			outputBytes := []byte(def.Output)

			if _, err := ossClient.UploadFile(
				context.Background(),
				bucketName,
				inputKey,
				bytes.NewReader(inputBytes),
				int64(len(inputBytes)),
				"text/plain; charset=utf-8",
			); err != nil {
				return err
			}
			if _, err := ossClient.UploadFile(
				context.Background(),
				bucketName,
				outputKey,
				bytes.NewReader(outputBytes),
				int64(len(outputBytes)),
				"text/plain; charset=utf-8",
			); err != nil {
				return err
			}

			records = append(records, models.TestCase{
				QuestionID: q.Id,
				InputKey:   inputKey,
				OutputKey:  outputKey,
				InputSize:  int64(len(inputBytes)),
				OutputSize: int64(len(outputBytes)),
				IsHidden:   def.IsHidden,
			})
		}
		if err := db.Create(&records).Error; err != nil {
			return err
		}
	}
	return nil
}

func seedQuestionEdges(db *gorm.DB, questions map[string]models.Question) error {
	edges := seedQuestionRelationEdges()

	for _, e := range edges {
		if _, ok := questions[e.FromQuestionID]; !ok {
			continue
		}
		if _, ok := questions[e.ToQuestionID]; !ok {
			continue
		}
		edge := e
		edge.CreatedAt = seedBaseTime
		edge.UpdatedAt = seedBaseTime
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "from_question_id"},
				{Name: "to_question_id"},
				{Name: "edge_type"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"updated_at": seedBaseTime,
			}),
		}).Create(&edge).Error; err != nil {
			return err
		}
	}

	return nil
}

func seedQuestionRelationEdges() []models.QuestionRelationEdge {
	paths := [][]string{
		{"P1001", "P1003", "P1004", "P1005", "P1006"},
		{"P1101", "P1102", "P1103", "P1105", "P1107"},
		{"P2001", "P2101", "P2002", "P2102", "P2103", "P2201", "P2003"},
		{"P3001", "P3101", "P3201", "P3002", "P3102", "P3003", "P3103"},
		{"P4001", "P4002", "P4003", "P4004", "P4005", "P4006", "P4007"},
		{"P5005", "P5004", "P5002", "P5003", "P5007", "P5006"},
	}

	edges := make([]models.QuestionRelationEdge, 0, 140)
	for _, path := range paths {
		edges = append(edges, pathEdges(path)...)
	}

	similarPairs := [][2]string{
		{"P1001", "P1104"},
		{"P1002", "P1007"},
		{"P1003", "P1106"},
		{"P1004", "P1006"},
		{"P1104", "P1105"},
		{"P1106", "P1107"},
		{"P2001", "P2102"},
		{"P2002", "P5001"},
		{"P2003", "P1006"},
		{"P2101", "P5001"},
		{"P2103", "P2201"},
		{"P3001", "P3201"},
		{"P3002", "P3103"},
		{"P3003", "P3103"},
		{"P3101", "P3102"},
		{"P4001", "P4002"},
		{"P4003", "P4004"},
		{"P4005", "P5004"},
		{"P4006", "P4007"},
		{"P5002", "P5003"},
		{"P5004", "P5005"},
		{"P5006", "P5007"},
	}
	for _, pair := range similarPairs {
		edges = append(edges, bidirectionalSimilarEdges(pair[0], pair[1])...)
	}

	return edges
}

func pathEdges(path []string) []models.QuestionRelationEdge {
	capacity := 0
	if len(path) > 1 {
		capacity = (len(path) - 1) * 2
	}
	edges := make([]models.QuestionRelationEdge, 0, capacity)
	for i := 0; i+1 < len(path); i++ {
		from := path[i]
		to := path[i+1]
		edges = append(edges,
			models.QuestionRelationEdge{FromQuestionID: from, ToQuestionID: to, EdgeType: models.QuestionRelationEdgeTypeNextLevel},
			models.QuestionRelationEdge{FromQuestionID: from, ToQuestionID: to, EdgeType: models.QuestionRelationEdgeTypePrerequisite},
		)
	}
	return edges
}

func bidirectionalSimilarEdges(a string, b string) []models.QuestionRelationEdge {
	return []models.QuestionRelationEdge{
		{FromQuestionID: a, ToQuestionID: b, EdgeType: models.QuestionRelationEdgeTypeSimilar},
		{FromQuestionID: b, ToQuestionID: a, EdgeType: models.QuestionRelationEdgeTypeSimilar},
	}
}

func seedSubmissionsAndSolved(db *gorm.DB, questions map[string]models.Question) error {
	userUUID := map[string]string{
		"alice": "22222222-2222-2222-2222-222222222222",
		"bob":   "33333333-3333-3333-3333-333333333333",
		"carol": "44444444-4444-4444-4444-444444444444",
	}

	now := seedBaseTime
	submissionDefs := []seedSubmissionDef{
		{ID: "seed-alice-array-two-sum-ac-1", User: "alice", QuestionID: "P1001", Language: "go", Status: "completed", Correct: true, AgeHours: 360, RuntimeMs: 2, MemoryKB: 1024},
		{ID: "seed-alice-array-merge-ac-1", User: "alice", QuestionID: "P1003", Language: "go", Status: "completed", Correct: true, AgeHours: 336, RuntimeMs: 4, MemoryKB: 1536},
		{ID: "seed-alice-window-learning-wa-1", User: "alice", QuestionID: "P1004", Language: "python", Status: "completed", Correct: false, AgeHours: 312, RuntimeMs: 18, MemoryKB: 2048},
		{ID: "seed-alice-dp-climb-ac-1", User: "alice", QuestionID: "P2001", Language: "go", Status: "completed", Correct: true, AgeHours: 288, RuntimeMs: 2, MemoryKB: 1024},
		{ID: "seed-alice-dp-subarray-ac-1", User: "alice", QuestionID: "P2101", Language: "go", Status: "completed", Correct: true, AgeHours: 264, RuntimeMs: 3, MemoryKB: 1024},
		{ID: "seed-alice-dp-robber-ac-1", User: "alice", QuestionID: "P2002", Language: "go", Status: "completed", Correct: true, AgeHours: 240, RuntimeMs: 3, MemoryKB: 1024},
		{ID: "seed-alice-graph-islands-ac-1", User: "alice", QuestionID: "P3001", Language: "go", Status: "completed", Correct: true, AgeHours: 216, RuntimeMs: 5, MemoryKB: 1536},
		{ID: "seed-alice-greedy-stock-ac-1", User: "alice", QuestionID: "P5001", Language: "go", Status: "completed", Correct: true, AgeHours: 192, RuntimeMs: 2, MemoryKB: 1024},
		{ID: "seed-bob-binary-search-ac-1", User: "bob", QuestionID: "P1101", Language: "cpp", Status: "completed", Correct: true, AgeHours: 344, RuntimeMs: 1, MemoryKB: 512},
		{ID: "seed-bob-rotated-search-wa-1", User: "bob", QuestionID: "P1103", Language: "cpp", Status: "completed", Correct: false, AgeHours: 320, RuntimeMs: 3, MemoryKB: 512},
		{ID: "seed-bob-islands-ac-1", User: "bob", QuestionID: "P3001", Language: "go", Status: "completed", Correct: true, AgeHours: 296, RuntimeMs: 6, MemoryKB: 1536},
		{ID: "seed-bob-provinces-ac-1", User: "bob", QuestionID: "P3101", Language: "go", Status: "completed", Correct: true, AgeHours: 272, RuntimeMs: 4, MemoryKB: 1536},
		{ID: "seed-bob-mst-ac-1", User: "bob", QuestionID: "P3102", Language: "go", Status: "completed", Correct: true, AgeHours: 248, RuntimeMs: 8, MemoryKB: 2048},
		{ID: "seed-bob-network-delay-wa-1", User: "bob", QuestionID: "P3003", Language: "python", Status: "completed", Correct: false, AgeHours: 224, RuntimeMs: 22, MemoryKB: 4096},
		{ID: "seed-bob-network-delay-tle-1", User: "bob", QuestionID: "P3003", Language: "cpp", Status: "error", Correct: false, AgeHours: 220, RuntimeMs: 5000, MemoryKB: 4096, ErrorCode: "E003", ErrorMsg: "超出时间限制"},
		{ID: "seed-bob-tree-depth-ac-1", User: "bob", QuestionID: "P4001", Language: "go", Status: "completed", Correct: true, AgeHours: 196, RuntimeMs: 2, MemoryKB: 1024},
		{ID: "seed-carol-valid-parentheses-ac-1", User: "carol", QuestionID: "P1002", Language: "go", Status: "completed", Correct: true, AgeHours: 332, RuntimeMs: 2, MemoryKB: 1024},
		{ID: "seed-carol-decode-string-ac-1", User: "carol", QuestionID: "P1007", Language: "python", Status: "completed", Correct: true, AgeHours: 308, RuntimeMs: 12, MemoryKB: 2048},
		{ID: "seed-carol-subsets-ac-1", User: "carol", QuestionID: "P5005", Language: "go", Status: "completed", Correct: true, AgeHours: 284, RuntimeMs: 3, MemoryKB: 1024},
		{ID: "seed-carol-permutation-wa-1", User: "carol", QuestionID: "P5004", Language: "go", Status: "completed", Correct: false, AgeHours: 260, RuntimeMs: 4, MemoryKB: 1024},
		{ID: "seed-carol-path-sum-ac-1", User: "carol", QuestionID: "P4005", Language: "go", Status: "completed", Correct: true, AgeHours: 236, RuntimeMs: 3, MemoryKB: 1024},
		{ID: "seed-carol-coin-change-wa-1", User: "carol", QuestionID: "P2103", Language: "python", Status: "completed", Correct: false, AgeHours: 212, RuntimeMs: 16, MemoryKB: 2048},
		{ID: "seed-carol-edit-distance-processing-1", User: "carol", QuestionID: "P2003", Language: "go", Status: "processing", Correct: false, AgeHours: 6, RuntimeMs: 0, MemoryKB: 0},
	}

	records := make([]models.Submission, 0, len(submissionDefs))
	for _, def := range submissionDefs {
		rec, err := buildSeedSubmission(def, userUUID, questions, now)
		if err != nil {
			return err
		}
		records = append(records, rec)
	}
	for _, rec := range records {
		item := rec
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&item).Error; err != nil {
			return err
		}
	}

	solvedDefs := []seedSolvedDef{
		{User: "alice", QuestionID: "P1001", AgeHours: 359},
		{User: "alice", QuestionID: "P1003", AgeHours: 335},
		{User: "alice", QuestionID: "P2001", AgeHours: 287},
		{User: "alice", QuestionID: "P2101", AgeHours: 263},
		{User: "alice", QuestionID: "P2002", AgeHours: 239},
		{User: "alice", QuestionID: "P3001", AgeHours: 215},
		{User: "alice", QuestionID: "P5001", AgeHours: 191},
		{User: "bob", QuestionID: "P1101", AgeHours: 343},
		{User: "bob", QuestionID: "P3001", AgeHours: 295},
		{User: "bob", QuestionID: "P3101", AgeHours: 271},
		{User: "bob", QuestionID: "P3102", AgeHours: 247},
		{User: "bob", QuestionID: "P4001", AgeHours: 195},
		{User: "carol", QuestionID: "P1002", AgeHours: 331},
		{User: "carol", QuestionID: "P1007", AgeHours: 307},
		{User: "carol", QuestionID: "P5005", AgeHours: 283},
		{User: "carol", QuestionID: "P4005", AgeHours: 235},
	}
	for _, def := range solvedDefs {
		userID, ok := userUUID[def.User]
		if !ok {
			return fmt.Errorf("unknown demo user: %s", def.User)
		}
		q, ok := questions[def.QuestionID]
		if !ok {
			return fmt.Errorf("unknown solved question: %s", def.QuestionID)
		}
		s := models.UserSolvedQuestion{
			UserUUID:   userID,
			QuestionID: q.Id,
			SolvedAt:   now.Add(-time.Duration(def.AgeHours) * time.Hour),
		}
		item := s
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_uuid"}, {Name: "question_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"solved_at": item.SolvedAt,
			}),
		}).Create(&item).Error; err != nil {
			return err
		}
	}

	masteryDefs := []seedMasteryDef{
		{User: "alice", SkillKey: "array", Mastery: 0.86},
		{User: "alice", SkillKey: "hash-table", Mastery: 0.82},
		{User: "alice", SkillKey: "dp", Mastery: 0.78},
		{User: "alice", SkillKey: "linear-dp", Mastery: 0.74},
		{User: "alice", SkillKey: "graph", Mastery: 0.55},
		{User: "alice", SkillKey: "greedy", Mastery: 0.66},
		{User: "alice", SkillKey: "string", Mastery: 0.34},
		{User: "bob", SkillKey: "graph", Mastery: 0.83},
		{User: "bob", SkillKey: "union-find", Mastery: 0.80},
		{User: "bob", SkillKey: "mst", Mastery: 0.74},
		{User: "bob", SkillKey: "shortest-path", Mastery: 0.42},
		{User: "bob", SkillKey: "binary-search", Mastery: 0.61},
		{User: "bob", SkillKey: "tree", Mastery: 0.50},
		{User: "carol", SkillKey: "string", Mastery: 0.82},
		{User: "carol", SkillKey: "stack", Mastery: 0.78},
		{User: "carol", SkillKey: "backtracking", Mastery: 0.64},
		{User: "carol", SkillKey: "tree", Mastery: 0.57},
		{User: "carol", SkillKey: "dp", Mastery: 0.35},
		{User: "carol", SkillKey: "knapsack", Mastery: 0.28},
	}
	for _, def := range masteryDefs {
		userID, ok := userUUID[def.User]
		if !ok {
			return fmt.Errorf("unknown mastery user: %s", def.User)
		}
		mastery := models.UserSkillMastery{
			UserID:    userID,
			SkillKey:  def.SkillKey,
			Mastery:   def.Mastery,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "skill_key"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"mastery":    mastery.Mastery,
				"updated_at": mastery.UpdatedAt,
			}),
		}).Create(&mastery).Error; err != nil {
			return err
		}
	}

	return nil
}

func buildSeedSubmission(def seedSubmissionDef, users map[string]string, questions map[string]models.Question, now time.Time) (models.Submission, error) {
	userID, ok := users[def.User]
	if !ok {
		return models.Submission{}, fmt.Errorf("unknown submission user: %s", def.User)
	}
	question, ok := questions[def.QuestionID]
	if !ok {
		return models.Submission{}, fmt.Errorf("unknown submission question: %s", def.QuestionID)
	}

	status := strings.TrimSpace(def.Status)
	if status == "" {
		status = "completed"
	}
	language := strings.TrimSpace(def.Language)
	if language == "" {
		language = "go"
	}
	code := seedCodeSample(language)
	createdAt := now.Add(-time.Duration(def.AgeHours) * time.Hour)

	return models.Submission{
		ID:         def.ID,
		UserID:     userID,
		QuestionID: question.Id,
		Language:   language,
		CodeLength: len(code),
		RuntimeMs:  def.RuntimeMs,
		MemoryKB:   def.MemoryKB,
		IsPublic:   true,
		Code:       code,
		Status:     status,
		Results:    seedSubmissionResults(status, def.Correct, def.RuntimeMs, def.MemoryKB),
		ErrorCode:  def.ErrorCode,
		ErrorMsg:   def.ErrorMsg,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
	}, nil
}

func seedSubmissionResults(status string, correct bool, runtimeMs int64, memoryKB int64) string {
	if status != "completed" {
		return "[]"
	}
	if runtimeMs <= 0 {
		runtimeMs = 1
	}
	if memoryKB <= 0 {
		memoryKB = 512
	}
	actual := "accepted"
	if !correct {
		actual = "wrong-answer"
	}
	return mustJSON([]models.TestCaseResult{
		{Input: "用例1", ExpectedOutput: "accepted", ActualOutput: actual, IsCorrect: correct, Runtime: runtimeMs, MemoryUsage: memoryKB},
		{Input: "用例2", ExpectedOutput: "accepted", ActualOutput: actual, IsCorrect: correct, Runtime: runtimeMs + 1, MemoryUsage: memoryKB},
	})
}

func seedCodeSample(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "python", "python3":
		return "print('demo')"
	case "cpp", "c++":
		return "#include <bits/stdc++.h>\nint main(){return 0;}"
	default:
		return "package main\nfunc main(){}"
	}
}

func seedOverview(db *gorm.DB) error {
	var total int64
	if err := db.Model(&models.Question{}).Where("status = ?", "published").Count(&total).Error; err != nil {
		return err
	}

	var view models.OjOverView
	if err := db.First(&view).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return err
		}
		view = models.OjOverView{
			TotalProlemCnt: int(total),
			HomeText:       "欢迎来到 PatreonOJ 演示环境。",
			Announcement:   "演示数据已加载，可使用 admin/admin123 登录体验。",
		}
		return db.Create(&view).Error
	}

	view.TotalProlemCnt = int(total)
	view.HomeText = "欢迎来到 PatreonOJ 演示环境。"
	view.Announcement = "演示数据已加载，可使用 admin/admin123 登录体验。"
	return db.Save(&view).Error
}
func md5Hex(text string) string {
	sum := md5.Sum([]byte(text))
	return hex.EncodeToString(sum[:])
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json failed: %v", err))
	}
	return string(b)
}
