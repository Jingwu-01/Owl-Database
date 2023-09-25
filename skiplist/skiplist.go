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
	head      *node[K, V]
	max_level int
	totalOps  atomic.Int32
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
	ret.max_level = max_level
	ret.totalOps = atomic.Int32{}
	ret.totalOps.Store(0)

	return &ret
}
