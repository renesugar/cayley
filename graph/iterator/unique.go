package iterator

import (
	"context"

	"github.com/cayleygraph/cayley/graph/values"
)

var _ Iterator = &Unique{}

// Unique iterator removes duplicate values from it's subiterator.
type Unique struct {
	uid      uint64
	subIt    Iterator
	result   values.Ref
	runstats IteratorStats
	err      error
	seen     map[interface{}]bool
}

func NewUnique(subIt Iterator) *Unique {
	return &Unique{
		uid:   NextUID(),
		subIt: subIt,
		seen:  make(map[interface{}]bool),
	}
}

func (it *Unique) UID() uint64 {
	return it.uid
}

// Reset resets the internal iterators and the iterator itself.
func (it *Unique) Reset() {
	it.result = nil
	it.subIt.Reset()
	it.seen = make(map[interface{}]bool)
}

func (it *Unique) TagResults(dst map[string]values.Ref) {
	if it.subIt != nil {
		it.subIt.TagResults(dst)
	}
}

// SubIterators returns a slice of the sub iterators. The first iterator is the
// primary iterator, for which the complement is generated.
func (it *Unique) SubIterators() []Generic {
	return []Generic{it.subIt}
}

// Next advances the subiterator, continuing until it returns a value which it
// has not previously seen.
func (it *Unique) Next(ctx context.Context) bool {
	it.runstats.Next += 1

	for it.subIt.Next(ctx) {
		curr := it.subIt.Result()
		key := values.ToKey(curr)
		if ok := it.seen[key]; !ok {
			it.result = curr
			it.seen[key] = true
			return true
		}
	}
	it.err = it.subIt.Err()
	return false
}

func (it *Unique) Err() error {
	return it.err
}

func (it *Unique) Result() values.Ref {
	return it.result
}

// Contains checks whether the passed value is part of the primary iterator,
// which is irrelevant for uniqueness.
func (it *Unique) Contains(ctx context.Context, val values.Ref) bool {
	it.runstats.Contains += 1
	return it.subIt.Contains(ctx, val)
}

// NextPath for unique always returns false. If we were to return multiple
// paths, we'd no longer be a unique result, so we have to choose only the first
// path that got us here. Unique is serious on this point.
func (it *Unique) NextPath(ctx context.Context) bool {
	return false
}

// Close closes the primary iterators.
func (it *Unique) Close() error {
	it.seen = nil
	return it.subIt.Close()
}

const uniquenessFactor = 2

func (it *Unique) Stats() IteratorStats {
	subStats := it.subIt.Stats()
	return IteratorStats{
		NextCost:     subStats.NextCost * uniquenessFactor,
		ContainsCost: subStats.ContainsCost,
		Size:         subStats.Size / uniquenessFactor,
		ExactSize:    false,
		Next:         it.runstats.Next,
		Contains:     it.runstats.Contains,
		ContainsNext: it.runstats.ContainsNext,
	}
}

func (it *Unique) Size() (int64, bool) {
	st := it.Stats()
	return st.Size, st.ExactSize
}

func (it *Unique) String() string {
	return "Unique"
}
