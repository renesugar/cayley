package shape

import (
	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/quad"
	"regexp"
	"strings"
)

// ValShape represent a query tree shape that returns values.
type ValShape interface {
	// BuildIterator constructs an iterator tree from a given shapes and binds it to QuadStore.
	BuildIterator() iterator.VIterator
	// Optimize runs an optimization pass over a query shape.
	//
	// It returns a bool that indicates if shape was replaced and should always return a copy of shape in this case.
	// In case no optimizations were made, it returns the same unmodified shape.
	//
	// If Optimizer is specified, it will be used instead of default optimizations.
	Optimize(r Optimizer) (ValShape, bool)
}

// NullV represent an empty set. Mostly used as a safe alias for nil shape.
type NullV struct{}

func (NullV) BuildIterator() iterator.VIterator {
	return iterator.NewNullV()
}
func (s NullV) Optimize(r Optimizer) (ValShape, bool) {
	if r != nil {
		return r.OptimizeExpr(s)
	}
	return nil, true
}

// IsNullExpr safely checks if shape represents an empty set. It accounts for both Null and nil.
func IsNullExpr(s ValShape) bool {
	_, ok := s.(NullV)
	return s == nil || ok
}

var _ ValShape = Values{}

type Values []quad.Value

func (s *Values) Add(v ...quad.Value) {
	*s = append(*s, v...)
}

func (s Values) BuildIterator() iterator.VIterator {
	return iterator.NewValues(s...)
}

func (s Values) Optimize(r Optimizer) (ValShape, bool) {
	if len(s) == 0 {
		return nil, true
	}
	if r != nil {
		return r.OptimizeExpr(s)
	}
	return s, false
}

// Count returns a count of objects in source as a single value. It always returns exactly one value.
type Count struct {
	Values Shape
}

func (s Count) BuildIterator() iterator.VIterator {
	var it iterator.Iterator
	if IsNull(s.Values) {
		it = iterator.NewNull()
	} else {
		it = s.Values.BuildIterator()
	}
	return iterator.NewCount(it)
}

func (s Count) Optimize(r Optimizer) (ValShape, bool) {
	if IsNull(s.Values) {
		return Values{quad.Int(0)}, true
	}
	var opt bool
	s.Values, opt = s.Values.Optimize(r)
	if IsNull(s.Values) {
		return Values{quad.Int(0)}, true
	}
	if r != nil {
		ns, nopt := r.OptimizeExpr(s)
		return ns, opt || nopt
	}
	// TODO: ask QS to estimate size - if it exact, then we can use it
	return s, opt
}

// ValueFilter is an interface for iterator wrappers that can filter node values.
type ValueFilter interface {
	BuildIterator(it iterator.VIterator) iterator.VIterator
}

// Filter filters all values from the source using a list of operations.
type Filter struct {
	From    ValShape      // source that will be filtered
	Filters []ValueFilter // filters to apply
}

func (s Filter) BuildIterator() iterator.VIterator {
	if IsNullExpr(s.From) {
		return iterator.NewNullV()
	}
	it := s.From.BuildIterator()
	for _, f := range s.Filters {
		it = f.BuildIterator(it)
	}
	return it
}
func (s Filter) Optimize(r Optimizer) (ValShape, bool) {
	if IsNullExpr(s.From) {
		return nil, true
	}
	var opt bool
	s.From, opt = s.From.Optimize(r)
	if r != nil {
		ns, nopt := r.OptimizeExpr(s)
		return ns, opt || nopt
	}
	if IsNullExpr(s.From) {
		return nil, true
	} else if len(s.Filters) == 0 {
		return s.From, true
	}
	return s, opt
}

var _ ValueFilter = Comparison{}

// Comparison is a value filter that evaluates binary operation in reference to a fixed value.
type Comparison struct {
	Op  iterator.Operator
	Val quad.Value
}

func (f Comparison) BuildIterator(it iterator.VIterator) iterator.VIterator {
	return iterator.NewComparison(it, f.Op, f.Val)
}

var _ ValueFilter = Regexp{}

// Regexp filters values using regular expression.
//
// Since regexp patterns can not be optimized in most cases, Wildcard should be used if possible.
type Regexp struct {
	Re   *regexp.Regexp
	Refs bool // allow to match IRIs
}

func (f Regexp) BuildIterator(it iterator.VIterator) iterator.VIterator {
	if f.Refs {
		return iterator.NewRegexWithRefs(it, f.Re)
	}
	return iterator.NewRegex(it, f.Re)
}

var _ ValueFilter = Wildcard{}

// Wildcard is a filter for string patterns.
//
//   % - zero or more characters
//   ? - exactly one character
type Wildcard struct {
	Pattern string // allowed wildcards are: % and ?
}

// Regexp returns an analog regexp pattern in format accepted by Go stdlib (RE2).
func (f Wildcard) Regexp() string {
	const any = `%`
	// escape all meta-characters in pattern string
	pattern := regexp.QuoteMeta(f.Pattern)
	// if the pattern is anchored, add regexp analog for it
	if !strings.HasPrefix(pattern, any) {
		pattern = "^" + pattern
	} else {
		pattern = strings.TrimPrefix(pattern, any)
	}
	if !strings.HasSuffix(pattern, any) {
		pattern = pattern + "$"
	} else {
		pattern = strings.TrimSuffix(pattern, any)
	}
	// replace wildcards
	pattern = strings.NewReplacer(
		any, `.*`,
		`\?`, `.`,
	).Replace(pattern)
	return pattern
}

func (f Wildcard) BuildIterator(it iterator.VIterator) iterator.VIterator {
	if f.Pattern == "" {
		return iterator.NewNullV()
	} else if strings.Trim(f.Pattern, "%") == "" {
		return it
	}
	re, err := regexp.Compile(f.Regexp())
	if err != nil {
		return iterator.NewErrorV(err)
	}
	return iterator.NewRegexWithRefs(it, re)
}
