package giterator

import (
	"context"
	"fmt"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

type Namer interface {
	ValueOf(quad.Value) values.Ref
	NameOf(values.Ref) quad.Value
}

func NewValueToRef(qs Namer, it iterator.VIterator) iterator.Iterator {
	return &toRefIterator{uid: iterator.NextUID(), qs: qs, vals: it}
}

func NewRefToValue(qs Namer, it iterator.Iterator) iterator.VIterator {
	return &toValIterator{uid: iterator.NextUID(), qs: qs, refs: it}
}

type toValIterator struct {
	uid  uint64
	qs   Namer
	refs iterator.Iterator
	err  error
}

func (it *toValIterator) Close() error {
	if err := it.refs.Close(); err != nil {
		return err
	}
	return it.err
}

func (it *toValIterator) Result() quad.Value {
	ref := it.refs.Result()
	if ref == nil {
		return nil
	}
	// FIXME: return error
	val := it.qs.NameOf(ref)
	return val
}

func (it *toValIterator) Contains(ctx context.Context, v quad.Value) bool {
	if it.err != nil {
		return false
	}
	// FIXME: return error
	ref := it.qs.ValueOf(v)
	return it.refs.Contains(ctx, ref)
}

func (it *toValIterator) Err() error {
	if err := it.refs.Err(); err != nil {
		return err
	}
	return it.err
}

func (it *toValIterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	return it.refs.Next(ctx)
}

func (it *toValIterator) NextPath(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	return it.refs.NextPath(ctx)
}

func (it *toValIterator) Reset() {
	it.refs.Reset()
	it.err = nil
}

func (it *toValIterator) Size() (int64, bool) {
	sz, _ := it.refs.Size()
	return sz, false
}

func (it *toValIterator) Stats() iterator.IteratorStats {
	st := it.refs.Stats()
	st.ExactSize = false
	// TODO: estimate lookup cost
	return st
}

func (it *toValIterator) String() string {
	return fmt.Sprintf("ToVal(%v)", it.refs)
}

func (it *toValIterator) SubIterators() []iterator.Generic {
	return []iterator.Generic{it.refs}
}

func (it *toValIterator) TagResults(m map[string]values.Ref) {
	it.refs.TagResults(m)
}

func (it *toValIterator) UID() uint64 {
	return it.uid
}

type toRefIterator struct {
	uid  uint64
	qs   Namer
	vals iterator.VIterator
	err  error
}

func (it *toRefIterator) Close() error {
	if err := it.vals.Close(); err != nil {
		return err
	}
	return it.err
}

func (it *toRefIterator) Result() values.Ref {
	val := it.vals.Result()
	if val == nil {
		return nil
	}
	// FIXME: return error
	ref := it.qs.ValueOf(val)
	return ref
}

func (it *toRefIterator) Contains(ctx context.Context, v values.Ref) bool {
	if it.err != nil {
		return false
	}
	// FIXME: return error
	val := it.qs.NameOf(v)
	return it.vals.Contains(ctx, val)
}

func (it *toRefIterator) Err() error {
	if err := it.vals.Err(); err != nil {
		return err
	}
	return it.err
}

func (it *toRefIterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	return it.vals.Next(ctx)
}

func (it *toRefIterator) NextPath(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	return it.vals.NextPath(ctx)
}

func (it *toRefIterator) Reset() {
	it.vals.Reset()
	it.err = nil
}

func (it *toRefIterator) Size() (int64, bool) {
	sz, _ := it.vals.Size()
	return sz, false
}

func (it *toRefIterator) Stats() iterator.IteratorStats {
	st := it.vals.Stats()
	st.ExactSize = false
	// TODO: estimate lookup cost
	return st
}

func (it *toRefIterator) String() string {
	return fmt.Sprintf("ToRef(%v)", it.vals)
}

func (it *toRefIterator) SubIterators() []iterator.Generic {
	return []iterator.Generic{it.vals}
}

func (it *toRefIterator) TagResults(m map[string]values.Ref) {
	it.vals.TagResults(m)
}

func (it *toRefIterator) UID() uint64 {
	return it.uid
}
