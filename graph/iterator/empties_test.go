package iterator_test

import (
	"context"

	. "github.com/cayleygraph/cayley/graph/iterator"
)

// A testing iterator that returns the given values for Next() and Err().
type testIterator struct {
	*Fixed

	NextVal bool
	ErrVal  error
}

func newTestIterator(next bool, err error) Iterator {
	return &testIterator{
		Fixed:   NewFixed(),
		NextVal: next,
		ErrVal:  err,
	}
}

func (it *testIterator) Next(ctx context.Context) bool {
	return it.NextVal
}

func (it *testIterator) Err() error {
	return it.ErrVal
}

// A testing iterator that returns the given values for Next() and Err().
type testVIterator struct {
	*Values

	NextVal bool
	ErrVal  error
}

func newTestVIterator(next bool, err error) VIterator {
	return &testVIterator{
		Values:  NewValues(),
		NextVal: next,
		ErrVal:  err,
	}
}

func (it *testVIterator) Next(ctx context.Context) bool {
	return it.NextVal
}

func (it *testVIterator) Err() error {
	return it.ErrVal
}
