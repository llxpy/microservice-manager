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
	Type        string `json:"type"`
	Path        string `json:"path"`
	WorkDir     string `json:"workDir"`
	JavaOpts    string `json:"javaOpts"`
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
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Upsert(sv *Service) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT INTO services (id,name,group_name,type,path,work_dir,java_opts,port,health_url,auto_restart,enabled)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(path) DO UPDATE SET name=excluded.name, group_name=excluded.group_name,
		work_dir=excluded.work_dir, java_opts=excluded.java_opts, port=excluded.port,
		health_url=excluded.health_url, auto_restart=excluded.auto_restart, enabled=excluded.enabled`,
		sv.ID, sv.Name, sv.Group, sv.Type, sv.Path, sv.WorkDir, sv.JavaOpts, sv.Port, sv.HealthURL,
		b2i(sv.AutoRestart), b2i(sv.Enabled))
	return err
}

func (s *Store) GetAll() ([]*Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT id,name,group_name,type,path,work_dir,java_opts,port,health_url,auto_restart,enabled FROM services ORDER BY group_name,name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Service
	for rows.Next() {
		sv := &Service{}
		var ar, en int
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.Group, &sv.Type, &sv.Path, &sv.WorkDir, &sv.JavaOpts, &sv.Port, &sv.HealthURL, &ar, &en); err != nil {
			return nil, err
		}
		sv.AutoRestart = ar == 1
		sv.Enabled = en == 1
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) GetByPath(path string) (*Service, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sv := &Service{}
	var ar, en int
	err := s.db.QueryRow(`SELECT id,name,group_name,type,path,work_dir,java_opts,port,health_url,auto_restart,enabled FROM services WHERE path=?`, path).
		Scan(&sv.ID, &sv.Name, &sv.Group, &sv.Type, &sv.Path, &sv.WorkDir, &sv.JavaOpts, &sv.Port, &sv.HealthURL, &ar, &en)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sv.AutoRestart = ar == 1
	sv.Enabled = en == 1
	return sv, nil
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
