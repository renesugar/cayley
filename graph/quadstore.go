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

package graph

// Defines the QuadStore interface. Every backing store must implement at
// least this interface.
//
// Most of these are pretty straightforward. As long as we can surface this
// interface, the rest of the stack will "just work" and we can connect to any
// quad backing store we prefer.

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/cayleygraph/cayley/query/shape"
)

type BatchQuadStore interface {
	ValuesOf(ctx context.Context, vals []values.Ref) ([]quad.Value, error)
}

func ValuesOf(ctx context.Context, qs Resolver, vals []values.Ref) ([]quad.Value, error) {
	it := qs.ToValue(shape.Fixed(vals)).BuildIterator()
	defer it.Close()
	out := make([]quad.Value, 0, len(vals))
	for it.Next(ctx) {
		out = append(out, it.Result())
	}
	return out, it.Err()
}

func ValueOf(ctx context.Context, qs Resolver, v values.Ref) (quad.Value, error) {
	arr, err := ValuesOf(ctx, qs, []values.Ref{v})
	if err != nil {
		return nil, err
	} else if len(arr) == 0 || arr[0] == nil {
		return nil, nil
	}
	return arr[0], nil
}

func RefOf(ctx context.Context, qs QuadStore, v quad.Value) (values.Ref, error) {
	it := qs.ToRef(shape.Values{v}).BuildIterator()
	defer it.Close()
	if !it.Next(ctx) {
		return nil, it.Err()
	}
	return it.Result(), it.Err()
}

// TODO: require and implement Resolver

type Resolver interface {
	// Given a node ID, return the opaque token used by the QuadStore
	// to represent that id.
	ToValue(s shape.Shape) shape.ValShape
	// Given an opaque token, return the node that it represents.
	ToRef(s shape.ValShape) shape.Shape
}

type QuadIndexer interface {
	// Given an opaque token, returns the quad for that token from the store.
	Quad(values.Ref) quad.Quad

	// Given a direction and a token, creates an iterator of links which have
	// that node token in that directional field.
	QuadIterator(quad.Direction, values.Ref) shape.Shape

	// TODO: QuadDirection should be an optional interface on Ref

	// Convenience function for speed. Given a quad token and a direction
	// return the node token for that direction. Sometimes, a QuadStore
	// can do this without going all the way to the backing store, and
	// gives the QuadStore the opportunity to make this optimization.
	//
	// Iterators will call this. At worst, a valid implementation is
	//
	//  qs.ValueOf(qs.Quad(id).Get(dir))
	//
	QuadDirection(id values.Ref, d quad.Direction) values.Ref
}

type Stats struct {
	//Exact bool  `json:"exact,omitempty"`
	//Nodes int64 `json:"nodes,omitempty"`

	Links int64 `json:"links,omitempty"`
}

type QuadStore interface {
	Resolver
	QuadIndexer

	// The only way in is through building a transaction, which
	// is done by a replication strategy.
	ApplyDeltas(in []Delta, opts IgnoreOpts) error

	// Returns a query that enumerates all nodes in the graph.
	AllNodes() shape.Shape

	// Returns a query that enumerates all links in the graph.
	AllQuads() shape.Shape

	// Returns the number of quads currently stored.
	Stats() Stats

	// Close the quad store and clean up. (Flush to disk, cleanly
	// sever connections, etc)
	Close() error
}

type Options map[string]interface{}

var (
	typeInt = reflect.TypeOf(int(0))
)

func (d Options) IntKey(key string, def int) (int, error) {
	if val, ok := d[key]; ok {
		if reflect.TypeOf(val).ConvertibleTo(typeInt) {
			i := reflect.ValueOf(val).Convert(typeInt).Int()
			return int(i), nil
		}

		return def, fmt.Errorf("Invalid %s parameter type from config: %T", key, val)
	}
	return def, nil
}

func (d Options) StringKey(key string, def string) (string, error) {
	if val, ok := d[key]; ok {
		if v, ok := val.(string); ok {
			return v, nil
		}

		return def, fmt.Errorf("Invalid %s parameter type from config: %T", key, val)
	}

	return def, nil
}

func (d Options) BoolKey(key string, def bool) (bool, error) {
	if val, ok := d[key]; ok {
		if v, ok := val.(bool); ok {
			return v, nil
		}

		return def, fmt.Errorf("Invalid %s parameter type from config: %T", key, val)
	}

	return def, nil
}

var (
	ErrDatabaseExists = errors.New("quadstore: cannot init; database already exists")
	ErrNotInitialized = errors.New("quadstore: not initialized")
)

type BulkLoader interface {
	// BulkLoad loads Quads from a quad.Unmarshaler in bulk to the QuadStore.
	// It returns ErrCannotBulkLoad if bulk loading is not possible. For example if
	// you cannot load in bulk to a non-empty database, and the db is non-empty.
	BulkLoad(quad.Reader) error
}
