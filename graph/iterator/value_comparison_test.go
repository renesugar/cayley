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

package iterator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/iterator/giterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
	"github.com/stretchr/testify/require"
)

type stringQS []quad.Value

func (qs stringQS) valueAt(i int) quad.Value {
	return qs[i]
}

func (qs stringQS) ValueOf(s quad.Value) values.Ref {
	if s == nil {
		return nil
	}
	for i := range qs {
		if va := qs.valueAt(i); va != nil && s.String() == va.String() {
			return iterator.Int64Node(i)
		}
	}
	return nil
}

func (qs stringQS) NameOf(v values.Ref) quad.Value {
	switch v.(type) {
	case iterator.Int64Node:
		i := int(v.(iterator.Int64Node))
		if i < 0 || i >= len(qs) {
			return nil
		}
		return qs.valueAt(i)
	default:
		return nil
	}
}

var (
	simpleStore stringQS
	stringStore stringQS
	mixedStore  stringQS
)

func init() {
	for i := 0; i <= 5; i++ {
		simpleStore = append(simpleStore, quad.Int(i))
	}
	for _, s := range []string{
		"foo", "bar", "baz", "echo",
	} {
		stringStore = append(stringStore, quad.String(s))
	}
	mixedStore = append(mixedStore, simpleStore...)
	mixedStore = append(mixedStore, stringStore...)
}

func simpleFixedIterator() shape.Values {
	return shape.Values(simpleStore[:5])
}

func stringFixedIterator() shape.Values {
	return shape.Values(stringStore)
}

func mixedFixedIterator() shape.Values {
	return shape.Values(mixedStore)
}

var comparisonTests = []struct {
	name     string
	operand  quad.Value
	operator shape.CmpOperator
	expect   []quad.Value
	shape    func() shape.Values
}{
	{
		name:     "int64 less",
		operand:  quad.Int(3),
		operator: shape.CompareLT,
		expect:   []quad.Value{quad.Int(0), quad.Int(1), quad.Int(2)},
		shape:    simpleFixedIterator,
	},
	{
		name:     "empty int64 less",
		operand:  quad.Int(0),
		operator: shape.CompareLT,
		expect:   nil,
		shape:    simpleFixedIterator,
	},
	{
		name:     "int64 greater",
		operand:  quad.Int(2),
		operator: shape.CompareGT,
		expect:   []quad.Value{quad.Int(3), quad.Int(4)},
		shape:    simpleFixedIterator,
	},
	{
		name:     "int64 greater or equal",
		operand:  quad.Int(2),
		operator: shape.CompareGTE,
		expect:   []quad.Value{quad.Int(2), quad.Int(3), quad.Int(4)},
		shape:    simpleFixedIterator,
	},
	{
		name:     "int64 greater or equal (mixed)",
		operand:  quad.Int(2),
		operator: shape.CompareGTE,
		expect:   []quad.Value{quad.Int(2), quad.Int(3), quad.Int(4), quad.Int(5)},
		shape:    mixedFixedIterator,
	},
	{
		name:     "string less",
		operand:  quad.String("echo"),
		operator: shape.CompareLT,
		expect:   []quad.Value{quad.String("bar"), quad.String("baz")},
		shape:    stringFixedIterator,
	},
	{
		name:     "empty string less",
		operand:  quad.String(""),
		operator: shape.CompareLT,
		expect:   nil,
		shape:    stringFixedIterator,
	},
	{
		name:     "string greater",
		operand:  quad.String("echo"),
		operator: shape.CompareGT,
		expect:   []quad.Value{quad.String("foo")},
		shape:    stringFixedIterator,
	},
	{
		name:     "string greater or equal",
		operand:  quad.String("echo"),
		operator: shape.CompareGTE,
		expect:   []quad.Value{quad.String("foo"), quad.String("echo")},
		shape:    stringFixedIterator,
	},
}

func TestValueComparison(t *testing.T) {
	ctx := context.TODO()
	for _, test := range comparisonTests {
		t.Run(test.name, func(t *testing.T) {
			vc := shape.Compare(test.shape(), test.operator, test.operand).BuildIterator()

			var got []quad.Value
			for vc.Next(ctx) {
				got = append(got, vc.Result())
			}
			require.Equal(t, test.expect, got)
		})
	}
}

var vciContainsTests = []struct {
	name     string
	operator shape.CmpOperator
	check    quad.Value
	expect   bool
	qs       giterator.Namer
	val      quad.Value
	shape    func() shape.Values
}{
	{
		name:     "1 is less than 2",
		operator: shape.CompareGTE,
		check:    quad.Int(1),
		expect:   false,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		name:     "2 is greater than or equal to 2",
		operator: shape.CompareGTE,
		check:    quad.Int(2),
		expect:   true,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		name:     "3 is greater than or equal to 2",
		operator: shape.CompareGTE,
		check:    quad.Int(3),
		expect:   true,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		name:     "5 is absent from iterator",
		operator: shape.CompareGTE,
		check:    quad.Int(5),
		expect:   false,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		name:     "foo is greater than or equal to echo",
		operator: shape.CompareGTE,
		check:    quad.String("foo"),
		expect:   true,
		qs:       stringStore,
		val:      quad.String("echo"),
		shape:    stringFixedIterator,
	},
	{
		name:     "echo is greater than or equal to echo",
		operator: shape.CompareGTE,
		check:    quad.String("echo"),
		expect:   true,
		qs:       stringStore,
		val:      quad.String("echo"),
		shape:    stringFixedIterator,
	},
	{
		name:     "foo is missing from the iterator",
		operator: shape.CompareLTE,
		check:    quad.String("foo"),
		expect:   false,
		qs:       stringStore,
		val:      quad.String("echo"),
		shape:    stringFixedIterator,
	},
}

func TestVCIContains(t *testing.T) {
	ctx := context.TODO()
	for _, test := range vciContainsTests {
		t.Run(test.name, func(t *testing.T) {
			vc := shape.Compare(test.shape(), test.operator, test.val).BuildIterator()
			require.True(t, vc.Contains(ctx, test.check) == test.expect)
		})
	}
}

func TestValueFilterIteratorErr(t *testing.T) {
	ctx := context.TODO()
	wantErr := errors.New("unique")
	errIt := newTestVIterator(false, wantErr)

	vc := iterator.NewValueFilter(errIt, func(_ quad.Value) (bool, error) {
		return true, nil
	})
	require.False(t, vc.Next(ctx))
	require.Equal(t, wantErr, vc.Err())
}
