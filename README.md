# PkgHub · 轻量级软件包管理系统

> 使用 **Go + SQLite** 实现的软件包管理 Web 程序，单一可执行文件即可运行，零依赖、零配置。

PkgHub 提供软件包的多版本上传与下载、分类管理、用户与权限管理，并自带一套现代化的单页界面（深 / 浅色主题）。所有静态资源通过 `go:embed` 内嵌，最终只需发布一个独立的二进制文件。

## ✨ 功能特性

- 📦 **软件包多版本管理**：同一软件包可上传任意多个版本，自动计算 SHA-256 校验和，支持断点续传下载。
- 🗂️ **分类管理**：可自定义软件包分类，支持描述、主题色，下含软件包时禁止误删。
- 👥 **用户与权限**：内置管理员 / 普通用户两种角色；管理员可管理分类与全部软件包，普通用户可上传并管理自己创建的软件包。
- 🚀 **零部署**：单个二进制文件即可启动；端口、上传上限、数据与软件存储位置均可通过 `config.toml`、环境变量或命令行参数配置。
- 🎨 **现代界面**：玻璃态顶栏、渐变品牌色、卡片化布局、动画过渡，自带暗色主题并跟随系统；软件包列表支持**卡片 / 列表双视图**切换（选择会被记住）。
- ⚡ **上传体验**：拖拽上传、实时进度条、上限校验、SHA-256 自动计算。
- 🔐 **安全**：基于 Cookie 的会话（HttpOnly + SameSite），密码使用 bcrypt 加盐哈希。

## 🚀 快速开始

### 下载发布版本

