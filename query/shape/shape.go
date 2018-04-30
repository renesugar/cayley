package shape

import (
	"reflect"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

type Morphism interface {
	// Apply is a curried function that can generates a new iterator based on some prior iterator.
	Apply(Shape) Shape
}

type MorphismFunc func(Shape) Shape

func (m MorphismFunc) Apply(s Shape) Shape {
	return m(s)
}

// Shape represent a query tree shape.
type Shape interface {
	// BuildIterator constructs an iterator tree from a given shapes and binds it to QuadStore.
	BuildIterator() iterator.Iterator
	// Optimize runs an optimization pass over a query shape.
	//
	// It returns a bool that indicates if shape was replaced and should always return a copy of shape in this case.
	// In case no optimizations were made, it returns the same unmodified shape.
	//
	// If Optimizer is specified, it will be used instead of default optimizations.
	Optimize(r Optimizer) (Shape, bool)
}

type Optimizer interface {
	OptimizeShape(s Shape) (Shape, bool)
	OptimizeValShape(s ValShape) (ValShape, bool)
}

// Composite shape can be simplified to a tree of more basic shapes.
type Composite interface {
	Simplify() Shape
}

// WalkFunc is used to visit all shapes in the tree.
// If false is returned, branch will not be traversed further.
type WalkFunc func(Shape) bool

var rtShape = reflect.TypeOf((*Shape)(nil)).Elem()

// Walk calls provided function for each shape in the tree.
func Walk(s Shape, fnc WalkFunc) {
	if s == nil {
		return
	}
	if !fnc(s) {
		return
	}
	walkReflect(reflect.ValueOf(s), fnc)
}

func walkReflect(rv reflect.Value, fnc WalkFunc) {
	rt := rv.Type()
	switch rv.Kind() {
	case reflect.Slice:
		if rt.Elem().ConvertibleTo(rtShape) {
			// all element are shapes - call function on each of them
			for i := 0; i < rv.Len(); i++ {
				Walk(rv.Index(i).Interface().(Shape), fnc)
			}
		} else {
			// elements are not shapes, but might contain them
			for i := 0; i < rv.Len(); i++ {
				walkReflect(rv.Index(i), fnc)
			}
		}
	case reflect.Map:
		keys := rv.MapKeys()
		if rt.Elem().ConvertibleTo(rtShape) {
			// all element are shapes - call function on each of them
			for _, k := range keys {
				Walk(rv.MapIndex(k).Interface().(Shape), fnc)
			}
		} else {
			// elements are not shapes, but might contain them
			for _, k := range keys {
				walkReflect(rv.MapIndex(k), fnc)
			}
		}
	case reflect.Struct:
		// visit all fields
		for i := 0; i < rt.NumField(); i++ {
			f := rt.Field(i)
			// if field is of shape type - call function on it
			// we skip anonymous fields because they were already visited as part of the parent
			if !f.Anonymous && f.Type.ConvertibleTo(rtShape) {
				Walk(rv.Field(i).Interface().(Shape), fnc)
				continue
			}
			// it might be a struct/map/slice field, so we need to go deeper
			walkReflect(rv.Field(i), fnc)
		}
	}
}

// InternalQuad is an internal representation of quad index in QuadStore.
type InternalQuad struct {
	Subject   values.Ref
	Predicate values.Ref
	Object    values.Ref
	Label     values.Ref
}

// Get returns a specified direction of the quad.
func (q InternalQuad) Get(d quad.Direction) values.Ref {
	switch d {
	case quad.Subject:
		return q.Subject
	case quad.Predicate:
		return q.Predicate
	case quad.Object:
		return q.Object
	case quad.Label:
		return q.Label
	default:
		return nil
	}
}

// Set assigns a specified direction of the quad to a given value.
func (q InternalQuad) Set(d quad.Direction, v values.Ref) {
	switch d {
	case quad.Subject:
		q.Subject = v
	case quad.Predicate:
		q.Predicate = v
	case quad.Object:
		q.Object = v
	case quad.Label:
		q.Label = v
	default:
		panic(d)
	}
}

// QuadIndexer is an optional interface for quad stores that keep an index of quad directions.
//
// It is used to optimize shapes based on stats from these indexes.
type QuadIndexer interface {
	// SizeOfIndex returns a size of a quad index with given constraints.
	SizeOfIndex(c map[quad.Direction]values.Ref) (int64, bool)
	// LookupQuadIndex finds a quad that matches a given constraint.
	// It returns false if quad was not found, or there are multiple quads matching constraint.
	LookupQuadIndex(c map[quad.Direction]values.Ref) (InternalQuad, bool)
}

// IsNull safely checks if shape represents an empty set. It accounts for both Null and nil.
func IsNull(s Shape) bool {
	_, ok := s.(Null)
	return s == nil || ok
}

// Null represent an empty set. Mostly used as a safe alias for nil shape.
type Null struct{}

func (Null) BuildIterator() iterator.Iterator {
	return iterator.NewNull()
}
func (s Null) Optimize(r Optimizer) (Shape, bool) {
	if r != nil {
		return r.OptimizeShape(s)
	}
	return nil, true
}

// Fixed is a static set of nodes. Defined only for a particular QuadStore.
type Fixed []values.Ref

func (s *Fixed) Add(v ...values.Ref) {
	*s = append(*s, v...)
}
func (s Fixed) BuildIterator() iterator.Iterator {
	it := iterator.NewFixed()
	for _, v := range s {
		if _, ok := v.(quad.Value); ok {
			panic("quad value in fixed iterator")
		}
		it.Add(v)
	}
	return it
}
func (s Fixed) Optimize(r Optimizer) (Shape, bool) {
	if len(s) == 0 {
		return nil, true
	}
	if r != nil {
		return r.OptimizeShape(s)
	}
	return s, false
}

// FixedTags adds a set of fixed tag values to query results. It does not affect query execution in any other way.
//
// Shape implementations should try to push these objects up the tree during optimization process.
type FixedTags struct {
	Tags map[string]values.Ref
	On   Shape
}

func (s FixedTags) BuildIterator() iterator.Iterator {
	if IsNull(s.On) {
		return iterator.NewNull()
	}
	it := s.On.BuildIterator()
	sv := iterator.NewSave(it)
	for k, v := range s.Tags {
		sv.AddFixedTag(k, v)
	}
	return sv
}
func (s FixedTags) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.On) {
		return nil, true
	}
	var opt bool
	s.On, opt = s.On.Optimize(r)
	if len(s.Tags) == 0 {
		return s.On, true
	} else if s2, ok := s.On.(FixedTags); ok {
		tags := make(map[string]values.Ref, len(s.Tags)+len(s2.Tags))
		for k, v := range s.Tags {
			tags[k] = v
		}
		for k, v := range s2.Tags {
			tags[k] = v
		}
		s, opt = FixedTags{On: s2.On, Tags: tags}, true
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	return s, opt
}

