// Package skiplist implements the skiplist interface
// as specified by the owlDB api.
package skiplist

import (
	"cmp"
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"sync/atomic"
)

// A struct representing a node in a skip list.
type node[K cmp.Ordered, V any] struct {
	sync.Mutex                               // Locking embedded in the nodes for concurrency.
	key         K                            // The key of this skiplist entry.
	value       V                            // The value fo this skiplist entry.
	topLevel    int                          // The top level at which this entry is inserted.
	marked      atomic.Bool                  // A marked bit saying whether this entry has been removed.
	fullyLinked atomic.Bool                  // A fully linked bit saying whether this entry has been fully added yet.
	next        []atomic.Pointer[node[K, V]] // A list of atomic pointers to the next node at a given level - size topLevel.
}

// A struct representing a skiplist.
type SkipList[K cmp.Ordered, V any] struct {
	head     *node[K, V]   // The head of the skiplist.
	totalOps *atomic.Int32 // An atomic counter updated when things are added or removed - used by query.
}

// A struct encapsulating the key, value returns from a query.
type Pair[K cmp.Ordered, V any] struct {
	Key   K // The key of this list entry.
	Value V // The value of this list entry.
}

// For strings, the min and max values
const (
	STRING_MIN    = ""
	STRING_MAX    = string(rune(256)) // Assuming no unicode, ASCII only
	DEFAULT_LEVEL = 5
)

// A function that determines whether to update a value given a key's current value
type UpdateCheck[K cmp.Ordered, V any] func(key K, currValue V, exists bool) (newValue V, err error)

// Creates an empty new skiplist object
func New[K cmp.Ordered, V any](minKey, maxKey K, max_level int) SkipList[K, V] {
	var head, tail node[K, V]

	// Construct head node.
	head.key = minKey
	head.topLevel = max_level - 1 // Because indexing at 0.
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
	tail.next = make([]atomic.Pointer[node[K, V]], max_level)

	// Link head to tail
	for i := 0; i < max_level; i++ {
		head.next[i].Store(&tail)
		tail.next[i].Store(nil)
	}
	// Set head to fully linked once its been linked to tail.
	head.fullyLinked.Swap(true)

	// Construct the skip list
	var ret SkipList[K, V]
	ret.head = &head
	ret.totalOps = &atomic.Int32{}
	ret.totalOps.Store(0)

	return ret
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
	newnode.next = make([]atomic.Pointer[node[K, V]], topLevel+1)

	return &newnode
}

// Helper method for Find, Upsert and Remove.
func (s SkipList[K, V]) find(key K) (int, []*node[K, V], []*node[K, V]) {
	slog.Debug("Called find", "key", key) // Call trace

	// Initialize vars for searching the list.
	foundLevel := -1
	pred := s.head
	level := s.head.topLevel

	// Initialize return values (+1 to account for 0)
	preds := make([]*node[K, V], s.head.topLevel+1)
	succs := make([]*node[K, V], s.head.topLevel+1)

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
	slog.Debug("Called Find", "key", key) // Call trace

	levelFound, _, succs := s.find(key)

	if levelFound == -1 {
		var zero V
		return zero, false
	}

	found := succs[levelFound]
	return found.value, found.fullyLinked.Load() && !found.marked.Load()
}

