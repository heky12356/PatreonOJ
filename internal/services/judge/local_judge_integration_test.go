package judge

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"patreonoj/internal/config"
)

func TestDockerJudgeIsolation(t *testing.T) {
	if os.Getenv("OJ_DOCKER_TEST") != "1" {
		t.Skip("set OJ_DOCKER_TEST=1 to run isolated Docker checks")
	}
	cfg := config.LocalJudgeConfig{Enabled: true, Executor: "docker", MaxMemory: 256, MaxTime: 5000, MaxOutputSize: 64, MaxWorkSize: 16}
	cfg.DockerImageGo, cfg.DockerImageJava = os.Getenv("OJ_TEST_GO_IMAGE"), os.Getenv("OJ_TEST_JAVA_IMAGE")
	for _, tc := range []struct{ language, code, want string }{
		{"python", "print(int(input()) + 1)", "3"},
		{"cpp", "#include <iostream>\nint main(){int n;std::cin>>n;std::cout<<n+1;}", "3"},
		{"go", "package main\nimport \"fmt\"\nfunc main(){var n int;fmt.Scan(&n);fmt.Print(n+1)}", "3"},
		{"java", "public class Main { public static void main(String[] args) { java.util.Scanner s=new java.util.Scanner(System.in); System.out.print(s.nextInt()+1); } }", "3"},
		{"python", "import errno\ntry:\n open('/root-write-check','w')\nexcept OSError as e:\n assert e.errno == errno.EROFS\ntry:\n with open('/work/fill','wb') as f:\n  for _ in range(32): f.write(b'x' * (1024*1024))\nexcept OSError as e:\n assert e.errno == errno.ENOSPC\n print('bounded')", "bounded"},
	} {
		t.Run(tc.language+"_"+tc.want, func(t *testing.T) {
			if tc.language == "go" && cfg.DockerImageGo == "" {
				t.Skip("set OJ_TEST_GO_IMAGE to a local image containing go")
			}
			if tc.language == "java" && cfg.DockerImageJava == "" {
				t.Skip("set OJ_TEST_JAVA_IMAGE to a local image containing javac/java")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			results, err := NewLocalJudgeService(&cfg).JudgeBatchContext(ctx, tc.code, []string{"2"}, tc.language)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 1 || !results[0].IsCorrect || strings.TrimSpace(results[0].ActualOutput) != tc.want {
				t.Fatalf("unexpected results: %+v", results)
			}
		})
	}
}
