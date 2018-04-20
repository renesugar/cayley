// Copyright 2014 The Cayley Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iterator

import (
	"context"

	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

var _ VIterator = &ValueFilter{}

type ValueFilter struct {
	uid    uint64
	sub    VIterator
	filter ValueFilterFunc
	result quad.Value
	err    error
}

type ValueFilterFunc func(quad.Value) (bool, error)

func NewValueFilter(sub VIterator, filter ValueFilterFunc) *ValueFilter {
	return &ValueFilter{
		uid:    NextUID(),
		sub:    sub,
		filter: filter,
	}
}

func (it *ValueFilter) UID() uint64 {
	return it.uid
}

func (it *ValueFilter) doFilter(val quad.Value) bool {
	ok, err := it.filter(val)
	if err != nil {
		it.err = err
	}
	return ok
}

func (it *ValueFilter) Close() error {
	return it.sub.Close()
}

func (it *ValueFilter) Reset() {
	it.sub.Reset()
	it.err = nil
	it.result = nil
}

func (it *ValueFilter) Next(ctx context.Context) bool {
	for it.sub.Next(ctx) {
		val := it.sub.Result()
		if it.doFilter(val) {
			it.result = val
			return true
		}
	}
	it.err = it.sub.Err()
	return false
}

func (it *ValueFilter) Err() error {
	return it.err
}

func (it *ValueFilter) Result() quad.Value {
	return it.result
}

func (it *ValueFilter) NextPath(ctx context.Context) bool {
	for {
		hasNext := it.sub.NextPath(ctx)
		if !hasNext {
			it.err = it.sub.Err()
			return false
		}
		if it.doFilter(it.sub.Result()) {
			break
		}
	}
	it.result = it.sub.Result()
	return true
}

func (it *ValueFilter) SubIterators() []Generic {
	return []Generic{it.sub}
}

func (it *ValueFilter) Contains(ctx context.Context, val quad.Value) bool {
	if !it.doFilter(val) {
		return false
	}
	ok := it.sub.Contains(ctx, val)
	if !ok {
		it.err = it.sub.Err()
	}
	return ok
}

// If we failed the check, then the subiterator should not contribute to the result
// set. Otherwise, go ahead and tag it.
func (it *ValueFilter) TagResults(dst map[string]values.Ref) {
	it.sub.TagResults(dst)
}

func (it *ValueFilter) String() string {
	return "ValueFilter"
}

// We're only as expensive as our subiterator.
// Again, optimized value comparison iterators should do better.
func (it *ValueFilter) Stats() IteratorStats {
	return it.sub.Stats()
}

func (it *ValueFilter) Size() (int64, bool) {
	sz, _ := it.sub.Size()
	return sz / 2, false
}
