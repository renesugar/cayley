package giterator

import (
	"context"
	"github.com/cayleygraph/cayley/graph/iterator"
)

// A testing iterator that returns the given values for Next() and Err().
type testIterator struct {
	*iterator.Fixed

	NextVal bool
	ErrVal  error
}

func newTestIterator(next bool, err error) iterator.Iterator {
	return &testIterator{
		Fixed:   iterator.NewFixed(),
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
