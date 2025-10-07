package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type WALEntry struct {
	Operation string    `json:"op"` // "SET" or "DELETE"
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Sequence  uint64    `json:"sequence"`
}

type WriteAheadLog struct {
	file      *os.File
	filePath  string
	sequence  uint64
	mu        sync.Mutex
	batchSize int
}

func NewWriteAheadLog(dataDir string, batchSize int) (*WriteAheadLog, error) {
	walPath := filepath.Join(dataDir, "wal.log")

	file, err := os.OpenFile(walPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL file: %w", err)
	}

	// Recover sequence numbers from an existing file
	var sequence uint64
	if info, err := file.Stat(); err == nil && info.Size() > 0 {
		// set sequence to the number of rows
		// TODO: parse the last record
		sequence = uint64(info.Size() / 100) // approximate estimate
	}

	return &WriteAheadLog{
		file:      file,
		filePath:  walPath,
		sequence:  sequence,
		batchSize: batchSize,
	}, nil
}

func (wal *WriteAheadLog) LogSet(key, value string) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	entry := WALEntry{
		Operation: "SET",
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
		Sequence:  wal.sequence,
	}

	wal.sequence++
	return wal.writeEntry(entry)
}

func (wal *WriteAheadLog) LogDelete(key string) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	entry := WALEntry{
		Operation: "DELETE",
		Key:       key,
		Timestamp: time.Now(),
		Sequence:  wal.sequence,
	}

	wal.sequence++
	return wal.writeEntry(entry)
}

func (wal *WriteAheadLog) writeEntry(entry WALEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	data = append(data, '\n')
	_, err = wal.file.Write(data)
	return err
}

func (wal *WriteAheadLog) Close() error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	return wal.file.Close()
}

func (wal *WriteAheadLog) Recover(storage Storage) error {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	// close the current file and open it for reading
	wal.file.Close()

	file, err := os.Open(wal.filePath)
	if err != nil {
		return fmt.Errorf("failed to open WAL for recovery: %w", err)
	}
	defer file.Close()

	// read and apply the notes
	decoder := json.NewDecoder(file)
	for decoder.More() {
		var entry WALEntry
		if err := decoder.Decode(&entry); err != nil {
			return fmt.Errorf("failed to decode WAL entry: %w", err)
		}

		switch entry.Operation {
		case "SET":
			storage.Set(entry.Key, entry.Value)
		case "DELETE":
			storage.Delete(entry.Key)
		}
	}

	// reopen the file for new entries
	wal.file, err = os.OpenFile(wal.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen WAL: %w", err)
	}

	return nil
}

// WALStorage wrapper with write-ahead log
type WALStorage struct {
	Storage
	wal *WriteAheadLog
}

func NewWALStorage(base Storage, dataDir string) (*WALStorage, error) {
	wal, err := NewWriteAheadLog(dataDir, 1000)
	if err != nil {
		return nil, err
	}

	return &WALStorage{
		Storage: base,
		wal:     wal,
	}, nil
}

func (ws *WALStorage) Set(key, value string) error {
	// Firstly, write into WAL
	if err := ws.wal.LogSet(key, value); err != nil {
		return err
	}

	// then into the main storage
	return ws.Storage.Set(key, value)
}

func (ws *WALStorage) Delete(key string) error {
	if err := ws.wal.LogDelete(key); err != nil {
		return err
	}

	return ws.Storage.Delete(key)
}

func (ws *WALStorage) Close() error {
	if err := ws.wal.Close(); err != nil {
		return err
	}
	return ws.Storage.Close()
}
