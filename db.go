package varroa

import (
	"sync"

	"github.com/asdine/storm"
	"github.com/asdine/storm/codec/msgpack"
	"github.com/pkg/errors"
)

var database *Database
var onceDatabase sync.Once

// Database allows manupulating stats or release entries.
type Database struct {
	path string
	DB   *storm.DB
}

// Open the Database
func (db *Database) Open(path string) error {
	openedDatabase, err := storm.Open(path, storm.Codec(msgpack.Codec))
	if err != nil {
		return err
	}
	db.path = path
	db.DB = openedDatabase
	return nil
}

// Close the Database
func (db *Database) Close() error {
	if db.DB != nil {
		return db.DB.Close()
	}
	return nil
}

// NewDatabase opens the Database once.
func NewDatabase(path string) (*Database, error) {
	var err error
	onceDatabase.Do(func() {
		db := &Database{}
		if err = db.Open(path); err != nil {
			err = errors.Wrap(err, "Error opening history database")
			return
		}
		database = db
	})
	return database, err
}