var MaterializeThreshold = 100 // TODO: tune

// Materialize loads results of sub-query into memory during execution to speedup iteration.
type Materialize struct {
	Size   int // approximate size; zero means undefined
	Values Shape
}

func (s Materialize) BuildIterator() iterator.Iterator {
	if IsNull(s.Values) {
		return iterator.NewNull()
	}
	it := s.Values.BuildIterator()
	return iterator.NewMaterializeWithSize(it, int64(s.Size))
}
func (s Materialize) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.Values) {
		return nil, true
	}
	var opt bool
	s.Values, opt = s.Values.Optimize(r)
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	return s, opt
}

func ClearFixedTags(arr []Shape) ([]Shape, map[string]values.Ref) {
	var tags map[string]values.Ref
	for i := 0; i < len(arr); i++ {
		if ft, ok := arr[i].(FixedTags); ok {
			if tags == nil {
				tags = make(map[string]values.Ref)
				na := make([]Shape, len(arr))
				copy(na, arr)
				arr = na
			}
			arr[i] = ft.On
			for k, v := range ft.Tags {
				tags[k] = v
			}
		}
	}
	return arr, tags
}

// Union joins results of multiple queries together. It does not make results unique.
type Union []Shape

func (s Union) BuildIterator() iterator.Iterator {
	if len(s) == 0 {
		return iterator.NewNull()
	}
	sub := make([]iterator.Iterator, 0, len(s))
	for _, c := range s {
		sub = append(sub, c.BuildIterator())
	}
	if len(sub) == 1 {
		return sub[0]
	}
	return iterator.NewOr(sub...)
}
func (s Union) Optimize(r Optimizer) (Shape, bool) {
	var opt bool
	realloc := func() {
		if !opt {
			arr := make(Union, len(s))
			copy(arr, s)
			s = arr
		}
	}
	// optimize subiterators
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == nil {
			continue
		}
		v, ok := c.Optimize(r)
		if !ok {
			continue
		}
		realloc()
		opt = true
		s[i] = v
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	if arr, ft := ClearFixedTags([]Shape(s)); ft != nil {
		ns, _ := FixedTags{On: Union(arr), Tags: ft}.Optimize(r)
		return ns, true
	}
	// second pass - remove Null
	for i := 0; i < len(s); i++ {
		c := s[i]
		if IsNull(c) {
			realloc()
			opt = true
			s = append(s[:i], s[i+1:]...)
		}
	}
	if len(s) == 0 {
		return nil, true
	} else if len(s) == 1 {
		return s[0], true
	}
	// TODO: join Fixed
	return s, opt
}

