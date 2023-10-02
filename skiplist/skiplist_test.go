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
func TestInsertSuccess(t *testing.T) {
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

func TestInsertFailure(t *testing.T) {
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
	ok, _ := list.Upsert(1, check)

	if ok {
		t.Fatalf("expected false. got %t", ok)
	}
}
