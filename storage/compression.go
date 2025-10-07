package storage

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"strings"
	"sync"
)

type CompressionType string

const (
	CompressionNone CompressionType = "none"
	CompressionGZIP CompressionType = "gzip"
	CompressionZlib CompressionType = "zlib"
)

type CompressedStorage struct {
	Storage
	compressionType CompressionType
	threshold       int // min size for compression
	mu              sync.RWMutex
}

func NewCompressedStorage(base Storage, compType CompressionType, threshold int) *CompressedStorage {
	return &CompressedStorage{
		Storage:         base,
		compressionType: compType,
		threshold:       threshold,
	}
}

func (cs *CompressedStorage) Set(key, value string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// compress the value if it is large enough
	if len(value) > cs.threshold {
		compressed, err := cs.compress(value)
		if err != nil {
			return fmt.Errorf("compression failed: %w", err)
		}
		value = compressed
	}

	return cs.Storage.Set(key, value)
}

func (cs *CompressedStorage) Get(key string) (string, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	value, err := cs.Storage.Get(key)
	if err != nil {
		return "", err
	}

	// check whether the value is compressed and decompress it if necessary
	if cs.isCompressed(value) {
		decompressed, err := cs.decompress(value)
		if err != nil {
			return "", fmt.Errorf("decompression failed: %w", err)
		}
		return decompressed, nil
	}

	return value, nil
}

func (cs *CompressedStorage) compress(data string) (string, error) {
	var buf bytes.Buffer
	var writer io.Writer

	switch cs.compressionType {
	case CompressionGZIP:
		writer = gzip.NewWriter(&buf)
	case CompressionZlib:
		writer = zlib.NewWriter(&buf)
	default:
		return data, nil
	}

	if _, err := writer.Write([]byte(data)); err != nil {
		return "", err
	}

	if closer, ok := writer.(io.Closer); ok {
		closer.Close()
	}

	// add a prefix to indicate compression
	return string(cs.getCompressionPrefix()) + buf.String(), nil
}

func (cs *CompressedStorage) decompress(data string) (string, error) {
	// remove the compression prefix
	compressedData := data[len(cs.getCompressionPrefix()):]
	buf := bytes.NewBufferString(compressedData)
	var reader io.Reader
	var err error

	switch cs.compressionType {
	case CompressionGZIP:
		reader, err = gzip.NewReader(buf)
	case CompressionZlib:
		reader, err = zlib.NewReader(buf)
	default:
		return data, nil
	}

	if err != nil {
		return "", err
	}

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(decompressed), nil
}

func (cs *CompressedStorage) isCompressed(data string) bool {
	return strings.HasPrefix(data, string(cs.getCompressionPrefix()))
}

func (cs *CompressedStorage) getCompressionPrefix() []byte {
	switch cs.compressionType {
	case CompressionGZIP:
		return []byte{0x1F, 0x8B} // GZIP magic number
	case CompressionZlib:
		return []byte{0x78} // Zlib magic number
	default:
		return []byte{}
	}
}

// Method for monitoring compression efficiency
func (cs *CompressedStorage) GetCompressionStats() map[string]interface{} {
	items, err := cs.Storage.GetAll()
	if err != nil {
		return nil
	}

	totalOriginalSize := 0
	totalCompressedSize := 0
	compressedItems := 0

	for _, item := range items {
		totalOriginalSize += len(item.Value)
		if cs.isCompressed(item.Value) {
			compressedItems++
			// approximate size without prefix
			totalCompressedSize += len(item.Value) - len(cs.getCompressionPrefix())
		} else {
			totalCompressedSize += len(item.Value)
		}
	}

	compressionRatio := 1.0
	if totalOriginalSize > 0 {
		compressionRatio = float64(totalCompressedSize) / float64(totalOriginalSize)
	}

	return map[string]interface{}{
		"total_items":           len(items),
		"compressed_items":      compressedItems,
		"total_original_size":   totalOriginalSize,
		"total_compressed_size": totalCompressedSize,
		"compression_ratio":     compressionRatio,
		"savings_percent":       (1 - compressionRatio) * 100,
	}
}
