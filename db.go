package varroa

import (
	"github.com/asdine/storm"
	"github.com/asdine/storm/codec/msgpack"
)

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
