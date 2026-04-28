package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// AdultPythonLauncher 负责随 Go 后端一起启动 / 关闭 Python 番号刮削微服务
//
// 工作流程：
//  1. 读取 AdultScraperConfig.AutoStartPython / PythonExecutable / PythonServiceDir
//  2. 若开关打开：定位 python 可执行文件 → 启动 app.py 子进程 → 持续探测 /health
//  3. 健康探测通过后，若 PythonServiceURL 未配置，自动回填为本地地址（便于刮削链直接调用）
//  4. 收到 Stop() 时：优先发送 SIGTERM（Windows 平台发送 Kill），等待进程退出
type AdultPythonLauncher struct {
	cfg    *config.Config
	logger *zap.SugaredLogger

	mu       sync.Mutex
	cmd      *exec.Cmd
	running  bool
	endpoint string // 实际对外提供服务的 URL，如 http://127.0.0.1:5001
}

// NewAdultPythonLauncher 创建启动器
func NewAdultPythonLauncher(cfg *config.Config, logger *zap.SugaredLogger) *AdultPythonLauncher {
	return &AdultPythonLauncher{cfg: cfg, logger: logger}
}

// Start 启动 Python 微服务子进程（非阻塞）
// 返回 nil 表示已成功拉起并通过健康检查；返回 error 表示启动失败但不影响主服务运行
func (l *AdultPythonLauncher) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	ac := &l.cfg.AdultScraper
	if !ac.Enabled {
		// 番号刮削总开关未开启，Python 微服务本身就不会被调用，直接跳过
		return nil
	}
	if !ac.AutoStartPython {
		l.logger.Infof("[adult-python] 自动启动开关未开启（adult_scraper.auto_start_python=false），跳过拉起 Python 微服务；如需启用请在配置文件中设置 auto_start_python: true")
		return nil
	}
	if l.running {
		return nil
	}
	l.logger.Infof("[adult-python] 正在准备启动 Python 番号刮削微服务...")

	// 1. 解析 python 可执行文件路径
	pyExec := strings.TrimSpace(ac.PythonExecutable)
	if pyExec == "" {
		pyExec = detectPython()
	}
	if pyExec == "" {
		return fmt.Errorf("未找到可用的 python 可执行文件。Windows 用户请注意：如果系统提示『Python was not found; run without arguments to install from the Microsoft Store』，请在「设置 → 应用 → 应用执行别名」中关闭 python.exe / python3.exe 的别名；或在配置 adult_scraper.python_executable 中显式指定完整路径（例如 C:\\Users\\xxx\\AppData\\Local\\Python\\pythoncore-3.14-64\\python.exe）")
	}
	l.logger.Infof("[adult-python] 使用 python 可执行文件: %s", pyExec)

	// 2. 解析脚本目录
	scriptDir := strings.TrimSpace(ac.PythonServiceDir)
	if scriptDir == "" {
		scriptDir = "scripts/adult-scraper"
	}
	absScriptDir, err := filepath.Abs(scriptDir)
	if err != nil {
		return fmt.Errorf("解析 Python 脚本目录失败: %w", err)
	}
	appPy := filepath.Join(absScriptDir, "app.py")
	if _, err := os.Stat(appPy); err != nil {
		return fmt.Errorf("Python 入口文件不存在: %s", appPy)
	}

	// 3. 选择监听端口（优先使用配置中的 URL 端口，否则取 5000；端口占用时自动递增）
	host, port := parseHostPort(ac.PythonServiceURL, "127.0.0.1", 5000)
	if !isPortFree(host, port) {
		newPort := findFreePort(host, port+1, port+20)
		if newPort == 0 {
			return fmt.Errorf("端口 %d 已占用且附近端口也不可用", port)
		}
		l.logger.Warnf("[adult-python] 端口 %d 已被占用，自动切换到 %d", port, newPort)
		port = newPort
	}
	l.endpoint = fmt.Sprintf("http://%s:%d", host, port)

	// 4. 构造子进程
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, pyExec, "app.py")
	cmd.Dir = absScriptDir
	cmd.Env = append(os.Environ(),
		"PYTHONUNBUFFERED=1",
		"PYTHONIOENCODING=UTF-8",
		"SCRAPER_HOST="+host,
		"SCRAPER_PORT="+strconv.Itoa(port),
	)
	if ac.PythonServiceAPIKey != "" {
		cmd.Env = append(cmd.Env, "SCRAPER_API_KEY="+ac.PythonServiceAPIKey)
	}
	// 跨平台进程组设置（便于整组终止）
	configurePlatformProcAttr(cmd)

	// 5. 透传子进程日志到 zap
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("创建 stderr 管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 Python 子进程失败: %w", err)
	}
	l.cmd = cmd
	l.running = true

	go l.pipeLogs("stdout", stdout)
	go l.pipeLogs("stderr", stderr)

	// 6. 等待进程退出后自动标记
	go func() {
		if werr := cmd.Wait(); werr != nil {
			msg := werr.Error()
			l.logger.Warnf("[adult-python] 子进程退出: %v", werr)
			if strings.Contains(msg, "9009") {
				l.logger.Warnf("[adult-python] 提示：exit status 9009 通常是 Windows 应用执行别名问题。请：① 在「设置 → 应用 → 应用执行别名」中关闭 python.exe / python3.exe 的别名；或 ② 在配置 adult_scraper.python_executable 中显式指定完整的 python.exe 路径")
			}
		} else {
			l.logger.Infof("[adult-python] 子进程正常退出")
		}
		l.mu.Lock()
		l.running = false
		l.mu.Unlock()
	}()

	// 7. 健康探测（最多 20 秒）
	if err := l.waitHealthy(20 * time.Second); err != nil {
		l.logger.Warnf("[adult-python] 健康探测失败: %v（子进程仍在运行，Go 后端不受影响）", err)
		return nil
	}

	// 8. 若用户未显式配置 PythonServiceURL，则自动回填，让刮削链能直接用
	if strings.TrimSpace(ac.PythonServiceURL) == "" {
		ac.PythonServiceURL = l.endpoint
		l.logger.Infof("[adult-python] 自动回填 python_service_url=%s", l.endpoint)
	}

	l.logger.Infof("[adult-python] 微服务已就绪：%s (pid=%d)", l.endpoint, cmd.Process.Pid)
	return nil
}

