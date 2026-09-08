package store

import (
	"database/sql"
	"sync"

	_ "modernc.org/sqlite"
)

type Service struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Group       string `json:"group"`
	Type        string `json:"type"` // jar | custom
	Path        string `json:"path"`
	WorkDir     string `json:"workDir"`
	JavaOpts    string `json:"javaOpts"`
	Command     string `json:"command"`       // custom 类型：启动命令，如 python app.py
	Description string `json:"description"`   // 服务说明注释
	Port        int    `json:"port"`
	HealthURL   string `json:"healthUrl"`
	AutoRestart bool   `json:"autoRestart"`
	Enabled     bool   `json:"enabled"`
}

type Store struct {
	db *sql.DB
	mu sync.Mutex
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	schema := `CREATE TABLE IF NOT EXISTS services (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		group_name TEXT DEFAULT 'default',
		type TEXT DEFAULT 'jar',
		path TEXT UNIQUE NOT NULL,
		work_dir TEXT DEFAULT '',
		java_opts TEXT DEFAULT '',
		port INTEGER DEFAULT 0,
		health_url TEXT DEFAULT '',
		auto_restart INTEGER DEFAULT 1,
		enabled INTEGER DEFAULT 1
	);`
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	// 旧库迁移：新增列（已存在则忽略报错）
	db.Exec(`ALTER TABLE services ADD COLUMN description TEXT DEFAULT ''`)
	db.Exec(`ALTER TABLE services ADD COLUMN command TEXT DEFAULT ''`)
	s.initSettings()
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

const svcCols = `id,name,group_name,type,path,work_dir,java_opts,command,description,port,health_url,auto_restart,enabled`

func scanSvc(row interface{ Scan(...interface{}) error }) (*Service, error) {
	sv := &Service{}
	var ar, en int
	if err := row.Scan(&sv.ID, &sv.Name, &sv.Group, &sv.Type, &sv.Path, &sv.WorkDir, &sv.JavaOpts,
		&sv.Command, &sv.Description, &sv.Port, &sv.HealthURL, &ar, &en); err != nil {
		return nil, err
	}
	sv.AutoRestart = ar == 1
	sv.Enabled = en == 1
	return sv, nil
}

func (s *Store) Upsert(sv *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO services (id,name,group_name,type,path,work_dir,java_opts,command,description,port,health_url,auto_restart,enabled)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET name=excluded.name, group_name=excluded.group_name,
		work_dir=excluded.work_dir, java_opts=excluded.java_opts, port=excluded.port,
		health_url=excluded.health_url, auto_restart=excluded.auto_restart, enabled=excluded.enabled`,
		sv.ID, sv.Name, sv.Group, sv.Type, sv.Path, sv.WorkDir, sv.JavaOpts, sv.Command, sv.Description,
		sv.Port, sv.HealthURL, b2i(sv.AutoRestart), b2i(sv.Enabled))
	return err
}

func (s *Store) Update(sv *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE services SET name=?,group_name=?,type=?,work_dir=?,java_opts=?,command=?,description=?,port=?,health_url=?,auto_restart=?,enabled=? WHERE id=?`,
		sv.Name, sv.Group, sv.Type, sv.WorkDir, sv.JavaOpts, sv.Command, sv.Description, sv.Port, sv.HealthURL,
		b2i(sv.AutoRestart), b2i(sv.Enabled), sv.ID)
	return err
}

func (s *Store) GetAll() ([]*Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT ` + svcCols + ` FROM services ORDER BY group_name,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Service
	for rows.Next() {
		sv, err := scanSvc(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) GetByID(id string) (*Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, err := scanSvc(s.db.QueryRow(`SELECT `+svcCols+` FROM services WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sv, err
}

func (s *Store) GetByPath(path string) (*Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv, err := scanSvc(s.db.QueryRow(`SELECT `+svcCols+` FROM services WHERE path=?`, path))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sv, err
}

func (s *Store) Count() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM services`).Scan(&n)
	return n, err
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) initSettings() {
	s.db.Exec(`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`)
}

func (s *Store) GetSetting(key string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var v string
	s.db.QueryRow(`SELECT value FROM settings WHERE key=?`, key).Scan(&v)
	return v
}

func (s *Store) SetSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO settings (key,value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM services WHERE id=?`, id)
	return err
}