// A general function to update or insert a value in this skip list
// depending on the UpdateCheck function.
func (s SkipList[K, V]) Upsert(key K, check UpdateCheck[K, V]) (updated bool, err error) {
	slog.Debug("Called Upsert", "key", key) // Call trace

	// Pick random top level
	topLevel := 0
	for rand.Float32() < 0.5 && topLevel < s.head.topLevel {
		topLevel++
	}

	slog.Debug("Output level chosen", "level", topLevel, "key", key)

	// Keep trying insert
	for {
		// Check if already existing key
		levelFound, preds, succs := s.find(key)
		if levelFound != -1 {
			found := succs[levelFound]
			if !found.marked.Load() {
				// Node already exists (update case)

				// Need to wait for node to be fully linked if currently being added.
				for !found.fullyLinked.Load() {

				}

				// Only need to obtain found's lock for update
				found.Lock()

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

		// declared for zero value
		var def V
		newV, err := check(key, def, false)
		if err != nil {
			return false, err
		}

		valid := true
		level := 0

		prevKey := key
		used := make([]int, 0)

		// Lock all predecessors
		for ; valid && level <= topLevel; level++ {
			// Selective lock to not lock the same preds (reentrant)
			if preds[level].key < prevKey {
				preds[level].Lock()
				prevKey = preds[level].key
				used = append(used, level)
			}

			// Check pred/succ still valid
			unmarked := !preds[level].marked.Load() && !succs[level].marked.Load()
			connected := preds[level].next[level].Load() == succs[level]
			valid = unmarked && connected
		}

		if !valid {
			// Pred/succ changed. Unlock and retry
			// Selective unlock to only unlock the ones previous locked (reentrant)
			for _, i := range used {
				preds[i].Unlock()
			}

			continue
		}

		// Insert node
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

		// Selective unlock to only unlock the ones previous locked (reentrant)
		slog.Debug("Unlocking preds", "used", used)
		for _, i := range used {
			preds[i].Unlock()
		}

		s.totalOps.Add(1)
		return false, nil
	}
}

// Remove an element from this skiplist by its key.
// On success, return the value. Otherwise, return nil.
func (s SkipList[K, V]) Remove(key K) (value V, found bool) {
	slog.Debug("Called Remove", "key", key) // Call trace

	isMarked := false
	topLevel := -1
	var victim *node[K, V]
	var zero V

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
				return zero, false
			}

			if !victim.fullyLinked.Load() {
				// Victim not fully inserted
				return zero, false
			}

			if victim.marked.Load() {
				// Victim already being removed
				return zero, false
			}

			if victim.topLevel != levelFound {
				// Not fully linked when found
				return zero, false
			}

			topLevel = victim.topLevel
			victim.Lock()
			if victim.marked.Load() {
				// Another call beat us
				victim.Unlock()
				return zero, false
			}

			victim.marked.Store(true)
			isMarked = true
		}

		// Victim found, lock predecessors
		level := 0
		valid := true
		prevKey := key
		used := make([]int, 0)

		for valid && (level <= topLevel) {
			pred := preds[level]

			// Selective locking (reentrant)
			if pred.key < prevKey {
				pred.Lock()
				prevKey = pred.key
				used = append(used, level)
			}

			successor := pred.next[level].Load() == victim
			valid = !pred.marked.Load() && successor
			level++
		}

		if !valid {
			// Selective unlock to only unlock the ones previous locked (reentrant)
			for _, i := range used {
				preds[i].Unlock()
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

		// Selective unlock to only unlock the ones previous locked (reentrant)
		for _, i := range used {
			preds[i].Unlock()
		}

		s.totalOps.Add(1)
		return victim.value, true
	}
}

// Multiple-pass query; find an element between start and key values.
// Context can be passed in to stop the query operation if elapsed.
func (s SkipList[K, V]) Query(ctx context.Context, start K, end K) (results []Pair[K, V], err error) {
	slog.Debug("Called Query", "start", start, "end", end) // Call trace

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

// Single-pass query; find an element between start and end key values.
func (s SkipList[K, V]) query(start K, end K) (results []Pair[K, V]) {
	// Initialize return values
	results = make([]Pair[K, V], 0)

	// Do a linear search at the bottom of the skip list
	// Not possible to 'skip' in query as its possible to skip past elements that satisfy
	curr := s.head.next[0].Load()
	for {
		next := curr.next[0].Load()
		if curr.key < start {
			curr = curr.next[0].Load()
		} else if curr.key > end || next == nil {
			break
		} else {
			results = append(results, Pair[K, V]{curr.key, curr.value})
			curr = next
		}
	}

	return results
}
