package giterator

import (
	"context"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
)

type GIterator interface {
	iterator.Generic

	// Returns the current result.
	Result() values.Value

	// Contains returns whether the value is within the set held by the iterator.
	Contains(ctx context.Context, v values.Value) bool
}
