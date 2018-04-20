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
	"reflect"
	"testing"

	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/graphmock"
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
)

var (
	simpleStore = &graphmock.Oldstore{Data: []string{"0", "1", "2", "3", "4", "5"}, Parse: true}
	stringStore = &graphmock.Oldstore{Data: []string{"foo", "bar", "baz", "echo"}, Parse: true}
	mixedStore  = &graphmock.Oldstore{Data: []string{"0", "1", "2", "3", "4", "5", "foo", "bar", "baz", "echo"}, Parse: true}
)

func simpleFixedIterator() shape.Values {
	var f shape.Values
	for i := 0; i < 5; i++ {
		f.Add(quad.Int(i))
	}
	return f
}

func stringFixedIterator() shape.Values {
	var f shape.Values
	for _, value := range stringStore.Data {
		f.Add(quad.String(value))
	}
	return f
}

func mixedFixedIterator() shape.Values {
	var f shape.Values
	for i := 0; i < len(mixedStore.Data); i++ {
		f.Add(quad.Int(i))
	}
	return f
}

var comparisonTests = []struct {
	message  string
	operand  quad.Value
	operator iterator.Operator
	expect   []quad.Value
	shape    func() shape.Values
}{
	{
		message:  "successful int64 less than comparison",
		operand:  quad.Int(3),
		operator: iterator.CompareLT,
		expect:   []quad.Value{quad.Int(0), quad.Int(1), quad.Int(2)},
		shape:    simpleFixedIterator,
	},
	{
		message:  "empty int64 less than comparison",
		operand:  quad.Int(0),
		operator: iterator.CompareLT,
		expect:   nil,
		shape:    simpleFixedIterator,
	},
	{
		message:  "successful int64 greater than comparison",
		operand:  quad.Int(2),
		operator: iterator.CompareGT,
		expect:   []quad.Value{quad.Int(3), quad.Int(4)},
		shape:    simpleFixedIterator,
	},
	{
		message:  "successful int64 greater than or equal comparison",
		operand:  quad.Int(2),
		operator: iterator.CompareGTE,
		expect:   []quad.Value{quad.Int(2), quad.Int(3), quad.Int(4)},
		shape:    simpleFixedIterator,
	},
	{
		message:  "successful int64 greater than or equal comparison (mixed)",
		operand:  quad.Int(2),
		operator: iterator.CompareGTE,
		expect:   []quad.Value{quad.Int(2), quad.Int(3), quad.Int(4), quad.Int(5)},
		shape:    mixedFixedIterator,
	},
	{
		message:  "successful string less than comparison",
		operand:  quad.String("echo"),
		operator: iterator.CompareLT,
		expect:   []quad.Value{quad.String("bar"), quad.String("baz")},
		shape:    stringFixedIterator,
	},
	{
		message:  "empty string less than comparison",
		operand:  quad.String(""),
		operator: iterator.CompareLT,
		expect:   nil,
		shape:    stringFixedIterator,
	},
	{
		message:  "successful string greater than comparison",
		operand:  quad.String("echo"),
		operator: iterator.CompareGT,
		expect:   []quad.Value{quad.String("foo")},
		shape:    stringFixedIterator,
	},
	{
		message:  "successful string greater than or equal comparison",
		operand:  quad.String("echo"),
		operator: iterator.CompareGTE,
		expect:   []quad.Value{quad.String("foo"), quad.String("echo")},
		shape:    stringFixedIterator,
	},
}

func TestValueComparison(t *testing.T) {
	ctx := context.TODO()
	for _, test := range comparisonTests {
		vc := iterator.NewComparison(test.shape().BuildIterator(), test.operator, test.operand)

		var got []quad.Value
		for vc.Next(ctx) {
			got = append(got, vc.Result())
		}
		if !reflect.DeepEqual(got, test.expect) {
			t.Errorf("Failed to show %s, got:%q expect:%q", test.message, got, test.expect)
		}
	}
}

var vciContainsTests = []struct {
	message  string
	operator iterator.Operator
	check    quad.Value
	expect   bool
	qs       graph.Namer
	val      quad.Value
	shape    func() shape.Values
}{
	{
		message:  "1 is less than 2",
		operator: iterator.CompareGTE,
		check:    quad.Int(1),
		expect:   false,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		message:  "2 is greater than or equal to 2",
		operator: iterator.CompareGTE,
		check:    quad.Int(2),
		expect:   true,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		message:  "3 is greater than or equal to 2",
		operator: iterator.CompareGTE,
		check:    quad.Int(3),
		expect:   true,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		message:  "5 is absent from iterator",
		operator: iterator.CompareGTE,
		check:    quad.Int(5),
		expect:   false,
		qs:       simpleStore,
		val:      quad.Int(2),
		shape:    simpleFixedIterator,
	},
	{
		message:  "foo is greater than or equal to echo",
		operator: iterator.CompareGTE,
		check:    quad.String("foo"),
		expect:   true,
		qs:       stringStore,
		val:      quad.String("echo"),
		shape:    stringFixedIterator,
	},
	{
		message:  "echo is greater than or equal to echo",
		operator: iterator.CompareGTE,
		check:    quad.String("echo"),
		expect:   true,
		qs:       stringStore,
		val:      quad.String("echo"),
		shape:    stringFixedIterator,
	},
	{
		message:  "foo is missing from the iterator",
		operator: iterator.CompareLTE,
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
		vc := iterator.NewComparison(test.shape().BuildIterator(), test.operator, test.val)
		if vc.Contains(ctx, test.check) != test.expect {
			t.Errorf("Failed to show %s", test.message)
		}
	}
}

var comparisonIteratorTests = []struct {
	message string
	qs      graph.Namer
	val     quad.Value
}{
	{
		message: "2 is absent from iterator",
		qs:      simpleStore,
		val:     quad.Int(2),
	},
	{
		message: "'missing' is absent from iterator",
		qs:      stringStore,
		val:     quad.String("missing"),
	},
}

func TestComparisonIteratorErr(t *testing.T) {
	ctx := context.TODO()
	wantErr := errors.New("unique")
	errIt := newTestVIterator(false, wantErr)

	for _, test := range comparisonIteratorTests {
		vc := iterator.NewComparison(errIt, iterator.CompareLT, test.val)

		if vc.Next(ctx) != false {
			t.Errorf("Comparison iterator did not pass through initial 'false': %s", test.message)
		}
		if vc.Err() != wantErr {
			t.Errorf("Comparison iterator did not pass through underlying Err: %s", test.message)
		}
	}
}
