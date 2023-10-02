package skiplist

import (
	"errors"
	"log/slog"
	"os"
	"testing"
)

/*
 * Upsert
 */
func TestUpsertSuccess(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)
	ok, err := list.Upsert(1, check)

	if err != nil {
		t.Fatalf("expected no errors, got %s", err.Error())
	}

	if !ok {
		t.Fatalf("expected true. got %t", ok)
	}
}

func TestUpsertFailure(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)
	_, err := list.Upsert(1, check)
	ok, _ := list.Upsert(1, check)

	if ok || err != nil {
		t.Fatalf("expected false, nil. got %t, %s", ok, err.Error())
	}
}

func TestUpsertInserts(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
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

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)

	ok, err := list.Upsert(1, check)
	if !ok || err != nil {
		t.Fatalf("expected true. got %t", ok)
	}

	ok, err = list.Upsert(2, check)
	if !ok || err != nil {
		t.Fatalf("expected true. got %t", ok)
	}

	ok, err = list.Upsert(3, check)
	if !ok || err != nil {
		t.Fatalf("expected true. got %t", ok)
	}
}

/*
 * Remove
 */

func TestRemoveExists(t *testing.T) {
	// Setting log level.
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(h))

	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)
	list.Upsert(1, check)

	v, ok := list.Remove(1)
	if !ok || v != 6 {
		t.Fatalf("expected 6, true. got %d, %t", v, ok)
	}
}
