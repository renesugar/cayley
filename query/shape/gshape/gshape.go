package gshape

import (
	"regexp"
	"strings"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/graph/iterator/giterator"
	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	. "github.com/cayleygraph/cayley/query/shape"
)

// ValueFilter is an interface for iterator wrappers that can filter node values.
type ValueFilter interface {
	BuildIterator(it iterator.Iterator) iterator.Iterator
}

// Filter filters all values from the source using a list of operations.
type Filter struct {
	From    Shape         // source that will be filtered
	Filters []ValueFilter // filters to apply
}

func (s Filter) BuildIterator() iterator.Iterator {
	if IsNull(s.From) {
		return iterator.NewNull()
	}
	it := s.From.BuildIterator()
	for _, f := range s.Filters {
		it = f.BuildIterator(it)
	}
	return it
}
func (s Filter) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.From) {
		return nil, true
	}
	var opt bool
	s.From, opt = s.From.Optimize(r)
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	if IsNull(s.From) {
		return nil, true
	} else if len(s.Filters) == 0 {
		return s.From, true
	}
	return s, opt
}

var _ ValueFilter = Comparison{}

// Comparison is a value filter that evaluates binary operation in reference to a fixed value.
type Comparison struct {
	Op  giterator.Operator
	Val quad.Value
}

func (f Comparison) BuildIterator(it iterator.Iterator) iterator.Iterator {
	return giterator.NewComparison(it, f.Op, f.Val, qs)
}

var _ ValueFilter = Regexp{}

// Regexp filters values using regular expression.
//
// Since regexp patterns can not be optimized in most cases, Wildcard should be used if possible.
type Regexp struct {
	Re   *regexp.Regexp
	Refs bool // allow to match IRIs
}

func (f Regexp) BuildIterator(it iterator.Iterator) iterator.Iterator {
	if f.Refs {
		return giterator.NewRegexWithRefs(it, f.Re, qs)
	}
	return giterator.NewRegex(it, f.Re, qs)
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

func (f Wildcard) BuildIterator(it iterator.Iterator) iterator.Iterator {
	if f.Pattern == "" {
		return iterator.NewNull()
	} else if strings.Trim(f.Pattern, "%") == "" {
		return it
	}
	re, err := regexp.Compile(f.Regexp())
	if err != nil {
		return iterator.NewError(err)
	}
	return giterator.NewRegexWithRefs(it, re, qs)
}

// Count returns a count of objects in source as a single value. It always returns exactly one value.
type Count struct {
	Values Shape
}

func (s Count) BuildIterator() iterator.Iterator {
	var it iterator.Iterator
	if IsNull(s.Values) {
		it = iterator.NewNull()
	} else {
		it = s.Values.BuildIterator()
	}
	return giterator.NewCount(it, qs)
}
func (s Count) Optimize(r Optimizer) (Shape, bool) {
	if IsNull(s.Values) {
		return Fixed{values.PreFetched(quad.Int(0))}, true
	}
	var opt bool
	s.Values, opt = s.Values.Optimize(r)
	if IsNull(s.Values) {
		return Fixed{values.PreFetched(quad.Int(0))}, true
	}
	if r != nil {
		ns, nopt := r.OptimizeShape(s)
		return ns, opt || nopt
	}
	// TODO: ask QS to estimate size - if it exact, then we can use it
	return s, opt
}
