package store

// Overview 汇总首页所需的统计数据。
func (s *Store) Overview() (*Overview, error) {
	var o Overview
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM packages`).Scan(&o.PackageCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM versions`).Scan(&o.VersionCount); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COALESCE(SUM(downloads),0), COALESCE(SUM(size),0) FROM versions`).Scan(&o.Downloads, &o.TotalSize); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&o.UserCount); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(`SELECT COALESCE(c.id, 0), COALESCE(c.name, '未分类'), COALESCE(c.color, '#94a3b8'),
		COUNT(DISTINCT p.id), COUNT(v.id)
		FROM packages p
		LEFT JOIN categories c ON c.id = p.category_id
		LEFT JOIN versions v ON v.package_id = p.id
		GROUP BY COALESCE(c.id, 0) ORDER BY 4 DESC, 2`)
	if err != nil {
		return nil, err
	}
	o.CategoryStat = []CategoryStat{}
	for rows.Next() {
		var cs CategoryStat
		if err := rows.Scan(&cs.CategoryID, &cs.CategoryName, &cs.Color, &cs.PackageCount, &cs.VersionCount); err != nil {
			rows.Close()
			return nil, err
		}
		o.CategoryStat = append(o.CategoryStat, cs)
	}
	rows.Close()

	vrows, err := s.db.Query(`SELECT ` + versionColumns + versionFrom + ` ORDER BY v.created_at DESC, v.id DESC LIMIT 8`)
	if err != nil {
		return nil, err
	}
	defer vrows.Close()
	o.RecentVersion = []RecentVersion{}
	for vrows.Next() {
		v, err := scanVersion(vrows)
		if err != nil {
			return nil, err
		}
		o.RecentVersion = append(o.RecentVersion, RecentVersion{
			ID: v.ID, PackageID: v.PackageID, PackageName: v.PackageName, Version: v.Version,
			FileName: v.FileName, Size: v.Size, Uploader: v.Uploader, CreatedAt: v.CreatedAt,
		})
	}

	prows, err := s.db.Query(`SELECT p.id, p.name, COALESCE(SUM(v.downloads),0) AS d
		FROM packages p LEFT JOIN versions v ON v.package_id = p.id
		GROUP BY p.id ORDER BY d DESC, p.name LIMIT 5`)
	if err != nil {
		return nil, err
	}
	defer prows.Close()
	o.TopPackages = []PackageDownload{}
	for prows.Next() {
		var pd PackageDownload
		if err := prows.Scan(&pd.PackageID, &pd.Name, &pd.Downloads); err != nil {
			return nil, err
		}
		o.TopPackages = append(o.TopPackages, pd)
	}
	return &o, prows.Err()
}