// Page provides a simple form of pagination. Can be used to skip or limit results.
type Page struct {
	From  Shape
	Skip  int64
	Limit int64 // zero means unlimited
}

func (s Page) BuildIterator() iterator.Iterator {
	if IsNull(s.From) {
		return iterator.NewNull()
	}
	it := s.From.BuildIterator()
	if s.Skip > 0 {
		it = iterator.NewSkip(it, s.Skip)
	}
	if s.Limit > 0 {
		it = iterator.NewLimit(it, s.Limit)
	}
	return it
}
func (s Page) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.From) {
		return nil, true
	}
	var opt bool
	s.From, opt = s.From.Optimize(r)
	if s.Skip <= 0 && s.Limit <= 0 {
		return s.From, true
	}
	if p, ok := s.From.(Page); ok {
		p2 := p.ApplyPage(s)
		if p2 == nil {
			return nil, true
		}
		s, opt = *p2, true
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	// TODO: check size
	return s, opt
}
func (s Page) ApplyPage(p Page) *Page {
	s.Skip += p.Skip
	if s.Limit > 0 {
		s.Limit -= p.Skip
		if s.Limit <= 0 {
			return nil
		}
		if p.Limit > 0 && s.Limit > p.Limit {
			s.Limit = p.Limit
		}
	} else {
		s.Limit = p.Limit
	}
	return &s
}

// Unique makes query results unique.
type Unique struct {
	From Shape
}

func (s Unique) BuildIterator() iterator.Iterator {
	if IsNull(s.From) {
		return iterator.NewNull()
	}
	it := s.From.BuildIterator()
	return iterator.NewUnique(it)
}
func (s Unique) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.From) {
		return nil, true
	}
	var opt bool
	s.From, opt = s.From.Optimize(r)
	if IsNull(s.From) {
		return nil, true
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	return s, opt
}

// Save tags a results of query with provided tags.
type Save struct {
	Tags []string
	From Shape
}

func (s Save) BuildIterator() iterator.Iterator {
	if IsNull(s.From) {
		return iterator.NewNull()
	}
	it := s.From.BuildIterator()
	if len(s.Tags) != 0 {
		return iterator.NewSave(it, s.Tags...)
	}
	return it
}
func (s Save) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.From) {
		return nil, true
	}
	var opt bool
	s.From, opt = s.From.Optimize(r)
	if len(s.Tags) == 0 {
		return s.From, true
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	return s, opt
}
