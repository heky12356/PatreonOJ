package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dachuang/internal/config"
	"dachuang/internal/models"
)

// LocalJudgeService 本地评测服务
type LocalJudgeService struct {
	Config     *config.LocalJudgeConfig
	SandboxDir string
}

// NewLocalJudgeService 创建本地评测服务实例
func NewLocalJudgeService(cfg *config.LocalJudgeConfig) *LocalJudgeService {
	return &LocalJudgeService{
		Config:     cfg,
		SandboxDir: cfg.SandboxDir,
	}
}

func (ljs *LocalJudgeService) JudgeBatch(code string, inputs []string, language string) ([]models.TestCaseResult, error) {
	executor := strings.ToLower(strings.TrimSpace(ljs.Config.Executor))
	if executor == "docker" {
		return ljs.judgeBatchDocker(code, inputs, language)
	}

	results := make([]models.TestCaseResult, 0, len(inputs))
	for _, input := range inputs {
		r, err := ljs.JudgeCode(code, input, language)
		if err != nil {
			r = &models.TestCaseResult{Input: input, ActualOutput: fmt.Sprintf("Error: %v", err), IsCorrect: false, Runtime: 0, MemoryUsage: 0}
		}
		results = append(results, *r)
	}
	return results, nil
}

// JudgeCode 本地评测代码（兼容旧接口：单 case）
func (ljs *LocalJudgeService) JudgeCode(code, input, language string) (*models.TestCaseResult, error) {
	log.Print("start judge code")

	results, err := ljs.JudgeBatch(code, []string{input}, language)
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, fmt.Errorf("本地评测结果数量异常")
	}
	out := results[0]
	return &out, nil
}

// createSandbox 创建沙箱目录
func (ljs *LocalJudgeService) createSandbox() (string, error) {
	sandboxPath := filepath.Join(ljs.Config.SandboxDir, fmt.Sprintf("sandbox_%d", time.Now().UnixNano()))

	// 创建沙箱目录
	err := os.MkdirAll(sandboxPath, 0o755)
	if err != nil {
		return "", fmt.Errorf("创建沙箱目录失败: %w", err)
	}

	// 验证目录是否创建成功
	if _, err := os.Stat(sandboxPath); os.IsNotExist(err) {
		return "", fmt.Errorf("沙箱目录创建后不存在: %s", sandboxPath)
	}

	return sandboxPath, nil
}

// cleanupSandbox 清理沙箱目录
func (ljs *LocalJudgeService) cleanupSandbox(sandboxPath string) {
	os.RemoveAll(sandboxPath)
}

// writeCodeFile 写入代码文件
func (ljs *LocalJudgeService) writeCodeFile(sandboxPath, code, language string) (string, error) {
	log.Printf("write code file, language: %s", language)
	var filename string
	switch language {
	case "go":
		filename = "main.go"
	case "python":
		filename = "main.py"
	case "cpp":
		filename = "main.cpp"
	case "java":
		filename = "Main.java"
	default:
		return "", fmt.Errorf("不支持的语言: %s", language)
	}

	codeFile := filepath.Join(sandboxPath, filename)

	log.Printf("write code file: %s", codeFile)

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(codeFile), 0o755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	err := ioutil.WriteFile(codeFile, []byte(code), 0o644)
	if err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}

	// 验证文件是否存在
	if _, err := os.Stat(codeFile); os.IsNotExist(err) {
		return "", fmt.Errorf("文件写入后不存在: %s", codeFile)
	}

	return codeFile, nil
}

func (ljs *LocalJudgeService) dockerImageForLanguage(language string) string {
	lang := strings.ToLower(strings.TrimSpace(language))
	switch lang {
	case "go":
		if strings.TrimSpace(ljs.Config.DockerImageGo) != "" {
			return strings.TrimSpace(ljs.Config.DockerImageGo)
		}
		return "golang:1.22-bookworm"
	case "cpp":
		if strings.TrimSpace(ljs.Config.DockerImageCpp) != "" {
			return strings.TrimSpace(ljs.Config.DockerImageCpp)
		}
		return "gcc:13-bookworm"
	case "python":
		if strings.TrimSpace(ljs.Config.DockerImagePython) != "" {
			return strings.TrimSpace(ljs.Config.DockerImagePython)
		}
		return "python:3.12-bookworm"
	case "java":
		if strings.TrimSpace(ljs.Config.DockerImageJava) != "" {
			return strings.TrimSpace(ljs.Config.DockerImageJava)
		}
		return "eclipse-temurin:21-jdk"
	default:
		return ""
	}
}

func (ljs *LocalJudgeService) dockerMountSpec(hostDir string) (string, error) {
	abs, err := filepath.Abs(hostDir)
	if err != nil {
		return "", err
	}
	abs = filepath.ToSlash(abs)
	return abs + ":/work:rw", nil
}

