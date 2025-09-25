.PHONY: test test-replication test-storage test-all

test-replication:
	go test ./replication -v -timeout=30s

test-storage:
	go test ./storage -v -timeout=10s

test-cluster:
	go test ./cluster -v -timeout=30s

test-synchro:
	go test ./synchro -v -timeout=30s

test-failover:
	go test ./cluster ./synchro -v -timeout=60s

test-all:
	go test ./... -v -timeout=60s

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out