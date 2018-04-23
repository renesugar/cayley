package shape

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cayleygraph/cayley/graph/iterator"
	"github.com/cayleygraph/cayley/quad"
)

// ValueFilter is an interface for iterator wrappers that can filter node values.
type ValueFilter interface {
	FilterValue(quad.Value) (bool, error)
}

type ValueFilterBuilder interface {
	ValueFilter
	BuildValueFilter() ValueFilter
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
		if b, ok := f.(ValueFilterBuilder); ok {
			f = b.BuildValueFilter()
			if f == nil {
				continue
			}
		}
		it = iterator.NewValueFilter(it, f.FilterValue)
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
		ns, nopt := r.OptimizeValShape(s)
		return ns, opt || nopt
	}
	if IsNullExpr(s.From) {
		return nil, true
	} else if len(s.Filters) == 0 {
		return s.From, true
	}
	// TODO: build filters
	return s, opt
}

var _ ValueFilter = Comparison{}

type CmpOperator int

func (op CmpOperator) String() string {
	switch op {
	case CompareEQ:
		return "=="
	case CompareNEQ:
		return "!="
	case CompareLT:
		return "<"
	case CompareLTE:
		return "<="
	case CompareGT:
		return ">"
	case CompareGTE:
		return ">="
	default:
		return fmt.Sprintf("op(%d)", int(op))
	}
}

const (
	CompareLT CmpOperator = iota
	CompareLTE
	CompareGT
	CompareGTE
	CompareEQ
	CompareNEQ
)

// Comparison is a value filter that evaluates binary operation in reference to a fixed value.
type Comparison struct {
	Op  CmpOperator
	Val quad.Value
}

func (f Comparison) FilterValue(qval quad.Value) (bool, error) {
	val, op := f.Val, f.Op
	switch cVal := val.(type) {
	case quad.Int:
		if cVal2, ok := qval.(quad.Int); ok {
			return runIntOp(cVal2, op, cVal), nil
		}
		return false, nil
	case quad.Float:
		if cVal2, ok := qval.(quad.Float); ok {
			return runFloatOp(cVal2, op, cVal), nil
		}
		return false, nil
	case quad.String:
		if cVal2, ok := qval.(quad.String); ok {
			return runStrOp(string(cVal2), op, string(cVal)), nil
		}
		return false, nil
	case quad.BNode:
		if cVal2, ok := qval.(quad.BNode); ok {
			return runStrOp(string(cVal2), op, string(cVal)), nil
		}
		return false, nil
	case quad.IRI:
		if cVal2, ok := qval.(quad.IRI); ok {
			return runStrOp(string(cVal2), op, string(cVal)), nil
		}
		return false, nil
	case quad.Time:
		if cVal2, ok := qval.(quad.Time); ok {
			return runTimeOp(time.Time(cVal2), op, time.Time(cVal)), nil
		}
		return false, nil
	default:
		return runStrOp(quad.StringOf(qval), op, quad.StringOf(val)), nil
	}
}

func runIntOp(a quad.Int, op CmpOperator, b quad.Int) bool {
	switch op {
	case CompareEQ:
		return a == b
	case CompareNEQ:
		return a != b
	case CompareLT:
		return a < b
	case CompareLTE:
		return a <= b
	case CompareGT:
		return a > b
	case CompareGTE:
		return a >= b
	default:
		panic("Unknown operator type")
	}
}

func runFloatOp(a quad.Float, op CmpOperator, b quad.Float) bool {
	switch op {
	case CompareEQ:
		return a == b
	case CompareNEQ:
		return a != b
	case CompareLT:
		return a < b
	case CompareLTE:
		return a <= b
	case CompareGT:
		return a > b
	case CompareGTE:
		return a >= b
	default:
		panic("Unknown operator type")
	}
}

func runStrOp(a string, op CmpOperator, b string) bool {
	switch op {
	case CompareEQ:
		return a == b
	case CompareNEQ:
		return a != b
	case CompareLT:
		return a < b
	case CompareLTE:
		return a <= b
	case CompareGT:
		return a > b
	case CompareGTE:
		return a >= b
	default:
		panic("Unknown operator type")
	}
}

func runTimeOp(a time.Time, op CmpOperator, b time.Time) bool {
	switch op {
	case CompareEQ:
		return a.Equal(b)
	case CompareNEQ:
		return !a.Equal(b)
	case CompareLT:
		return a.Before(b)
	case CompareLTE:
		return !a.After(b)
	case CompareGT:
		return a.After(b)
	case CompareGTE:
		return !a.Before(b)
	default:
		panic("Unknown operator type")
	}
}

var _ ValueFilter = Regexp{}

// Regexp filters values using regular expression.
//
// Since regexp patterns can not be optimized in most cases, Wildcard should be used if possible.
type Regexp struct {
	Re   *regexp.Regexp
	Refs bool // allow to match IRIs
}

func (f Regexp) FilterValue(v quad.Value) (bool, error) {
	re, refs := f.Re, f.Refs
	switch v := v.(type) {
	case quad.String:
		return re.MatchString(string(v)), nil
	case quad.TypedString:
		return re.MatchString(string(v.Value)), nil
	default:
		if refs {
			switch v := v.(type) {
			case quad.BNode:
				return re.MatchString(string(v)), nil
			case quad.IRI:
				return re.MatchString(string(v)), nil
			}
		}
	}
	return false, nil
}

var (
	_ ValueFilter        = Wildcard{}
	_ ValueFilterBuilder = Wildcard{}
)

// Wildcard is a filter for string patterns.
//
//   % - zero or more characters
//   ? - exactly one character
type Wildcard struct {
	Pattern string // allowed wildcards are: % and ?
}

func (f Wildcard) BuildValueFilter() ValueFilter {
	if nf, err := f.buildValueFilter(); err == nil {
		return nf
	}
	return f
}
func (f Wildcard) buildValueFilter() (ValueFilter, error) {
	if f.Pattern == "" {
		return f, nil // FIXME
	} else if strings.Trim(f.Pattern, "%") == "" {
		return nil, nil
	}
	re, err := regexp.Compile(f.Regexp())
	if err != nil {
		return nil, err
	}
	return Regexp{Re: re, Refs: true}, nil
}

func (f Wildcard) FilterValue(v quad.Value) (bool, error) {
	// FIXME: maybe throw an error or print warning?
	nf, err := f.buildValueFilter()
	if err != nil {
		return false, err
	}
	return nf.FilterValue(v)
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
