package store

// Role 用户角色。
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User 系统用户。
type User struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        Role   `json:"role"`
	Status      int    `json:"status"` // 1 启用，0 禁用
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	LastLoginAt string `json:"last_login_at"`
}

// IsAdmin 判断是否为管理员。
func (u *User) IsAdmin() bool { return u != nil && u.Role == RoleAdmin }

// IsActive 判断账号是否启用。
func (u *User) IsActive() bool { return u != nil && u.Status == 1 }

// Category 软件包分类。
type Category struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	Color        string `json:"color"`
	CreatedAt    string `json:"created_at"`
	PackageCount int64  `json:"package_count"`
}

// Package 软件包（同一软件包可包含多个版本）。
type Package struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Slug          string `json:"slug"`
	Description   string `json:"description"`
	CategoryID    int64  `json:"category_id"`
	CategoryName  string `json:"category_name"`
	CategoryColor string `json:"category_color"`
	OwnerID       int64  `json:"owner_id"`
	OwnerName     string `json:"owner_name"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	VersionCount  int64  `json:"version_count"`
	Downloads     int64  `json:"downloads"`
	TotalSize     int64  `json:"total_size"`
	LatestVersion string `json:"latest_version"`
}

// Version 软件包的某个具体版本（对应一个上传文件）。
type Version struct {
	ID          int64  `json:"id"`
	PackageID   int64  `json:"package_id"`
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	FileName    string `json:"file_name"`
	FilePath    string `json:"-"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Downloads   int64  `json:"downloads"`
	Notes       string `json:"notes"`
	UploaderID  int64  `json:"uploader_id"`
	Uploader    string `json:"uploader_name"`
	CreatedAt   string `json:"created_at"`
}

// Overview 首页统计信息。
type Overview struct {
	PackageCount  int64             `json:"package_count"`
	VersionCount  int64             `json:"version_count"`
	Downloads     int64             `json:"downloads"`
	TotalSize     int64             `json:"total_size"`
	UserCount     int64             `json:"user_count"`
	CategoryStat  []CategoryStat    `json:"category_stats"`
	RecentVersion []RecentVersion   `json:"recent_versions"`
	TopPackages   []PackageDownload `json:"top_packages"`
}

// CategoryStat 分类维度的统计。
type CategoryStat struct {
	CategoryID   int64  `json:"category_id"`
	CategoryName string `json:"category_name"`
	Color        string `json:"color"`
	PackageCount int64  `json:"package_count"`
	VersionCount int64  `json:"version_count"`
}

// RecentVersion 最近上传的版本。
type RecentVersion struct {
	ID          int64  `json:"id"`
	PackageID   int64  `json:"package_id"`
	PackageName string `json:"package_name"`
	Version     string `json:"version"`
	FileName    string `json:"file_name"`
	Size        int64  `json:"size"`
	Uploader    string `json:"uploader_name"`
	CreatedAt   string `json:"created_at"`
}

// PackageDownload 下载排行。
type PackageDownload struct {
	PackageID int64  `json:"package_id"`
	Name      string `json:"name"`
	Downloads int64  `json:"downloads"`
}
