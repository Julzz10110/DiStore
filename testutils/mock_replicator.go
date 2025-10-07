package testutils

type MockReplicator struct {
	Nodes        []string
	ReplicaCount int
	SetCalls     int
	DeleteCalls  int
	UpdateCalls  int
}

func NewMockReplicator(nodes []string, replicaCount int) *MockReplicator {
	return &MockReplicator{Nodes: append([]string{}, nodes...), ReplicaCount: replicaCount}
}

func (m *MockReplicator) ReplicateSet(key, value string) error { m.SetCalls++; return nil }
func (m *MockReplicator) ReplicateDelete(key string) error     { m.DeleteCalls++; return nil }
func (m *MockReplicator) UpdateNodes(newNodes []string) {
	m.Nodes = append([]string{}, newNodes...)
	m.UpdateCalls++
}
func (m *MockReplicator) GetNodes() []string        { return append([]string{}, m.Nodes...) }
func (m *MockReplicator) GetReplicaCount() int      { return m.ReplicaCount }
func (m *MockReplicator) SetReplicaCount(count int) { m.ReplicaCount = count }
