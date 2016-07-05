package slqlite3

import (
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"log"
	"time"

	"github.com/realfake/shrtie"
)

const maxLength = 2048

var (
	ErrWrongKey error = errors.New("Wrong key")
	ErrTTL            = errors.New("TTL exceeded")
)

type Sqlite3 struct {
	insertStmt, incrStmt, getStmt, infoStmt *sql.Stmt
}

func New(db *sql.DB) (shrtie.GetSaver, error) {
	b := Sqlite3{}

	if err := (&b).prepare(db); err != nil {
		return nil, err
	}

	return b, nil
}

func (s Sqlite3) Get(key string) (string, error) {
	id, err := toInt64(key)
	if err != nil {
		return "", err
	}

	var url string
	var until int64
	if err = s.getStmt.QueryRow(id).Scan(&url, &until); err != nil {
		return "", nil
	}

	if until < time.Now().Unix() {
		return "", ErrTTL
	}

	return url, nil
}

func (s Sqlite3) Save(value string, ttl time.Duration) string {
	if len(value) > maxLength {
		return ""
	}

	var until int64
	now := time.Now()
	if ttl != 0 {
		until = now.Add(ttl).Unix()
	}

	res, err := s.insertStmt.Exec(value, until, now.Unix())
	if err != nil {
		return ""
	}

	// Make int64 to byte array and cut it to min lenght
	buf := make([]byte, 8)
	index, _ := res.LastInsertId()
	size := binary.PutVarint(buf, index)

	// Convert to base64, wich is URL save and without padding ('='*)
	return base64.RawURLEncoding.EncodeToString(buf[:size])
}

func (s Sqlite3) Info(key string) (*shrtie.Metadata, error) {
	id, err := toInt64(key)
	if err != nil {
		return nil, err
	}
	log.Println(id)

	var meta = &shrtie.Metadata{}
	var until, created int64
	err = s.infoStmt.QueryRow(id).Scan(&meta.URL, &until, &meta.Clicked, &created)

	log.Println(err)
	log.Println(meta, until, created)

	now := time.Now().Unix()
	if until > 0 {
		meta.TTL = until - now
	} else if until == 0 {
		meta.TTL = 0
	} else {
		return nil, ErrTTL
	}

	meta.Created = time.Unix(created, 0)

	return meta, nil
}

func toInt64(s string) (int64, error) {
	buf, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return 0, ErrWrongKey
	}

	id, _ := binary.Varint(buf)

	return id, nil
}

func (s *Sqlite3) prepare(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS shrtie_url (
			id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
			url TEXT NOT NULL,
			until INTEGER NOT NULL,
			count INTEGER DEFAULT 0 NOT NULL,
			created INTEGER NOT NULL);
	`)
	if err != nil {
		return err
	}

	s.insertStmt, err = db.Prepare(`
		INSERT INTO shrtie_url(url, until, created) VALUES (?,?,?);
	`)
	if err != nil {
		return err
	}

	s.incrStmt, err = db.Prepare(`
		UPDATE shrtie_url SET count = count + 1 WHERE id = ?;
	`)
	if err != nil {
		return err
	}

	s.getStmt, err = db.Prepare(`
		SELECT url, until FROM shrtie_url
			WHERE id = ?;
	`)

	s.infoStmt, err = db.Prepare(`
		SELECT url, until, count, created FROM shrtie_url
			WHERE id = ?;
	`)
	if err != nil {
		return err
	}

	return nil
}
