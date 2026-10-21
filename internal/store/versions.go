package store

import (
	"errors"
	"strings"
)

const versionColumns = `v.id, v.package_id, COALESCE(p.name, ''), v.version, v.file_name, v.file_path, v.size,
	v.sha256, v.downloads, v.notes, COALESCE(v.uploader_id, 0), COALESCE(NULLIF(u.display_name, ''), u.username, '—'), v.created_at`

const versionFrom = ` FROM versions v
	LEFT JOIN packages p ON p.id = v.package_id
	LEFT JOIN users u ON u.id = v.uploader_id`

func scanVersion(row interface{ Scan(...any) error }) (*Version, error) {
	var v Version
	if err := row.Scan(&v.ID, &v.PackageID, &v.PackageName, &v.Version, &v.FileName, &v.FilePath, &v.Size,
		&v.SHA256, &v.Downloads, &v.Notes, &v.UploaderID, &v.Uploader, &v.CreatedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

// CreateVersion 记录一个新版本（调用方需保证文件已写入磁盘）。
func (s *Store) CreateVersion(packageID int64, version, fileName, filePath, sha256, notes string, size, uploaderID int64) (*Version, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, ErrInvalid
	}
	res, err := s.db.Exec(`INSERT INTO versions (package_id, version, file_name, file_path, size, sha256, downloads, notes, uploader_id, created_at)
		VALUES (?,?,?,?,?,?,0,?,?,?)`,
		packageID, version, fileName, filePath, size, sha256, notes, uploaderID, Now())
	if err != nil {
		if isUniqueErr(err) {
			return nil, errors.New("该版本号已存在")
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	if err := s.TouchPackage(packageID); err != nil {
		return nil, err
	}
	return s.GetVersion(id)
}

// GetVersion 按 ID 查询版本。
func (s *Store) GetVersion(id int64) (*Version, error) {
	row := s.db.QueryRow(`SELECT `+versionColumns+versionFrom+` WHERE v.id = ?`, id)
	v, err := scanVersion(row)
	if err != nil {
		return nil, ErrNotFound
	}
	return v, nil
}

// ListVersions 返回指定软件包的全部版本（按创建时间倒序）。
func (s *Store) ListVersions(packageID int64) ([]Version, error) {
	rows, err := s.db.Query(`SELECT `+versionColumns+versionFrom+` WHERE v.package_id = ? ORDER BY v.created_at DESC, v.id DESC`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Version{}
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *v)
	}
	return list, rows.Err()
}

// LatestVersion 返回软件包的最新版本（按创建时间）。
func (s *Store) LatestVersion(packageID int64) (*Version, error) {
	row := s.db.QueryRow(`SELECT `+versionColumns+versionFrom+` WHERE v.package_id = ? ORDER BY v.created_at DESC, v.id DESC LIMIT 1`, packageID)
	return scanVersion(row)
}

// DeleteVersion 删除版本记录，返回被删除的记录以便清理文件。
func (s *Store) DeleteVersion(id int64) (*Version, error) {
	v, err := s.GetVersion(id)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(`DELETE FROM versions WHERE id = ?`, id); err != nil {
		return nil, err
	}
	_ = s.TouchPackage(v.PackageID)
	return v, nil
}

// BumpDownloads 递增下载次数。
func (s *Store) BumpDownloads(id int64) error {
	_, err := s.db.Exec(`UPDATE versions SET downloads = downloads + 1 WHERE id = ?`, id)
	return err
}

// PackageFilePaths 返回软件包下所有版本的文件相对路径，用于删除时清理磁盘。
func (s *Store) PackageFilePaths(packageID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT file_path FROM versions WHERE package_id = ?`, packageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
