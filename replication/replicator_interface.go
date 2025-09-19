package replication

// ReplicatorInterface defines the interface for replication
type ReplicatorInterface interface {
	ReplicateSet(key, value string) error
	ReplicateDelete(key string) error
	UpdateNodes(newNodes []string)
	GetNodes() []string
	GetReplicaCount() int
	SetReplicaCount(count int)
}

// Ensure Replicator implements ReplicatorInterface
var _ ReplicatorInterface = (*Replicator)(nil)
