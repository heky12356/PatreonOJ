package graph

import (
	"context"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jClient Neo4j客户端
type Neo4jClient struct {
	driver neo4j.DriverWithContext
	config Neo4jConfig
}

// NewNeo4jClient 创建新的Neo4j客户端
func NewNeo4jClient(config Neo4jConfig) (*Neo4jClient, error) {
	driver, err := neo4j.NewDriverWithContext(
		config.URI,
		neo4j.BasicAuth(config.Username, config.Password, ""),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	client := &Neo4jClient{
		driver: driver,
		config: config,
	}

	// 测试连接
	if err := client.TestConnection(); err != nil {
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	// 创建数据库（如果不存在）
	if err := client.createDatabaseIfNotExists(); err != nil {
		log.Printf("Warning: failed to create database: %v", err)
	}

	// 创建索引和约束
	if err := client.createConstraints(); err != nil {
		log.Printf("Warning: failed to create constraints: %v", err)
	}

	return client, nil
}

// TestConnection 测试数据库连接
func (c *Neo4jClient) TestConnection() error {
	ctx := context.Background()
	return c.driver.VerifyConnectivity(ctx)
}

// Close 关闭连接
func (c *Neo4jClient) Close() error {
	ctx := context.Background()
	return c.driver.Close(ctx)
}

// createDatabaseIfNotExists 创建数据库（如果不存在）
func (c *Neo4jClient) createDatabaseIfNotExists() error {
	ctx := context.Background()
	
	// 如果使用默认数据库，跳过创建步骤
	if c.config.Database == "neo4j" || c.config.Database == "" {
		log.Printf("Using default database, skipping database creation")
		return nil
	}
	
	// 使用系统数据库连接来创建新数据库
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: "system", // 使用系统数据库
	})
	defer session.Close(ctx)

	// 检查数据库是否存在
	checkQuery := "SHOW DATABASES YIELD name WHERE name = $dbName RETURN count(*) as count"
	result, err := session.Run(ctx, checkQuery, map[string]interface{}{
		"dbName": c.config.Database,
	})
	if err != nil {
		// 如果SHOW DATABASES命令不支持，说明是社区版或旧版本，直接返回
		log.Printf("Cannot check database existence (possibly Neo4j Community Edition): %v", err)
		return nil
	}

	if result.Next(ctx) {
		count := result.Record().Values[0].(int64)
		if count > 0 {
			log.Printf("Database '%s' already exists", c.config.Database)
			return nil
		}
	}

	// 创建数据库
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		createQuery := fmt.Sprintf("CREATE DATABASE `%s` IF NOT EXISTS", c.config.Database)
		_, err := tx.Run(ctx, createQuery, nil)
		return nil, err
	})

	if err != nil {
		// 如果创建失败，可能是社区版不支持多数据库，记录警告但不返回错误
		log.Printf("Warning: failed to create database '%s' (possibly Neo4j Community Edition): %v", c.config.Database, err)
		return nil
	}

	log.Printf("Database '%s' created successfully", c.config.Database)
	return nil
}

// createConstraints 创建约束和索引
func (c *Neo4jClient) createConstraints() error {
	ctx := context.Background()
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.config.Database,
	})
	defer session.Close(ctx)

	// 创建题目编号的唯一约束
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := "CREATE CONSTRAINT question_number_unique IF NOT EXISTS FOR (q:Question) REQUIRE q.question_number IS UNIQUE"
		_, err := tx.Run(ctx, query, nil)
		return nil, err
	})

	if err != nil {
		return fmt.Errorf("failed to create unique constraint: %w", err)
	}

	// 创建题目编号的索引
	_, err = session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		query := "CREATE INDEX question_number_index IF NOT EXISTS FOR (q:Question) ON (q.question_number)"
		_, err := tx.Run(ctx, query, nil)
		return nil, err
	})

	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// ExecuteQuery 执行查询
func (c *Neo4jClient) ExecuteQuery(ctx context.Context, query string, params map[string]interface{}) (neo4j.ResultWithContext, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.config.Database,
	})
	defer session.Close(ctx)

	return session.Run(ctx, query, params)
}

// ExecuteWrite 执行写操作
func (c *Neo4jClient) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork) (interface{}, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.config.Database,
	})
	defer session.Close(ctx)

	return session.ExecuteWrite(ctx, work)
}

// ExecuteRead 执行读操作
func (c *Neo4jClient) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork) (interface{}, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: c.config.Database,
	})
	defer session.Close(ctx)

	return session.ExecuteRead(ctx, work)
}