从 [Releases](#) 获取 `pkghub-<os>-amd64[.exe]` 后直接运行：

```bash
# Linux / macOS
chmod +x pkghub-linux-amd64
./pkghub-linux-amd64

# Windows
pkghub.exe
```

默认监听 `:8080`，首次启动会在可执行文件同级生成 `config.toml`（带注释的默认模板，可按需修改），并在 `data/` 目录创建 SQLite 数据库与文件存储，同时输出初始管理员账号：

```
初始账号 : admin / admin123 （请登录后立即修改密码）
```

> 可通过环境变量 `PKGHUB_ADMIN_PASSWORD` 或启动参数 `-admin-password` 自定义初始密码。

打开浏览器访问 <http://localhost:8080/> 即可使用。

### 从源码构建

```bash
# 默认构建（输出 pkghub / pkghub.exe）
make build            # 或 go build -o pkghub .

# 跨平台构建（输出 dist/）
make build-all        # 或 ./build.sh

# 自定义版本号
build.bat 1.2.0
./build.sh 1.2.0
```

## ⚙️ 配置

PkgHub 支持 **配置文件 / 环境变量 / 命令行参数** 三种配置方式，生效优先级：

```
内置默认值  <  配置文件 config.toml  <  环境变量  <  命令行参数
```

首次启动时，若配置文件不存在会自动生成一份带中文注释的默认模板 `config.toml`（默认位于**可执行文件同级目录**），修改后重启服务即可生效。

### 1. 配置文件（推荐）

```toml
# PkgHub 配置文件（TOML 格式）
# 相对路径均相对于程序的「当前工作目录」，建议使用绝对路径。

# 平台名称，显示在浏览器标签页、登录页与侧边栏（最长 32 个字符）
app_name = "PkgHub"

# HTTP 监听地址，形如 ":8080"（监听全部网卡）或 "127.0.0.1:9000"（仅本机）
addr = ":8080"

# 数据根目录：存放 SQLite 数据库，并作为软件存储目录的默认父级。
# 留空则使用「可执行文件同级目录/data」
data_dir = ""

# SQLite 数据库文件路径；留空则使用 <data_dir>/pkghub.db
db_path = ""

# 软件包文件存储目录（上传的安装包实际存放位置）。
# 需把大文件放到其它磁盘/分区时，填写具体路径即可；留空则使用 <data_dir>/files
storage_dir = ""

# 单个文件上传大小上限（MB）
max_upload_mb = 2048

# 登录会话有效期（小时）
session_hours = 12

# 首次启动时自动创建的默认管理员
admin_user = "admin"
admin_password = ""

# 启动后是否自动打开浏览器
open_browser = false
```

查看当前实际生效的配置（含路径推导结果）：

```bash
pkghub -print-config
```

### 2. 环境变量

| 变量 | 说明 |
| --- | --- |
| `PKGHUB_CONFIG` | 指定配置文件路径 |
| `PKGHUB_APP_NAME` | 平台名称 |
| `PKGHUB_ADDR` | 监听地址 |
| `PKGHUB_DATA_DIR` | 数据根目录 |
| `PKGHUB_DB_PATH` | 数据库文件路径 |
| `PKGHUB_STORAGE_DIR` | 软件包文件存储目录 |
| `PKGHUB_MAX_UPLOAD` | 上传上限（MB） |
| `PKGHUB_SESSION_HOURS` | 会话有效期（小时） |
| `PKGHUB_ADMIN_USER` / `PKGHUB_ADMIN_PASSWORD` | 初始管理员账号 / 密码 |
| `PKGHUB_OPEN_BROWSER` | 是否自动打开浏览器（`1` / `true`） |

### 3. 命令行参数

命令行参数优先级最高，适合作临时覆盖。

| 参数 | 说明 |
| --- | --- |
| `-config` | 配置文件路径（默认：可执行文件同级目录下的 `config.toml`） |
| `-app-name` | 平台名称（网页标题、登录页、侧边栏） |
| `-addr` | HTTP 监听地址 |
| `-data` | 数据目录 |
| `-db` | SQLite 数据库文件路径 |
| `-storage` | 软件包文件存储目录 |
| `-max-upload` | 单个文件上传上限（MB） |
| `-session-hours` | 会话有效期（小时） |
| `-admin-user` / `-admin-password` | 初始管理员用户名 / 密码 |
| `-open` | 启动后自动打开浏览器 |
| `-print-config` | 打印生效的完整配置后退出 |
| `-version` | 打印版本信息后退出 |

示例：临时换端口，并把安装包放到独立的数据盘

```bash
pkghub -addr 127.0.0.1:9000 -storage /mnt/nas/packages -max-upload 512
```

## 📁 数据目录结构

```
config.toml                        # 配置文件（首启自动生成，与可执行文件同级）
data/                              # 数据根目录，由 data_dir 指定
└── pkghub.db                      # SQLite 数据库（含 -wal / -shm 运行时文件）
D:\<storage_dir>/                  # 软件存储目录，由 storage_dir 指定；留空则为 data/files
└── packages/
    └── <package_id>/
        └── <random>-<filename>    # 每个版本一个文件
```

路径均可在 `config.toml` 中自定义，例如把大体积安装包放到独立的磁盘：

```toml
data_dir = "D:/PkgHubData"
storage_dir = "E:/PkgHubFiles"
```

备份时整体迁移 `data_dir` 与 `storage_dir` 两个目录即可。

## 🔌 REST API 概览

所有接口统一返回 `{ "ok": true|false, "data": ..., "error": ... }`。需要登录的接口读取名为 `pkgmgr_session` 的 Cookie。

| 模块 | 接口 | 说明 |
| --- | --- | --- |
| 认证 | `POST /api/auth/login` `POST /api/auth/logout` `GET /api/auth/me` | 登录、登出、当前账号信息 |
| 概览 | `GET /api/stats/overview` | 仪表盘统计 |
| 分类 | `GET/POST /api/categories`、`PUT/DELETE /api/categories/{id}` | 分类 CRUD（写入需管理员） |
| 软件包 | `GET/POST /api/packages`、`GET/PUT/DELETE /api/packages/{id}` | 软件包 CRUD |
| 版本 | `GET /api/packages/{id}/versions`、`POST /api/packages/{id}/versions`（multipart） | 版本列表、上传 |
| 下载 | `GET /api/versions/{id}/download`、`GET /api/packages/{id}/download` | 下载指定版本 / 最新版本 |
| 用户 | `GET/POST /api/users`、`PUT/DELETE /api/users/{id}`、`POST /api/users/{id}/password` | 用户管理（管理员） |
| 个人 | `PUT /api/account/profile`、`POST /api/account/password` | 修改昵称 / 密码 |
| 系统 | `GET /healthz` | 健康检查 |

完整 API 与请求参数可在浏览器中查看 `internal/web/*.go` 源码。

## 🧱 目录结构

```
.
├── main.go                 # 程序入口：配置合并、启动、优雅关闭
├── internal/config/        # 配置文件加载、路径推导与默认模板生成
├── internal/store/         # SQLite 数据访问层（用户、分类、软件包、版本）
├── internal/web/           # HTTP 服务：路由、API、上传下载、模板
│   └── ui/                 # go:embed 的前端资源（HTML/CSS/JS）
├── build.bat / build.sh    # 构建脚本
├── Makefile
└── go.mod
```

## 🛡️ 安全注意事项

- 默认监听 `0.0.0.0:8080`，对外暴露前请使用反向代理（Nginx / Caddy）并启用 HTTPS。
- 生产环境请务必通过 `-admin-password` 或环境变量修改初始管理员密码。
- 上传文件大小由 `-max-upload` 控制，默认 2GB（按需调小）；文件直接存放在本地文件系统。
- 项目使用了纯 Go 的 SQLite 驱动（`modernc.org/sqlite`），无需 CGO，可在任何主流平台上运行。

## 📝 License

MIT
