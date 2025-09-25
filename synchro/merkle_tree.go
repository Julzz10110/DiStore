package synchro

import (
	"crypto/sha1"
	"encoding/hex"
	"sort"
)

type MerkleTree struct {
	root   *MerkleNode
	leaves []*MerkleNode
}

type MerkleNode struct {
	Hash  string
	Left  *MerkleNode
	Right *MerkleNode
}

type MerkleDiff struct {
	Key    string
	Reason string
}

func NewMerkleTree(keys []string) *MerkleTree {
	if len(keys) == 0 {
		return &MerkleTree{
			root:   &MerkleNode{Hash: hashData("")},
			leaves: []*MerkleNode{},
		}
	}

	// Сортируем ключи для детерминированного дерева
	sort.Strings(keys)
	leaves := make([]*MerkleNode, len(keys))

	for i, key := range keys {
		leaves[i] = &MerkleNode{Hash: hashData(key)}
	}

	root := buildTree(leaves)
	return &MerkleTree{root: root, leaves: leaves}
}

func buildTree(nodes []*MerkleNode) *MerkleNode {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var newLevel []*MerkleNode
	for i := 0; i < len(nodes); i += 2 {
		if i+1 < len(nodes) {
			left := nodes[i]
			right := nodes[i+1]
			combinedHash := hashData(left.Hash + right.Hash)
			newLevel = append(newLevel, &MerkleNode{
				Hash:  combinedHash,
				Left:  left,
				Right: right,
			})
		} else {
			// Нечетное количество - дублируем последний узел
			combinedHash := hashData(nodes[i].Hash + nodes[i].Hash)
			newLevel = append(newLevel, &MerkleNode{
				Hash: combinedHash,
				Left: nodes[i],
			})
		}
	}

	return buildTree(newLevel)
}

func hashData(data string) string {
	hash := sha1.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (mt *MerkleTree) RootHash() string {
	if mt.root == nil {
		return ""
	}
	return mt.root.Hash
}

func CompareTrees(tree1, tree2 *MerkleTree) []MerkleDiff {
	if tree1.RootHash() == tree2.RootHash() {
		return nil // Деревья идентичны
	}

	return findDifferences(tree1.root, tree2.root, "")
}

func findDifferences(node1, node2 *MerkleNode, path string) []MerkleDiff {
	var diffs []MerkleDiff

	if node1 == nil && node2 == nil {
		return diffs
	}

	if node1 == nil {
		diffs = append(diffs, MerkleDiff{
			Key:    path,
			Reason: "node missing in first tree",
		})
		return diffs
	}

	if node2 == nil {
		diffs = append(diffs, MerkleDiff{
			Key:    path,
			Reason: "node missing in second tree",
		})
		return diffs
	}

	if node1.Hash != node2.Hash {
		if node1.Left == nil && node1.Right == nil {
			// Это лист - значит ключи разные
			diffs = append(diffs, MerkleDiff{
				Key:    path,
				Reason: "key difference detected",
			})
		} else {
			// Рекурсивно проверяем детей
			diffs = append(diffs, findDifferences(node1.Left, node2.Left, path+"L")...)
			diffs = append(diffs, findDifferences(node1.Right, node2.Right, path+"R")...)
		}
	}

	return diffs
}
