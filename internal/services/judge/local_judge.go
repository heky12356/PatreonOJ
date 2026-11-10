package judge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"patreonoj/internal/config"
	"patreonoj/internal/models"
)

type LocalJudgeService struct {
	Config     *config.LocalJudgeConfig
	SandboxDir string
}

func NewLocalJudgeService(cfg *config.LocalJudgeConfig) *LocalJudgeService {
	return &LocalJudgeService{Config: cfg, SandboxDir: cfg.SandboxDir}
}

func (ljs *LocalJudgeService) JudgeBatch(code string, inputs []string, language string) ([]models.TestCaseResult, error) {
	return ljs.JudgeBatchContext(context.Background(), code, inputs, language)
}

func (ljs *LocalJudgeService) JudgeBatchContext(ctx context.Context, code string, inputs []string, language string) ([]models.TestCaseResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if ljs == nil || ljs.Config == nil {
		return nil, errors.New("local judge is not configured")
	}
	if strings.ToLower(strings.TrimSpace(ljs.Config.Executor)) != "docker" {
		return nil, errors.New("host judge is disabled: configure the docker executor or go-judge to isolate untrusted code")
	}
	if len(inputs) == 0 {
		return nil, errors.New("no test cases supplied")
	}
	if !ljs.IsLanguageSupported(language) {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
	return ljs.judgeBatchDocker(ctx, code, inputs, language)
}

func (ljs *LocalJudgeService) JudgeCode(code, input, language string) (*models.TestCaseResult, error) {
	results, err := ljs.JudgeBatch(code, []string{input}, language)
	if err != nil {
		return nil, err
	}
	return &results[0], nil
}

func (ljs *LocalJudgeService) dockerImageForLanguage(language string) string {
	switch language {
	case "go":
		if ljs.Config.DockerImageGo != "" {
			return ljs.Config.DockerImageGo
		}
		return "golang:1.22-bookworm"
	case "cpp":
		if ljs.Config.DockerImageCpp != "" {
			return ljs.Config.DockerImageCpp
		}
		return "gcc:13-bookworm"
	case "python":
		if ljs.Config.DockerImagePython != "" {
			return ljs.Config.DockerImagePython
		}
		return "python:3.12-bookworm"
	case "java":
		if ljs.Config.DockerImageJava != "" {
			return ljs.Config.DockerImageJava
		}
		return "eclipse-temurin:21-jdk"
	}
	return ""
}

func (ljs *LocalJudgeService) judgeBatchDocker(ctx context.Context, code string, inputs []string, language string) ([]models.TestCaseResult, error) {
	filename := map[string]string{"go": "main.go", "cpp": "main.cpp", "python": "main.py", "java": "Main.java"}[language]
	container := "oj_" + uuid.NewString()
	memory := ljs.Config.MaxMemory
	if memory <= 0 {
		memory = 256
	}
	compileMemory := max(memory, 512)
	workSize := ljs.Config.MaxWorkSize
	if workSize <= 0 {
		workSize = 256
	}
	args := []string{"run", "-d", "--rm", "--name", container, "--network", "none", "--cpus", "1", "--memory", strconv.Itoa(compileMemory) + "m", "--memory-swap", strconv.Itoa(compileMemory) + "m", "--pids-limit", "64", "--cap-drop", "ALL", "--security-opt", "no-new-privileges", "--read-only", "--user", "65534:65534", "--tmpfs", "/tmp:rw,nosuid,nodev,size=256m,mode=1777", "--tmpfs", fmt.Sprintf("/work:rw,exec,nosuid,nodev,size=%dm,mode=1777", workSize), "-w", "/work", "-e", "HOME=/tmp", "-e", "GOCACHE=/tmp/go-build", "-e", "GOMODCACHE=/tmp/go-mod", "-e", "GO111MODULE=off", "-e", "GOTOOLCHAIN=local", "-e", "GOMAXPROCS=2", "--entrypoint", "sh", ljs.dockerImageForLanguage(language), "-c", "while true; do sleep 3600; done"}
	// Register cleanup before creation: a cancelled CLI may still have created a container.
	defer ljs.removeContainer(container)
	createCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	out, stderr, _, err := runBoundedCommand(createCtx, "", nil, 64*1024, "docker", args...)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("docker run failed: %w: %s%s", err, out, stderr)
	}
	// Source travels through stdin. Untrusted code never receives a writable host
	// directory; /work and /tmp are bounded tmpfs mounts charged to the memory limit.
	copyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, stderr, _, err = ljs.dockerExec(copyCtx, container, code, 4096, "sh", "-c", "cat > "+filename)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("copy source into sandbox: %w: %s", err, stderr)
	}
	var compileArgs []string
	switch language {
	case "go":
		compileArgs = []string{"go", "build", "-o", "main", filename}
	case "cpp":
		compileArgs = []string{"g++", "-O2", "-std=c++17", "-o", "main", filename}
	case "java":
		compileArgs = []string{"javac", filename}
	}
	if len(compileArgs) > 0 {
		compileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, stderr, overflow, compileErr := ljs.dockerExec(compileCtx, container, "", 64*1024, compileArgs...)
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if compileErr != nil || overflow {
			return nil, &CompileError{Message: strings.TrimSpace(fmt.Sprint(compileErr) + " " + stderr)}
		}
	}
	// Compilation has a separate budget; execution must use the problem's memory limit.
	// Discard compiler caches before lowering the cgroup budget: tmpfs consumes RAM.
	cleanCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, stderr, _, err = ljs.dockerExec(cleanCtx, container, "", 4096, "sh", "-c", "rm -rf /tmp/go-build /tmp/go-mod")
	cancel()
	if err != nil {
		return nil, fmt.Errorf("clear compiler cache: %w: %s", err, stderr)
	}
	updateCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	_, stderr, _, err = runBoundedCommand(updateCtx, "", nil, 64*1024, "docker", "update", "--memory", strconv.Itoa(memory)+"m", "--memory-swap", strconv.Itoa(memory)+"m", container)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("set judge memory limit: %w: %s", err, stderr)
	}
	timeLimit := ljs.caseTimeLimit()
	outputLimit := ljs.Config.MaxOutputSize
	if outputLimit <= 0 {
		outputLimit = 1024
	}
	results := make([]models.TestCaseResult, 0, len(inputs))
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		runArgs := []string{"timeout", "-k", "1s", fmt.Sprintf("%.3fs", timeLimit.Seconds())}
		switch language {
		case "go", "cpp":
			runArgs = append(runArgs, "./main")
		case "python":
			runArgs = append(runArgs, "python3", "-u", filename)
		case "java":
			runArgs = append(runArgs, "java", "-XX:+UseSerialGC", fmt.Sprintf("-Xmx%dm", max(16, memory/2)), "-cp", ".", "Main")
		}
		runCtx, cancel := context.WithTimeout(ctx, timeLimit+2*time.Second)
		start := time.Now()
		stdout, stderr, overflow, runErr := ljs.dockerExec(runCtx, container, input, outputLimit*1024, runArgs...)
		deadline := errors.Is(runCtx.Err(), context.DeadlineExceeded)
		cancel()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result := models.TestCaseResult{Input: input, ActualOutput: strings.TrimSpace(stdout), Runtime: time.Since(start).Milliseconds(), IsCorrect: runErr == nil && !overflow}
		if overflow {
			result.ActualOutput = "Output Limit Exceeded"
			results = append(results, result)
			// Kill the whole sandbox so an output flood cannot survive the cancelled CLI.
			for len(results) < len(inputs) {
				results = append(results, models.TestCaseResult{ActualOutput: "Output Limit Exceeded"})
			}
			return results, nil
		}
		if runErr != nil {
			var exitErr *exec.ExitError
			if deadline || (errors.As(runErr, &exitErr) && (exitErr.ExitCode() == 124 || exitErr.ExitCode() == 137)) {
				result.ActualOutput = "Time Limit Exceeded"
			} else {
				result.ActualOutput = "Runtime Error: " + strings.TrimSpace(fmt.Sprint(runErr)+" "+stderr)
			}
		}
		results = append(results, result)
	}
	return results, nil
}

