package storage

import "errors"

var (
	ErrKeyNotFound = errors.New("key not found")
	ErrKeyExists   = errors.New("key already exists")
)

type KeyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Storage interface {
	Set(key, value string) error
	Get(key string) (string, error)
	Delete(key string) error
	GetAll() ([]KeyValue, error)
	Close() error
}
