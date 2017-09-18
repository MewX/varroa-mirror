package varroa

import (
	"path/filepath"
	"sync"

	"github.com/asdine/storm"
	"github.com/asdine/storm/codec/msgpack"
	"github.com/pkg/errors"
)

var database *Database
var onceDatabase sync.Once

type Database struct {
	path string
	DB   *storm.DB
}

func (db *Database) Open(path string) error {
	openedDatabase, err := storm.Open(path, storm.Codec(msgpack.Codec))
	if err != nil {
		return err
	}
	db.path = path
	db.DB = openedDatabase
	return nil
}

func (db *Database) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

func NewDatabase(path string) (*Database, error) {
	var err error
	onceDatabase.Do(func() {
		db := &Database{}
		if err = db.Open(filepath.Join(StatsDir, DefaultHistoryDB)); err != nil {
			err = errors.Wrap(err, "Error opening history database")
			return
		}
		database = db
	})
	return database, err
}
