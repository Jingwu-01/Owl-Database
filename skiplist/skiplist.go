package skiplist

import (
	"cmp"
	"context"
	"math/rand"
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
	totalOps *atomic.Int32
}

// For query returns.
type Pair[K cmp.Ordered, V any] struct {
	Key   K
	Value V
}

// For Upsert
type UpdateCheck[K cmp.Ordered, V any] func(key K, currValue V, exists bool) (newValue V, err error)

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
	ret.totalOps = &atomic.Int32{}
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
func (s SkipList[K, V]) find(key K) (int, []*node[K, V], []*node[K, V]) {
	// Initialize vars for searching the list.
	foundLevel := -1
	pred := s.head
	level := pred.topLevel

	// Initialize return values
	preds := make([]*node[K, V], level)
	succs := make([]*node[K, V], level)

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
		preds[level] = pred
		succs[level] = curr

		level--
	}

	return foundLevel, preds, succs
}

// Finds the value corresponding to key K in s.
func (s SkipList[K, V]) Find(key K) (V, bool) {
	levelFound, _, succs := s.find(key)

	if levelFound == -1 {
		var zero V
		return zero, false
	}

	found := succs[levelFound]
	return found.value, found.fullyLinked.Load() && !found.marked.Load()
}

func (s SkipList[K, V]) Upsert(key K, check UpdateCheck[K, V]) (updated bool, err error) {
	// Random top level
	// TODO: random?
	topLevel := rand.Intn(s.head.topLevel)

	// Keep trying insert
	for {
		// Check if already existing key
		levelFound, preds, succs := s.find(key)
		if levelFound != -1 {
			found := succs[levelFound]
			if !found.marked.Load() {
				// Node already exists (update case)
				// TODO: ensure that a node's 'real' value only exists at the lowest level
				// so only need to obtain found's lock
				found.Lock()

				// Recheck once lock is obtained
				if !found.marked.Load() {
					found.Unlock()
					continue
				}

				// Use updatecheck to either update or ignore
				newV, err := check(found.key, found.value, true)
				if err != nil {
					found.Unlock()
					return false, err
				} else {
					found.value = newV
					found.Unlock()
					s.totalOps.Add(1)
					return true, nil
				}
			}

			// Found node being removed; retry
			continue
		}

		// Key not found, Lock all predecessors
		// Decide to insert or not

		// TODO: right place to place?
		// declared for zero value
		var def V
		newV, err := check(key, def, false)
		if err != nil {
			return false, err
		}

		highestLocked := 1
		valid := true
		level := 0
		prevKey := key

		// Lock all predecessors
		for ; valid && level <= topLevel; level++ {
			if preds[level].key < prevKey {
				preds[level].Lock()
				prevKey = preds[level].key
			}

			highestLocked = level

			// Check pred/succ still valid
			unmarked := !preds[level].marked.Load() && !succs[level].marked.Load()
			connected := preds[level].next[level].Load() == succs[level]
			valid = unmarked && connected
		}

		if !valid {
			// Pred/succ changed. Unlock and retry
			for level = highestLocked; level >= 0; level-- {
				preds[level].Unlock()
			}

			continue
		}

		// Insert node
		// TODO: what is topLevel?
		node := newNode(key, newV, topLevel)

		// Set next pointers on each level
		for level = 0; level <= topLevel; level++ {
			node.next[level].Store(succs[level])
		}

		// Add to skip list from bottom up
		for level = 0; level <= topLevel; level++ {
			preds[level].next[level].Store(node)
		}

		node.fullyLinked.Store(true)

		// Unlock
		for level = highestLocked; level >= 0; level-- {
			preds[level].Unlock()
		}
		s.totalOps.Add(1)
		return true, nil
	}
}

func (s SkipList[K, V]) Remove(key K) (*node[K, V], bool) {
	isMarked := false
	topLevel := -1
	var victim *node[K, V]

	// Keep trying to remove until success/failure
	for {
		// Find victim to remove
		levelFound, preds, succs := s.find(key)

		if levelFound != -1 {
			victim = succs[levelFound]
		}

		if !isMarked {
			// First time through
			if levelFound == -1 {
				// Nothing found
				return nil, false
			}

			if !victim.fullyLinked.Load() {
				// Victim not fully inserted
				return nil, false
			}

			if victim.marked.Load() {
				// Victim already being removed
				return nil, false
			}

			if victim.topLevel != levelFound {
				// Not fully linked when found
				return nil, false
			}

			topLevel = victim.topLevel
			victim.Lock()
			if victim.marked.Load() {
				// Another call beat us
				victim.Unlock()
				return nil, false
			}

			victim.marked.Store(true)
			isMarked = true
		}

		// Victim found, lock predecessors
		highestLocked := -1
		level := 0
		valid := true
		prevKey := key

		for valid && (level <= topLevel) {
			pred := preds[level]
			if pred.key < prevKey {
				pred.Lock()
				prevKey = pred.key
			}

			highestLocked = level
			successor := pred.next[level].Load() == victim
			valid = !pred.marked.Load() && successor
			level++
		}

		if !valid {
			// Unlock
			level = highestLocked
			for level >= 0 {
				preds[level].Unlock()
				level--
			}

			// Predecessor changed, try again
			continue
		}

		// Begin removal - all nodes are locked and valid
		// Unlink
		level = topLevel
		for level >= 0 {
			preds[level].next[level].Store(victim.next[level].Load())
			level--
		}

		// Unlock
		victim.Unlock()
		level = highestLocked
		for level >= 0 {
			preds[level].Unlock()
			level--
		}

		s.totalOps.Add(1)
		return victim, true
	}
}

func (s SkipList[K, V]) Query(ctx context.Context, start K, end K) (results []Pair[K, V], err error) {
	// Repeatedly make queries
	for {
		// Use a counter to check that a write has not done
		// before query has finished
		oldOps := s.totalOps.Load()
		res := s.query(start, end)
		if oldOps == s.totalOps.Load() {
			return res, nil
		}

		// If deadline reached, then preemptively return
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// Implementation of the Query method
func (s SkipList[K, V]) query(start K, end K) (results []Pair[K, V]) {
	// Initialize vars for searching the list.
	pred := s.head
	level := pred.topLevel

	// Initialize return values
	results = make([]Pair[K, V], 0)

	// Find successor at each level.
	curr := pred
	for level > 0 {
		// Initialize current node.
		curr = pred.next[level].Load()

		// Look through this level of the list until we go past key.
		for start > curr.key {
			pred = curr
			curr = pred.next[level].Load()
		}

		level--
	}

	// When at last level, add everything until reach end
	for {
		if curr.key > end {
			break
		} else {
			results = append(results, Pair[K, V]{curr.key, curr.value})
			curr = curr.next[level].Load()
		}
	}

	return results
}