func (ljs *LocalJudgeService) caseTimeLimit() time.Duration {
	ms := ljs.Config.MaxTime
	if ms <= 0 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond
}

func (ljs *LocalJudgeService) dockerExec(ctx context.Context, container, input string, limit int, args ...string) (string, string, bool, error) {
	cli := append([]string{"exec", "-i", container}, args...)
	return runBoundedCommand(ctx, "", strings.NewReader(input), limit, "docker", cli...)
}

func (ljs *LocalJudgeService) removeContainer(container string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, _, _ = runBoundedCommand(ctx, "", nil, 4096, "docker", "rm", "-f", container)
}

// limitedBuffer cancels execution as soon as the capture limit is exceeded.
type limitedBuffer struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := max(0, b.limit-b.buffer.Len())
	if n > remaining {
		b.buffer.Write(p[:remaining])
		b.overflow = true
		b.cancel()
	} else {
		b.buffer.Write(p)
	}
	return n, nil
}

func runBoundedCommand(parent context.Context, dir string, stdin interface{ Read([]byte) (int, error) }, limit int, program string, args ...string) (string, string, bool, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	stdout := &limitedBuffer{limit: limit, cancel: cancel}
	stderr := &limitedBuffer{limit: min(limit, 64*1024), cancel: cancel}
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second
	err := cmd.Run()
	return stdout.buffer.String(), stderr.buffer.String(), stdout.overflow || stderr.overflow, err
}

func (ljs *LocalJudgeService) IsLanguageSupported(language string) bool {
	if len(ljs.Config.SupportedLanguages) == 0 {
		return language == "go" || language == "cpp" || language == "python" || language == "java"
	}
	for _, allowed := range ljs.Config.SupportedLanguages {
		if allowed == language {
			return true
		}
	}
	return false
}
