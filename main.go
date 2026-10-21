// PkgHub —— 使用 Go + SQLite 实现的软件包管理系统，所有前端资源内嵌于单一可执行文件。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"goPkgMgr/internal/config"
	"goPkgMgr/internal/store"
	"goPkgMgr/internal/web"
)

// version 程序版本（构建时可通过 -ldflags "-X main.version=..." 覆盖）。
var version = "1.1.0"

func main() {
	var (
		fConfig   = flag.String("config", "", "配置文件路径（默认：可执行文件同级目录下的 config.toml）")
		fAppName  = flag.String("app-name", "", "平台名称（显示在浏览器标签页、登录页与侧边栏）")
		fAddr     = flag.String("addr", "", "HTTP 监听地址，例如 :8080")
		fData     = flag.String("data", "", "数据目录（默认：可执行文件同级目录下的 data）")
		fDB       = flag.String("db", "", "SQLite 数据库文件路径（默认：<data_dir>/pkghub.db）")
		fStorage  = flag.String("storage", "", "软件包文件存储目录（默认：<data_dir>/files）")
		fMaxMB    = flag.Int("max-upload", 0, "单个文件上传大小上限（MB）")
		fSessHour = flag.Int("session-hours", 0, "登录会话有效期（小时）")
		fAdminU   = flag.String("admin-user", "", "初始化时创建的默认管理员用户名")
		fAdminP   = flag.String("admin-password", "", "初始化时创建的默认管理员密码")
		fOpen     = flag.Bool("open", false, "启动后自动打开浏览器")
		fPrintCfg = flag.Bool("print-config", false, "打印生效的完整配置后退出")
		fVersion  = flag.Bool("version", false, "打印版本信息并退出")
	)
	flag.Parse()

	if *fVersion {
		fmt.Printf("PkgHub %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
		return
	}

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("无法定位程序路径: %v", err)
	}
	exeDir := filepath.Dir(exe)

	// 1) 配置文件路径：-config > PKGHUB_CONFIG > 可执行文件同级 config.toml
	cfgPath := strings.TrimSpace(*fConfig)
	if cfgPath == "" {
		cfgPath = strings.TrimSpace(os.Getenv("PKGHUB_CONFIG"))
	}
	if cfgPath == "" {
		cfgPath = config.ConfigFilePath(exeDir)
	}

	// 2) 加载配置文件；不存在时自动生成带注释的默认模板
	cfg, cfgExists, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("× %v", err)
	}
	if !cfgExists {
		if created, werr := config.WriteIfAbsent(cfgPath, cfg); created && werr == nil {
			fmt.Printf("未找到配置文件，已生成默认配置：%s\n", cfgPath)
		} else if werr != nil {
			log.Printf("[warn] 无法写入默认配置文件 %s: %v", cfgPath, werr)
		}
	}

	// 3) 环境变量覆盖配置文件
	cfg.ApplyEnv()

	// 4) 显式传入的命令行参数拥有最高优先级
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if explicit["app-name"] {
		cfg.AppName = *fAppName
	}
	if explicit["addr"] {
		cfg.Addr = *fAddr
	}
	if explicit["data"] {
		cfg.DataDir = *fData
	}
	if explicit["db"] {
		cfg.DBPath = *fDB
	}
	if explicit["storage"] {
		cfg.StorageDir = *fStorage
	}
	if explicit["max-upload"] {
		cfg.MaxUploadMB = *fMaxMB
	}
	if explicit["session-hours"] {
		cfg.SessionHours = *fSessHour
	}
	if explicit["admin-user"] {
		cfg.AdminUser = *fAdminU
	}
	if explicit["admin-password"] {
		cfg.AdminPassword = *fAdminP
	}
	if explicit["open"] {
		cfg.OpenBrowser = *fOpen
	}
	if err := cfg.Normalize(); err != nil {
		log.Fatalf("× 配置参数非法: %v", err)
	}

	// 5) 推导实际路径并创建目录
	paths, err := cfg.ResolvePaths(exeDir)
	if err != nil {
		log.Fatalf("× %v", err)
	}

	if *fPrintCfg {
		fmt.Printf("配置文件      : %s\n", cfgPath)
		fmt.Printf("平台名称      : %s\n", cfg.AppName)
		fmt.Printf("监听地址      : %s\n", cfg.Addr)
		fmt.Printf("数据目录      : %s\n", paths.DataDir)
		fmt.Printf("数据库        : %s\n", paths.DBPath)
		fmt.Printf("软件存储目录  : %s\n", paths.StorageDir)
		fmt.Printf("上传上限      : %d MB\n", cfg.MaxUploadMB)
		fmt.Printf("会话有效期    : %d 小时\n", cfg.SessionHours)
		fmt.Printf("自动打开浏览器: %t\n", cfg.OpenBrowser)
		return
	}

	st, err := store.Open(paths.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()

	adminPass := strings.TrimSpace(cfg.AdminPassword)
	if adminPass == "" {
		adminPass = config.DefaultAdminPass
	}
	createdAdmin, err := st.EnsureDefaultAdmin(cfg.AdminUser, adminPass)
	if err != nil {
		log.Fatalf("初始化管理员失败: %v", err)
	}
	if err := st.EnsureDefaultCategory(); err != nil {
		log.Fatalf("初始化默认分类失败: %v", err)
	}
	_ = st.CleanupSessions()

	srv, err := web.New(st, web.Config{
		AppName:    cfg.AppName,
		DataDir:    paths.DataDir,
		DBPath:     paths.DBPath,
		FilesDir:   paths.StorageDir,
		MaxUpload:  cfg.MaxUploadBytes(),
		SessionTTL: time.Duration(cfg.SessionHours) * time.Hour,
		Version:    version,
	})
	if err != nil {
		log.Fatalf("初始化 Web 服务失败: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
		WriteTimeout:      0, // 大文件下载不限制写超时
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			_ = st.CleanupSessions()
		}
	}()

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", server.Addr, err)
	}
	url := fmt.Sprintf("http://localhost:%d/", portOf(ln.Addr()))
	printBanner(cfgPath, url, paths, cfg, createdAdmin, cfg.AdminUser, adminPass)

	if cfg.OpenBrowser {
		go openInBrowser(url)
	}

	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务异常退出: %v", err)
		}
	case <-quit:
		fmt.Println("\n正在关闭服务…")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}
}

