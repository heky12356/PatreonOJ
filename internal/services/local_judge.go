package services

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// JudgeCode 本地评测代码
func (ljs *LocalJudgeService) JudgeCode(code, input, language string) (*models.TestCaseResult, error) {
	log.Print("start judge code")
	// 创建沙箱目录
	sandboxPath, err := ljs.createSandbox()
	if err != nil {
		return nil, fmt.Errorf("创建沙箱失败: %w", err)
	}
	// 临时注释掉自动清理，用于调试
	defer ljs.cleanupSandbox(sandboxPath)

	log.Printf("沙箱目录创建成功: %s", sandboxPath)

	// 写入代码文件
	codeFile, err := ljs.writeCodeFile(sandboxPath, code, language)
	if err != nil {
		return nil, fmt.Errorf("写入代码文件失败: %w", err)
	}

	// 编译代码（如果需要）
	executablePath, err := ljs.compileCode(sandboxPath, codeFile, language)
	if err != nil {
		return nil, fmt.Errorf("编译失败: %w", err)
	}

	// 执行代码
	result, err := ljs.executeCode(sandboxPath, executablePath, input, language)
	if err != nil {
		return nil, fmt.Errorf("执行失败: %w", err)
	}

	return result, nil
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
