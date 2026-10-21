// Package config 负责 PkgHub 的配置文件加载、路径派生与默认模板生成。
//
// 配置优先级（由低到高）：内置默认值 → 配置文件 config.toml → 环境变量 → 命令行参数。
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// 默认取值。
const (
	DefaultAppName      = "PkgHub"
	DefaultAddr         = ":8080"
	DefaultDBName       = "pkghub.db"
	DefaultDataDirName  = "data"
	DefaultFilesDirName = "files"
	DefaultMaxUploadMB  = 2048
	DefaultSessionHours = 12
	DefaultAdminUser    = "admin"
	DefaultAdminPass    = "admin123"
	DefaultFileName     = "config.toml"
)

// Config 对应配置文件中的全部可配置项。
type Config struct {
	AppName       string `toml:"app_name"`       // 平台名称（显示在网页标题与登录页）
	Addr          string `toml:"addr"`           // HTTP 监听地址
	DataDir       string `toml:"data_dir"`       // 数据根目录
	DBPath        string `toml:"db_path"`        // SQLite 数据库路径
	StorageDir    string `toml:"storage_dir"`    // 软件包文件存储目录
	MaxUploadMB   int    `toml:"max_upload_mb"`  // 单文件上传上限（MB）
	SessionHours  int    `toml:"session_hours"`  // 会话有效期（小时）
	AdminUser     string `toml:"admin_user"`     // 初始管理员用户名
	AdminPassword string `toml:"admin_password"` // 初始管理员密码
	OpenBrowser   bool   `toml:"open_browser"`   // 启动后是否自动打开浏览器
}

// Paths 由 Config 派生出的实际运行路径。
type Paths struct {
	DataDir    string // 数据根目录
	DBPath     string // 数据库文件绝对路径
	StorageDir string // 软件包文件存储目录绝对路径
}

// Default 返回内置默认配置。
func Default() *Config {
	return &Config{
		AppName:      DefaultAppName,
		Addr:         DefaultAddr,
		MaxUploadMB:  DefaultMaxUploadMB,
		SessionHours: DefaultSessionHours,
		AdminUser:    DefaultAdminUser,
	}
}

// ConfigFilePath 推断默认配置文件路径（可执行文件同级目录）。
func ConfigFilePath(exeDir string) string {
	return filepath.Join(exeDir, DefaultFileName)
}

// Load 读取配置文件。
//
// path 为空或文件不存在时返回 exists=false 与默认配置；文件存在但解析失败时返回错误。
func Load(path string) (cfg *Config, exists bool, err error) {
	cfg = Default()
	if strings.TrimSpace(path) == "" {
		return cfg, false, nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return cfg, false, nil
		}
		return nil, false, fmt.Errorf("读取配置文件 %s 失败: %w", path, statErr)
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, true, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	if err := cfg.Normalize(); err != nil {
		return nil, true, fmt.Errorf("配置文件 %s 参数非法: %w", path, err)
	}
	return cfg, true, nil
}

// WriteIfAbsent 在配置文件不存在时写入带注释的默认模板，返回是否新建。
func WriteIfAbsent(path string, cfg *Config) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, errors.New("配置文件路径为空")
	}
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("检查配置文件失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(Render(cfg)), 0o644); err != nil {
		return false, fmt.Errorf("写入配置文件失败: %w", err)
	}
	return true, nil
}

// Normalize 清理空白并对关键参数做合法性校验。
func (c *Config) Normalize() error {
	c.AppName = strings.TrimSpace(c.AppName)
	if c.AppName == "" {
		c.AppName = DefaultAppName
	}
	if len(c.AppName) > 32 {
		c.AppName = strings.TrimSpace(string([]rune(c.AppName)[:32]))
	}
	c.Addr = strings.TrimSpace(c.Addr)
	if c.Addr == "" {
		c.Addr = DefaultAddr
	}
	c.DataDir = strings.TrimSpace(c.DataDir)
	c.DBPath = strings.TrimSpace(c.DBPath)
	c.StorageDir = strings.TrimSpace(c.StorageDir)
	if c.MaxUploadMB <= 0 {
		c.MaxUploadMB = DefaultMaxUploadMB
	}
	if c.SessionHours <= 0 {
		c.SessionHours = DefaultSessionHours
	}
	c.AdminUser = strings.TrimSpace(c.AdminUser)
	if c.AdminUser == "" {
		c.AdminUser = DefaultAdminUser
	}
	return nil
}

// MaxUploadBytes 返回上传上限对应的字节数。
func (c *Config) MaxUploadBytes() int64 { return int64(c.MaxUploadMB) * (1 << 20) }

