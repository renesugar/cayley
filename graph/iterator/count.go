package iterator

import (
	"context"

	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

var _ VIterator = &Count{}

// Count iterator returns one element with size of underlying iterator.
type Count struct {
	uid    uint64
	it     Generic
	done   bool
	result quad.Value
}

// NewCount creates a new iterator to count a number of results from a provided subiterator.
// qs may be nil - it's used to check if count Contains (is) a given value.
func NewCount(it Generic) *Count {
	return &Count{
		uid: NextUID(),
		it:  it,
	}
}

func (it *Count) UID() uint64 {
	return it.uid
}

// Reset resets the internal iterators and the iterator itself.
func (it *Count) Reset() {
	it.done = false
	it.result = nil
	it.it.Reset()
}

func (it *Count) TagResults(dst map[string]values.Value) {}

// SubIterators returns a slice of the sub iterators.
func (it *Count) SubIterators() []Generic {
	return []Generic{it.it}
}

// Next counts a number of results in underlying iterator.
func (it *Count) Next(ctx context.Context) bool {
	if it.done {
		return false
	}
	size, exact := it.it.Size()
	if !exact {
		for size = 0; it.it.Next(ctx); size++ {
			for ; it.it.NextPath(ctx); size++ {
			}
		}
	}
	it.result = quad.Int(size)
	it.done = true
	return true
}

func (it *Count) Err() error {
	return it.it.Err()
}

func (it *Count) Result() quad.Value {
	if it.result == nil {
		return nil
	}
	return it.result
}

func (it *Count) Contains(ctx context.Context, val quad.Value) bool {
	if !it.done {
		it.Next(ctx)
	}
	return val == it.result
}

func (it *Count) NextPath(ctx context.Context) bool {
	return false
}

func (it *Count) Close() error {
	return it.it.Close()
}

func (it *Count) Stats() IteratorStats {
	stats := IteratorStats{
		NextCost:  1,
		Size:      1,
		ExactSize: true,
	}
	if sub := it.it.Stats(); !sub.ExactSize {
		stats.NextCost = sub.NextCost * sub.Size
	}
	stats.ContainsCost = stats.NextCost
	return stats
}

func (it *Count) Size() (int64, bool) {
	return 1, true
}

func (it *Count) String() string { return "Count" }
