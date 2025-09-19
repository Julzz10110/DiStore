package replication

import "errors"

var (
	ErrReplicationFailed = errors.New("replication failed")
	ErrQuorumNotReached  = errors.New("quorum not reached")
	ErrNodeUnavailable   = errors.New("node unavailable")
)
