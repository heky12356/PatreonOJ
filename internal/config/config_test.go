package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultsAndEnvironmentOverrides(t *testing.T) {
	previous := GlobalConfig
	t.Cleanup(func() { GlobalConfig = previous })
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("database:\n  type: sqlite\nserver:\n  mode: test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATREONOJ_SERVER_SECURE_COOKIES", "true")
	t.Setenv("PATREONOJ_SERVER_AUTH_MAX_CONCURRENT", "1")
	t.Setenv("PATREONOJ_GRAPH_DATABASE_ENABLED", "true")
	if err := InitConfig(path); err != nil {
		t.Fatal(err)
	}
	if !GlobalConfig.Server.SecureCookies || GlobalConfig.Server.AuthMaxConcurrent != 1 || GlobalConfig.Judge.Local.MaxWorkSize != 256 || !GlobalConfig.GraphDatabase.Enabled {
		t.Fatal("configuration defaults or environment overrides were not applied")
	}
	if GlobalConfig.GetServerAddr() != "127.0.0.1:8080" {
		t.Fatal(GlobalConfig.GetServerAddr())
	}
}
