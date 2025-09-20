.PHONY: test test-replication test-storage test-all

test-replication:
	go test ./replication -v -timeout=30s

test-storage:
	go test ./storage -v -timeout=10s

test-all:
	go test ./... -v -timeout=60s

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out