// ResolvePaths 根据配置推导实际路径（相对路径基于当前工作目录），并确保目录存在。
func (c *Config) ResolvePaths(exeDir string) (Paths, error) {
	dataDir := c.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(exeDir, DefaultDataDirName)
	}
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		return Paths{}, fmt.Errorf("数据目录路径非法: %w", err)
	}
	if err := os.MkdirAll(absData, 0o755); err != nil {
		return Paths{}, fmt.Errorf("无法创建数据目录 %s: %w", absData, err)
	}

	dbPath := c.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(absData, DefaultDBName)
	}
	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		return Paths{}, fmt.Errorf("数据库路径非法: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absDB), 0o755); err != nil {
		return Paths{}, fmt.Errorf("无法创建数据库目录: %w", err)
	}

	storage := c.StorageDir
	if storage == "" {
		storage = filepath.Join(absData, DefaultFilesDirName)
	}
	absStorage, err := filepath.Abs(storage)
	if err != nil {
		return Paths{}, fmt.Errorf("存储目录路径非法: %w", err)
	}
	if err := os.MkdirAll(absStorage, 0o755); err != nil {
		return Paths{}, fmt.Errorf("无法创建存储目录 %s: %w", absStorage, err)
	}

	return Paths{DataDir: absData, DBPath: absDB, StorageDir: absStorage}, nil
}

// ApplyEnv 使用环境变量覆盖未设置的字段（envOnly=false 时为空才覆盖）。
func (c *Config) ApplyEnv() {
	if v := strings.TrimSpace(os.Getenv("PKGHUB_APP_NAME")); v != "" {
		c.AppName = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_ADDR")); v != "" {
		c.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_DATA_DIR")); v != "" {
		c.DataDir = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_DB_PATH")); v != "" {
		c.DBPath = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_STORAGE_DIR")); v != "" {
		c.StorageDir = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_MAX_UPLOAD")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.MaxUploadMB = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_SESSION_HOURS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.SessionHours = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_ADMIN_USER")); v != "" {
		c.AdminUser = v
	}
	if v := os.Getenv("PKGHUB_ADMIN_PASSWORD"); v != "" {
		c.AdminPassword = v
	}
	if v := strings.TrimSpace(os.Getenv("PKGHUB_OPEN_BROWSER")); v != "" {
		c.OpenBrowser = v == "1" || strings.EqualFold(v, "true")
	}
}

// Render 生成带注释的配置文件内容。
func Render(c *Config) string {
	src := *c
	if strings.TrimSpace(src.AppName) == "" {
		src.AppName = DefaultAppName
	}
	if src.Addr == "" {
		src.Addr = DefaultAddr
	}
	if src.MaxUploadMB <= 0 {
		src.MaxUploadMB = DefaultMaxUploadMB
	}
	if src.SessionHours <= 0 {
		src.SessionHours = DefaultSessionHours
	}
	if src.AdminUser == "" {
		src.AdminUser = DefaultAdminUser
	}
	return fmt.Sprintf(`# PkgHub 配置文件（TOML 格式）
# 1. 相对路径均相对于程序的「当前工作目录」，建议使用绝对路径。
# 2. 修改后需重启服务生效。
# 3. 生效优先级：内置默认值 < 本文件 < 环境变量 < 命令行参数。

# 平台名称，显示在浏览器标签页、登录页与侧边栏
app_name = %s

# HTTP 监听地址，形如 ":8080"（监听全部网卡）或 "127.0.0.1:9000"（仅本机）
addr = %s

# 数据根目录：存放 SQLite 数据库，并作为软件存储目录的默认父级。
# 留空则使用「可执行文件同级目录/data」
data_dir = %s

# SQLite 数据库文件路径；留空则使用 <data_dir>/pkghub.db
db_path = %s

# 软件包文件存储目录（上传的安装包实际存放位置）。
# 如需把大文件放到其它磁盘/分区，取消注释并填写具体路径即可；留空则使用 <data_dir>/files
storage_dir = %s

# 单个文件上传大小上限（MB）
max_upload_mb = %d

# 登录会话有效期（小时）
session_hours = %d

# 首次启动时自动创建的默认管理员
admin_user = %s
# 初始密码；留空则回退到环境变量 PKGHUB_ADMIN_PASSWORD，仍未设置则为 admin123
admin_password = %s

# 启动后是否自动打开浏览器
open_browser = %t
`, quote(src.AppName), quote(src.Addr), quote(src.DataDir), quote(src.DBPath), quote(src.StorageDir),
		src.MaxUploadMB, src.SessionHours, quote(src.AdminUser), quote(src.AdminPassword), src.OpenBrowser)
}

func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
