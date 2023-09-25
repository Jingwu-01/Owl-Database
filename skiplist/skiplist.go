package skiplist

import (
	"cmp"
	"sync"
	"sync/atomic"
)

// A struct representing a node in a skip list.
type node[K cmp.Ordered, V any] struct {
	sync.Mutex
	key         K
	value       V
	topLevel    int
	marked      atomic.Bool
	fullyLinked atomic.Bool
	next        []atomic.Pointer[node[K, V]]
}

// A struct representing a skiplist.
type SkipList[K cmp.Ordered, V any] struct {
	head     *node[K, V]
	totalOps atomic.Int32
}

// For query returns.
type Pair[K cmp.Ordered, V any] struct {
	Key   K
	Value V
}

// Creates an empty new skiplist object
func New[K cmp.Ordered, V any](minKey, maxKey K, max_level int) *SkipList[K, V] {
	var head, tail node[K, V]

	// Construct head node.
	head.key = minKey
	head.topLevel = max_level
	head.marked = atomic.Bool{}
	head.fullyLinked = atomic.Bool{}
	head.marked.Store(false)
	head.next = make([]atomic.Pointer[node[K, V]], max_level)

	// Construct tail node.
	tail.key = maxKey
	tail.topLevel = 0
	tail.marked = atomic.Bool{}
	tail.fullyLinked = atomic.Bool{}
	tail.marked.Store(false)
	tail.fullyLinked.Store(true)

	// Link head to tail
	for i := 0; i < max_level; i++ {
		head.next[i].Store(&tail)
	}
	// Set head to fully linked once its been linked to tail.
	head.fullyLinked.Swap(true)

	// Construct the skip list
	var ret SkipList[K, V]
	ret.head = &head
	ret.totalOps = atomic.Int32{}
	ret.totalOps.Store(0)

	return &ret
}

// Creates a new node object.
func newNode[K cmp.Ordered, V any](key K, val V, topLevel int) *node[K, V] {
	var newnode node[K, V]

	newnode.fullyLinked = atomic.Bool{}
	newnode.fullyLinked.Store(false)
	newnode.marked = atomic.Bool{}
	newnode.marked.Store(false)
	newnode.key = key
	newnode.value = val
	newnode.topLevel = topLevel
	// Will ask at LT if it can be to toplevel or has to be to max.
	newnode.next = make([]atomic.Pointer[node[K, V]], topLevel)

	return &newnode
}

// Helper method for Find, Upsert and Remove.
func (s *SkipList[K, V]) find(key K) (int, []atomic.Pointer[node[K, V]], []atomic.Pointer[node[K, V]]) {
	// Initialize vars for searching the list.
	foundLevel := -1
	pred := s.head
	level := pred.topLevel

	// Initialize return values
	preds := make([]atomic.Pointer[node[K, V]], level)
	succs := make([]atomic.Pointer[node[K, V]], level)

	// Find successor at each level.
	for level >= 0 {
		// Initialize current node.
		curr := pred.next[level].Load()

		// Look through this level of the list until we go past key.
		for key > curr.key {
			pred = curr
			curr = pred.next[level].Load()
		}

		// If we found key, indicate the highest level we found it - useful for remove.
		if foundLevel == -1 && key == curr.key {
			foundLevel = level
		}

		// Update preds, succs, and level.
		temp := atomic.Pointer[node[K, V]]{}
		temp.Store(pred)
		preds[level] = temp

		temp2 := atomic.Pointer[node[K, V]]{}
		temp.Store(curr)
		succs[level] = temp2

		level--
	}

	return foundLevel, preds, succs
}
