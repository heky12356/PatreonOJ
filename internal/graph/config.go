package graph

// Neo4jConfig Neo4j数据库配置
type Neo4jConfig struct {
	URI      string `yaml:"uri"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

// GraphConfig 图数据库总配置
type GraphConfig struct {
	Neo4j Neo4jConfig `yaml:"neo4j"`
}