func (ljs *LocalJudgeService) dockerRunDetached(ctx context.Context, containerName string, image string, mount string) error {
	mem := strings.TrimSpace(strconv.Itoa(ljs.Config.MaxMemory))
	if mem == "" || mem == "0" {
		mem = "128"
	}
	maxTime := ljs.Config.MaxTime
	if maxTime <= 0 {
		maxTime = 5
	}

	args := []string{
		"run", "-d", "--rm",
		"--name", containerName,
		"--network", "none",
		"--cpus", "1",
		"--memory", mem + "m",
		"--pids-limit", "64",
		"--read-only",
		"--tmpfs", "/tmp:rw,size=64m",
		"-v", mount,
		"-w", "/work",
		image,
		"sh", "-c", "while true; do sleep 3600; done",
	}
	cmd := exec.CommandContext(ctx, "docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run failed: %v, output: %s", err, string(out))
	}
	return nil
}

func (ljs *LocalJudgeService) dockerRemove(ctx context.Context, containerName string) {
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()
}

func (ljs *LocalJudgeService) dockerExec(ctx context.Context, containerName string, stdin string, args ...string) (string, error) {
	base := []string{"exec", "-i", containerName}
	base = append(base, args...)
	cmd := exec.CommandContext(ctx, "docker", base...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (ljs *LocalJudgeService) judgeBatchDocker(code string, inputs []string, language string) ([]models.TestCaseResult, error) {
	image := ljs.dockerImageForLanguage(language)
	if image == "" {
		return nil, fmt.Errorf("不支持的语言: %s", language)
	}

	sandboxPath, err := ljs.createSandbox()
	if err != nil {
		return nil, fmt.Errorf("创建沙箱失败: %w", err)
	}
	defer ljs.cleanupSandbox(sandboxPath)

	codeFile, err := ljs.writeCodeFile(sandboxPath, code, language)
	if err != nil {
		return nil, fmt.Errorf("写入代码文件失败: %w", err)
	}

	mount, err := ljs.dockerMountSpec(sandboxPath)
	if err != nil {
		return nil, err
	}

	containerName := fmt.Sprintf("oj_%d", time.Now().UnixNano())
	runCtx, cancelRun := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelRun()
	if err := ljs.dockerRunDetached(runCtx, containerName, image, mount); err != nil {
		return nil, err
	}
	defer ljs.dockerRemove(context.Background(), containerName)

	compileErr := ""
	filename := filepath.Base(codeFile)
	compileTimeout := time.Duration(ljs.Config.MaxTime)
	if compileTimeout <= 0 {
		compileTimeout = 5
	}
	compileTimeout = (compileTimeout * time.Second) + 15*time.Second
	cctx, cancelCompile := context.WithTimeout(context.Background(), compileTimeout)
	defer cancelCompile()
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "go":
		_, err = ljs.dockerExec(cctx, containerName, "", "go", "build", "-o", "main", filename)
	case "cpp":
		_, err = ljs.dockerExec(cctx, containerName, "", "g++", "-O2", "-std=c++17", "-o", "main", filename)
	case "java":
		_, err = ljs.dockerExec(cctx, containerName, "", "javac", filename)
	case "python":
		err = nil
	default:
		err = fmt.Errorf("不支持的语言: %s", language)
	}
	if err != nil {
		compileErr = fmt.Sprintf("Compile Error: %v", err)
	}

	maxOutput := ljs.Config.MaxOutputSize
	if maxOutput <= 0 {
		maxOutput = 1024
	}
	caseTimeoutSec := ljs.Config.MaxTime
	if caseTimeoutSec <= 0 {
		caseTimeoutSec = 5
	}

	results := make([]models.TestCaseResult, 0, len(inputs))
	for _, input := range inputs {
		if compileErr != "" {
			results = append(results, models.TestCaseResult{Input: input, ActualOutput: compileErr, IsCorrect: false, Runtime: 0, MemoryUsage: 0})
			continue
		}

		runArgs := []string{"timeout", "-k", "1s", fmt.Sprintf("%ds", caseTimeoutSec)}
		switch strings.ToLower(strings.TrimSpace(language)) {
		case "go", "cpp":
			runArgs = append(runArgs, "./main")
		case "python":
			runArgs = append(runArgs, "python", "-u", filename)
		case "java":
			runArgs = append(runArgs, "java", "-cp", ".", "Main")
		}

		start := time.Now()
		rctx, cancel := context.WithTimeout(context.Background(), time.Duration(caseTimeoutSec+2)*time.Second)
		out, runErr := ljs.dockerExec(rctx, containerName, input, runArgs...)
		cancel()
		runtime := time.Since(start).Milliseconds()

		actual := strings.TrimSpace(out)
		if len(actual) > maxOutput*1024 {
			actual = actual[:maxOutput*1024] + "...[输出被截断]"
		}

		if runErr != nil {
			if strings.Contains(actual, "Time") || strings.Contains(actual, "exceeded") {
				actual = "Time Limit Exceeded"
			} else if actual == "" {
				actual = fmt.Sprintf("Runtime Error: %v", runErr)
			}
		}

		results = append(results, models.TestCaseResult{Input: input, ActualOutput: actual, IsCorrect: false, Runtime: runtime, MemoryUsage: 0})
	}

	return results, nil
}

// compileCode 编译代码
func (ljs *LocalJudgeService) compileCode(sandboxPath, codeFile, language string) (string, error) {
	var cmd *exec.Cmd
	var executablePath string

	log.Printf("开始编译，沙箱路径: %s", sandboxPath)
	log.Printf("源文件路径: %s", codeFile)

	// 验证源文件是否存在
	if _, err := os.Stat(codeFile); os.IsNotExist(err) {
		return "", fmt.Errorf("源文件不存在: %s", codeFile)
	}

	// 获取相对于沙箱目录的文件名
	filename := filepath.Base(codeFile)
	log.Printf("文件名: %s", filename)

	switch language {
	case "go":
		executablePath = filepath.Join(sandboxPath, "main.exe")
		// 使用相对路径编译
		cmd = exec.Command("go", "build", "-o", "main.exe", filename)
	case "cpp":
		executablePath = filepath.Join(sandboxPath, "main.exe")
		// 使用相对路径编译
		cmd = exec.Command("g++", "-o", "main.exe", filename)
	case "java":
		// Java编译后的类文件
		cmd = exec.Command("javac", filename)
		executablePath = filepath.Join(sandboxPath, "Main.class")
	case "python":
		// Python不需要编译
		return codeFile, nil
	default:
		return "", fmt.Errorf("不支持的语言: %s", language)
	}

	// 设置工作目录为沙箱目录，这样就可以使用相对路径
	cmd.Dir = sandboxPath
	log.Printf("编译命令: %v", cmd.Args)
	log.Printf("工作目录: %s", cmd.Dir)

	output, err := cmd.CombinedOutput()
	log.Printf("编译输出: %s", string(output))

	if err != nil {
		log.Printf("编译失败: %v", err)
		return "", fmt.Errorf("编译错误: %s", string(output))
	}

	// 验证编译产物是否存在
	if language != "python" {
		if _, err := os.Stat(executablePath); os.IsNotExist(err) {
			return "", fmt.Errorf("编译产物不存在: %s", executablePath)
		}
	}

	log.Printf("编译成功，可执行文件: %s", executablePath)
	return executablePath, nil
}

// executeCode 执行代码
func (ljs *LocalJudgeService) executeCode(sandboxPath, executablePath, input, language string) (*models.TestCaseResult, error) {
	var cmd *exec.Cmd

	log.Printf("开始执行，沙箱路径: %s", sandboxPath)
	log.Printf("可执行文件路径: %s", executablePath)

	// 获取相对于沙箱目录的可执行文件名
	executableName := filepath.Base(executablePath)
	log.Printf("可执行文件名: %s", executableName)

	switch language {
	case "go", "cpp":
		// 使用相对路径执行
		cmd = exec.Command(".\\" + executableName)
	case "python":
		// Python使用相对路径
		pythonFile := filepath.Base(executablePath)
		cmd = exec.Command("python", pythonFile)
	case "java":
		cmd = exec.Command("java", "-cp", ".", "Main")
	default:
		return nil, fmt.Errorf("不支持的语言: %s", language)
	}

	log.Printf("执行命令: %v", cmd.Args)

	// 创建上下文以控制超时
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(ljs.Config.MaxTime)*time.Second)
	defer cancel()

	// 使用上下文执行命令
	cmd = exec.CommandContext(ctx, cmd.Args[0], cmd.Args[1:]...)
	cmd.Dir = sandboxPath
	cmd.Stdin = strings.NewReader(input)

	log.Printf("最终执行命令: %v", cmd.Args)
	log.Printf("工作目录: %s", cmd.Dir)

	startTime := time.Now()
	output, err := cmd.CombinedOutput()
	runtime := time.Since(startTime).Milliseconds()

	result := &models.TestCaseResult{
		Input:        input,
		ActualOutput: strings.TrimSpace(string(output)),
		Runtime:      runtime,
		MemoryUsage:  0, // 简单实现，暂不统计内存使用
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.ActualOutput = "Time Limit Exceeded"
		} else {
			result.ActualOutput = fmt.Sprintf("Runtime Error: %v", err)
		}
		result.IsCorrect = false
	}

	// 检查输出大小限制
	if len(result.ActualOutput) > ljs.Config.MaxOutputSize*1024 {
		result.ActualOutput = result.ActualOutput[:ljs.Config.MaxOutputSize*1024] + "...[输出被截断]"
	}

	return result, nil
}

// IsLanguageSupported 检查是否支持指定语言
func (ljs *LocalJudgeService) IsLanguageSupported(language string) bool {
	for _, lang := range ljs.Config.SupportedLanguages {
		if lang == language {
			return true
		}
	}
	return false
}