// Stop 优雅关闭 Python 子进程
func (l *AdultPythonLauncher) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.running || l.cmd == nil || l.cmd.Process == nil {
		return
	}
	l.logger.Infof("[adult-python] 正在关闭子进程 pid=%d", l.cmd.Process.Pid)
	if err := terminateProcess(l.cmd); err != nil {
		l.logger.Warnf("[adult-python] 关闭子进程失败: %v", err)
	}
	// 最多等待 5 秒
	done := make(chan struct{})
	go func() {
		_, _ = l.cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = l.cmd.Process.Kill()
	}
	l.running = false
}

// Endpoint 返回当前实际运行的微服务地址（空表示未启用或启动失败）
func (l *AdultPythonLauncher) Endpoint() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.endpoint
}

// ==================== 内部辅助函数 ====================

func (l *AdultPythonLauncher) pipeLogs(tag string, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		l.logger.Infof("[adult-python/%s] %s", tag, line)
	}
}

func (l *AdultPythonLauncher) waitHealthy(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	url := l.endpoint + "/health"
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("超时未就绪（%s）", url)
}

// detectPython 自动探测可用的 python 可执行文件
// Windows 下特别处理"应用执行别名"（WindowsApps 目录下的空壳 python.exe），
// 这类别名 LookPath 能成功但执行会返回 9009 错误
func detectPython() string {
	if runtime.GOOS == "windows" {
		return detectPythonWindows()
	}
	// Unix / macOS：python3 优先
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// detectPythonWindows Windows 平台专用的 python 探测
// 按可靠性排序：
//  1. py.exe （官方 Python Launcher，永远不会被别名劫持）
//  2. 常见默认安装路径下的 python.exe（用户级 PythonCore、Programs\Python、系统级）
//  3. PATH 中的 python.exe / python3.exe（但跳过 WindowsApps 别名）
func detectPythonWindows() string {
	// 1. py.exe（Python Launcher）
	if p, err := exec.LookPath("py.exe"); err == nil && isUsablePython(p) {
		return p
	}

	// 2. 用户级 & 系统级默认安装路径
	var searchGlobs []string
	if home, err := os.UserHomeDir(); err == nil {
		searchGlobs = append(searchGlobs,
			// Microsoft Store Python 分发包（用户安装 Python 3.x 后的真实路径）
			filepath.Join(home, "AppData", "Local", "Python", "pythoncore-*", "python.exe"),
			// 官方 installer 的用户级安装路径
			filepath.Join(home, "AppData", "Local", "Programs", "Python", "Python3*", "python.exe"),
		)
	}
	// 常见系统级安装路径
	for _, drive := range []string{"C:", "D:"} {
		searchGlobs = append(searchGlobs,
			drive+`\Python3*\python.exe`,
			drive+`\Program Files\Python3*\python.exe`,
			drive+`\Program Files (x86)\Python3*\python.exe`,
		)
	}
	for _, pat := range searchGlobs {
		matches, _ := filepath.Glob(pat)
		// 优先取版本号较高的（glob 默认按字母序，pythoncore-3.14 会排在 3.12 后面）
		for i := len(matches) - 1; i >= 0; i-- {
			if isUsablePython(matches[i]) {
				return matches[i]
			}
		}
	}

	// 3. PATH 中查找，但跳过 WindowsApps 下的应用执行别名空壳
	for _, c := range []string{"python.exe", "python3.exe"} {
		if p, err := exec.LookPath(c); err == nil {
			if strings.Contains(strings.ToLower(p), `\windowsapps\`) {
				// 这是 Windows 应用执行别名，跳过
				continue
			}
			if isUsablePython(p) {
				return p
			}
		}
	}
	return ""
}

// isUsablePython 通过执行 --version 验证 python 可执行文件是否真正可用
// 用于过滤掉 Windows 应用执行别名（exit code 9009）等空壳情况
func isUsablePython(path string) bool {
	if path == "" {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	// 抑制 Windows 弹窗 / 控制台闪屏
	configurePlatformProcAttr(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// 真实的 python --version 会输出 "Python 3.x.x"
	return strings.Contains(strings.ToLower(string(out)), "python")
}

// parseHostPort 从 url 中解析 host/port，解析失败则使用默认值
func parseHostPort(rawURL, defaultHost string, defaultPort int) (string, int) {
	if rawURL == "" {
		return defaultHost, defaultPort
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return defaultHost, defaultPort
	}
	host := u.Hostname()
	if host == "" {
		host = defaultHost
	}
	port := defaultPort
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return host, port
}

// isPortFree 判断端口是否空闲
func isPortFree(host string, port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// findFreePort 在 [start, end] 范围内寻找一个可用端口，找不到返回 0
func findFreePort(host string, start, end int) int {
	for p := start; p <= end; p++ {
		if isPortFree(host, p) {
			return p
		}
	}
	return 0
}