func portOf(addr net.Addr) int {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.Port
	}
	return 8080
}

// displayWidth 计算字符串的显示宽度（中日韩字符按 2 列计）。
func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if r > 0x2E7F { // CJK 及其扩展区、全角符号
			w += 2
		} else {
			w++
		}
	}
	return w
}

func printBanner(cfgPath, url string, paths config.Paths, cfg *config.Config, createdAdmin bool, adminUser, adminPass string) {
	const boxWidth = 54
	title := fmt.Sprintf("%s 软件包管理系统 v%s", cfg.AppName, version)
	pad := boxWidth - displayWidth(title) - 4
	if pad < 0 {
		pad = 0
	}
	line := strings.Repeat("─", boxWidth)
	fmt.Println()
	fmt.Printf("  ┌%s┐\n", line)
	fmt.Printf("  │  %s%s │\n", title, strings.Repeat(" ", pad))
	fmt.Printf("  └%s┘\n", line)
	fmt.Printf("  平台名称     : %s\n", cfg.AppName)
	fmt.Printf("  访问地址     : %s\n", url)
	fmt.Printf("  配置文件     : %s\n", cfgPath)
	fmt.Printf("  数据目录     : %s\n", paths.DataDir)
	fmt.Printf("  软件存储目录 : %s\n", paths.StorageDir)
	fmt.Printf("  上传上限     : %d MB/文件\n", cfg.MaxUploadMB)
	fmt.Printf("  会话有效期   : %d 小时\n", cfg.SessionHours)
	if createdAdmin {
		fmt.Printf("  初始账号     : %s / %s （请登录后立即修改密码）\n", adminUser, adminPass)
	}
	fmt.Println("  停止服务     : Ctrl+C")
	fmt.Println()
}

func openInBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
