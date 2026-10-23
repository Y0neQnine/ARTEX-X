package db

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Webshell 是一条目标 shell 的登记记录：agent 拿到 shell 后通过
// register_webshell 工具登记，或用户在「Webshell」页手工添加。
type Webshell struct {
	ID         int64             `json:"id"`
	Name       string            `json:"name"`
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	ParamName  string            `json:"param_name"`
	Headers    map[string]string `json:"headers"`
	Encoding   string            `json:"encoding"`
	Note       string            `json:"note"`
	TaskID     *int64            `json:"task_id"`
	TaskName   string            `json:"task_name"`
	AssetID    *int64            `json:"asset_id"`
	LastOutput string            `json:"last_output"`
	LastExecAt *time.Time        `json:"last_exec_at"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// WebshellCommand 是 webshell 上的一条命令执行记录。
type WebshellCommand struct {
	ID         int64     `json:"id"`
	WebshellID int64     `json:"webshell_id"`
	Command    string    `json:"command"`
	Output     string    `json:"output"`
	Status     string    `json:"status"`
	DurationMS int       `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// WebshellStore 封装 webshells / webshell_commands 的 CRUD。
type WebshellStore struct{ db *DB }

// Webshells returns the webshell store.
func (d *DB) Webshells() *WebshellStore { return &WebshellStore{db: d} }

const webshellCols = `w.id, w.name, w.url, w.method, w.param_name, w.headers, w.encoding, w.note,
       w.task_id, COALESCE(t.name,'') AS task_name, w.asset_id, w.last_output, w.last_exec_at,
       w.created_at, w.updated_at`

const webshellFrom = ` FROM webshells w LEFT JOIN tasks t ON t.id = w.task_id`

func scanWebshell(row interface{ Scan(...any) error }) (Webshell, error) {
	var w Webshell
	var headers []byte
	err := row.Scan(&w.ID, &w.Name, &w.URL, &w.Method, &w.ParamName, &headers, &w.Encoding,
		&w.Note, &w.TaskID, &w.TaskName, &w.AssetID, &w.LastOutput, &w.LastExecAt,
		&w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return w, err
	}
	if len(headers) > 0 {
		_ = json.Unmarshal(headers, &w.Headers)
	}
	if w.Headers == nil {
		w.Headers = map[string]string{}
	}
	return w, nil
}

// ListWebshells returns all webshells, newest first.
func (s *WebshellStore) List() ([]Webshell, error) {
	rows, err := s.db.Query(`SELECT ` + webshellCols + webshellFrom + ` ORDER BY w.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Webshell{}
	for rows.Next() {
		w, err := scanWebshell(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Get returns one webshell (nil if absent).
func (s *WebshellStore) Get(id int64) (*Webshell, error) {
	w, err := scanWebshell(s.db.QueryRow(`SELECT `+webshellCols+webshellFrom+` WHERE w.id=$1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &w, err
}

// Create inserts a webshell and returns the full row.
func (s *WebshellStore) Create(name, url, method, paramName string, headers map[string]string, encoding, note string, taskID, assetID *int64) (Webshell, error) {
	raw, err := json.Marshal(headers)
	if err != nil {
		return Webshell{}, err
	}
	if len(headers) == 0 {
		raw = []byte(`{}`)
	}
	var id int64
	if err := s.db.QueryRow(`
INSERT INTO webshells(name, url, method, param_name, headers, encoding, note, task_id, asset_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		name, url, method, paramName, raw, encoding, note, taskID, assetID).Scan(&id); err != nil {
		return Webshell{}, err
	}
	w, err := s.Get(id)
	if err != nil {
		return Webshell{}, err
	}
	return *w, nil
}

// Update replaces all editable fields of an existing webshell.
func (s *WebshellStore) Update(id int64, name, url, method, paramName string, headers map[string]string, encoding, note string, taskID, assetID *int64) (Webshell, error) {
	raw, err := json.Marshal(headers)
	if err != nil {
		return Webshell{}, err
	}
	if len(headers) == 0 {
		raw = []byte(`{}`)
	}
	res, err := s.db.Exec(`
UPDATE webshells
   SET name=$2, url=$3, method=$4, param_name=$5, headers=$6, encoding=$7, note=$8, task_id=$9, asset_id=$10
 WHERE id=$1`,
		id, name, url, method, paramName, raw, encoding, note, taskID, assetID)
	if err != nil {
		return Webshell{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Webshell{}, sql.ErrNoRows
	}
	w, err := s.Get(id)
	if err != nil {
		return Webshell{}, err
	}
	return *w, nil
}

// Delete removes a webshell (command history cascades).
func (s *WebshellStore) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM webshells WHERE id=$1`, id)
	return err
}

// RecordExec appends a command execution to the history and refreshes the
// shell's last_output / last_exec_at, atomically.
func (s *WebshellStore) RecordExec(webshellID int64, command, output, status string, durationMS int) (WebshellCommand, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return WebshellCommand{}, err
	}
	defer tx.Rollback()
	var c WebshellCommand
	err = tx.QueryRow(`
INSERT INTO webshell_commands(webshell_id, command, output, status, duration_ms)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, webshell_id, command, output, status, duration_ms, created_at`,
		webshellID, command, output, status, durationMS).
		Scan(&c.ID, &c.WebshellID, &c.Command, &c.Output, &c.Status, &c.DurationMS, &c.CreatedAt)
	if err != nil {
		return WebshellCommand{}, err
	}
	if _, err := tx.Exec(`UPDATE webshells SET last_output=$2, last_exec_at=NOW() WHERE id=$1`, webshellID, output); err != nil {
		return WebshellCommand{}, err
	}
	return c, tx.Commit()
}

// ListCommands returns the newest limit commands for a webshell (DESC order).
func (s *WebshellStore) ListCommands(webshellID int64, limit int) ([]WebshellCommand, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := s.db.Query(`
SELECT id, webshell_id, command, output, status, duration_ms, created_at
  FROM webshell_commands WHERE webshell_id=$1 ORDER BY id DESC LIMIT $2`, webshellID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WebshellCommand{}
	for rows.Next() {
		var c WebshellCommand
		if err := rows.Scan(&c.ID, &c.WebshellID, &c.Command, &c.Output, &c.Status, &c.DurationMS, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
