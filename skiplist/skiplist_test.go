package skiplist

import (
	"errors"
	"testing"
)

/*
* Upsert
 */
func TestInsertSuccess(t *testing.T) {
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
	check := func(key int, val int, exists bool) (int, error) {
		if exists {
			return 0, errors.New("In list already")
		} else {
			return 6, nil
		}
	}

	list := New[int, int](0, 10, 3)
	list.Upsert(1, check)
	ok, err := list.Upsert(1, check)

	if err != nil {
		t.Fatalf("expected no errors, got %s", err.Error())
	}

	if ok {
		t.Fatalf("expected false. got %t", ok)
	}
}
