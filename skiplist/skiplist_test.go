package skiplist

import (
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
)

func checkFactory(i int) UpdateCheck[int, int] {
	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return i, nil
		}
	}
	return check
}

/*
 * Upsert
 */
func TestUpsertSuccess(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	_, err := list.Upsert(1, checkFactory(6))

	if err != nil {
		t.Fatalf("expected no errors, got %s", err.Error())
	}
}

func TestUpsertFailure(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	_, err := list.Upsert(1, checkFactory(6))
	ok, _ := list.Upsert(1, checkFactory(6))

	if ok || err != nil {
		t.Fatalf("expected false, nil. got %t, %s", ok, err.Error())
	}
}

func TestUpsertInserts(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Find(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, ok. got %d, %t", v, ok)
	}

	_, ok = list.Find(6)
	if ok {
		t.Fatalf("expected false. got %t", ok)
	}
}

func TestUpsertUpdates(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 100, nil
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)
	list.Upsert(1, check)

	v, ok := list.Find(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, ok. got %d, %t", v, ok)
	}

	list.Upsert(1, check)

	v, ok = list.Find(1)
	if !ok || v != 100 {
		t.Fatalf("expected 100, ok. got %d, %t", v, ok)
	}
}

func TestMultipleUpserts(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)

	_, err := list.Upsert(1, checkFactory(6))
	if err != nil {
		t.Fatalf("expected no error. got %v", err)
	}

	_, err = list.Upsert(2, checkFactory(6))
	if err != nil {
		t.Fatalf("expected true. got %v", err)
	}

	_, err = list.Upsert(3, checkFactory(6))
	if err != nil {
		t.Fatalf("expected true. got %v", err)
	}
}

/*
 * Remove
 */

func TestRemoveExists(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Remove(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, true. got %d, %t", v, ok)
	}
}

func TestRemoveRemoves(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Remove(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, true. got %d, %t", v, ok)
	}

	_, ok = list.Find(1)
	if ok {
		t.Fatalf("expected false. got %t", ok)
	}

	_, err := list.Upsert(1, checkFactory(6))
	if err != nil {
		t.Fatalf("expected true, nil. got %t, %s", ok, err.Error())
	}

}

func TestRemoveDoesNotExist(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Remove(2)
	if ok {
		t.Fatalf("expected _, false. got %d, %t", v, ok)
	}
}

func TestRemoveEmpty(t *testing.T) {
	list := New[int, int](0, 10, 3)

	v, ok := list.Remove(1)
	if ok {
		t.Fatalf("expected _, false. got %d, %t", v, ok)
	}
}

/*
 * Find
 */

func TestFindExists(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Find(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, true. got %d, %t", v, ok)
	}
}

func TestFindDoesNotExist(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Find(2)
	if ok {
		t.Fatalf("expected _, false. got %d, %t", v, ok)
	}
}

func TestFindEmpty(t *testing.T) {
	list := New[int, int](0, 10, 3)

	v, ok := list.Find(1)
	if ok {
		t.Fatalf("expected _, false. got %d, %t", v, ok)
	}
}

func TestFindDoesNotRemove(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	list := New[int, int](0, 10, 3)
	list.Upsert(1, checkFactory(6))

	v, ok := list.Find(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, true. got %d, %t", v, ok)
	}

	ok, _ = list.Upsert(1, checkFactory(6))
	if ok {
		t.Fatalf("expected false. got %t", ok)
	}
}

/*
 * Concurrency
 */

func TestConcurrentDistinctInserts(t *testing.T) {
	for j := 1; j < 100; j++ {
		list := New[int, int](0, 10, 3)
		iters := 5

		var wg sync.WaitGroup

		for i := 1; i <= iters; i++ {
			wg.Add(1)

			go func(k int) {
				defer wg.Done()

				ok, _ := list.Upsert(k, checkFactory(0))
				if ok {
					t.Errorf("expected false. got %t", ok)
				}
			}(i)
		}

		wg.Wait()
	}
}

func TestConcurrentRepeatedInserts(t *testing.T) {
	for j := 1; j < 100; j++ {
		list := New[int, int](0, 10, 3)
		iters := 5

		var wg sync.WaitGroup
		var okChan chan bool = make(chan bool)

		for i := 0; i < iters; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				ok, _ := list.Upsert(1, checkFactory(1))
				okChan <- ok
			}()
		}

		numSuccesses := 0
		for i := 0; i < iters; i++ {
			ok := <-okChan
			if ok {
				numSuccesses++
			}
		}

		wg.Wait()

		if numSuccesses != 0 {
			t.Fatalf("expected only zero successful inserts. got %d", numSuccesses)
		}
	}
}

func TestConcurrentRepeatedRemoves(t *testing.T) {
	for j := 1; j < 100; j++ {
		list := New[int, int](0, 10, 3)
		iters := 5

		ok, _ := list.Upsert(1, checkFactory(1))
		if ok {
			t.Fatalf("expected false. got %t", ok)
		}

		var wg sync.WaitGroup
		var okChan chan bool = make(chan bool)

		for i := 0; i < iters; i++ {
			wg.Add(1)

			go func() {
				defer wg.Done()
				_, ok := list.Remove(1)
				okChan <- ok
			}()
		}

		numSuccesses := 0
		for i := 0; i < iters; i++ {
			ok := <-okChan
			if ok {
				numSuccesses++
			}
		}

		wg.Wait()

		if numSuccesses != 1 {
			t.Fatalf("expected only one successful remove. got %d", numSuccesses)
		}
	}
}
