package iterator

import (
	"context"
	"math"

	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
)

// Recursive iterator takes a base iterator and a morphism to be applied recursively, for each result.
type Recursive struct {
	uid      uint64
	subIt    Iterator
	result   seenAt
	runstats IteratorStats
	err      error

	morphism      Morphism
	seen          map[interface{}]seenAt
	nextIt        Iterator
	depth         int
	maxDepth      int
	pathMap       map[interface{}][]map[string]values.Ref
	pathIndex     int
	containsValue values.Ref
	depthTags     []string
	depthCache    []values.Ref
	baseIt        FixedIterator
}

type seenAt struct {
	depth int
	val   values.Ref
}

var _ Iterator = &Recursive{}

var DefaultMaxRecursiveSteps = 50

func NewRecursive(it Iterator, morphism Morphism, maxDepth int) *Recursive {
	if maxDepth == 0 {
		maxDepth = DefaultMaxRecursiveSteps
	}

	return &Recursive{
		uid:   NextUID(),
		subIt: it,

		morphism:      morphism,
		seen:          make(map[interface{}]seenAt),
		nextIt:        &Null{},
		baseIt:        NewFixed(),
		pathMap:       make(map[interface{}][]map[string]values.Ref),
		containsValue: nil,
		maxDepth:      maxDepth,
	}
}

func (it *Recursive) UID() uint64 {
	return it.uid
}

func (it *Recursive) Reset() {
	it.result.val = nil
	it.result.depth = 0
	it.err = nil
	it.subIt.Reset()
	it.seen = make(map[interface{}]seenAt)
	it.pathMap = make(map[interface{}][]map[string]values.Ref)
	it.containsValue = nil
	it.pathIndex = 0
	it.nextIt = &Null{}
	it.baseIt = NewFixed()
	it.depth = 0
}

func (it *Recursive) AddDepthTag(s string) {
	it.depthTags = append(it.depthTags, s)
}

func (it *Recursive) TagResults(dst map[string]values.Ref) {
	for _, tag := range it.depthTags {
		dst[tag] = values.PreFetched(quad.Int(it.result.depth))
	}

	if it.containsValue != nil {
		paths := it.pathMap[values.ToKey(it.containsValue)]
		if len(paths) != 0 {
			for k, v := range paths[it.pathIndex] {
				dst[k] = v
			}
		}
	}
	if it.nextIt != nil {
		it.nextIt.TagResults(dst)
		delete(dst, "__base_recursive")
	}
}

func (it *Recursive) SubIterators() []Generic {
	return []Generic{it.subIt}
}

func (it *Recursive) Next(ctx context.Context) bool {
	it.pathIndex = 0
	if it.depth == 0 {
		for it.subIt.Next(ctx) {
			res := it.subIt.Result()
			it.depthCache = append(it.depthCache, it.subIt.Result())
			tags := make(map[string]values.Ref)
			it.subIt.TagResults(tags)
			key := values.ToKey(res)
			it.pathMap[key] = append(it.pathMap[key], tags)
			for it.subIt.NextPath(ctx) {
				tags := make(map[string]values.Ref)
				it.subIt.TagResults(tags)
				it.pathMap[key] = append(it.pathMap[key], tags)
			}
		}
	}

	for {
		if !it.nextIt.Next(ctx) {
			if it.maxDepth > 0 && it.depth >= it.maxDepth {
				return false
			} else if len(it.depthCache) == 0 {
				return false
			}
			it.depth++
			it.baseIt = NewFixed(it.depthCache...)
			it.depthCache = nil
			if it.nextIt != nil {
				it.nextIt.Close()
			}
			it.nextIt = it.morphism(Tag(it.baseIt, "__base_recursive"))
			continue
		}
		val := it.nextIt.Result()
		results := make(map[string]values.Ref)
		it.nextIt.TagResults(results)
		key := values.ToKey(val)
		if _, seen := it.seen[key]; !seen {
			it.seen[key] = seenAt{
				val:   results["__base_recursive"],
				depth: it.depth,
			}
			it.result.depth = it.depth
			it.result.val = val
			it.containsValue = it.getBaseValue(val)
			it.depthCache = append(it.depthCache, val)
			return true
		}
	}
}

func (it *Recursive) Err() error {
	return it.err
}

func (it *Recursive) Result() values.Ref {
	return it.result.val
}

func (it *Recursive) getBaseValue(val values.Ref) values.Ref {
	var at seenAt
	var ok bool
	if at, ok = it.seen[values.ToKey(val)]; !ok {
		panic("trying to getBaseValue of something unseen")
	}
	for at.depth != 1 {
		if at.depth == 0 {
			panic("seen chain is broken")
		}
		at = it.seen[values.ToKey(at.val)]
	}
	return at.val
}

func (it *Recursive) Contains(ctx context.Context, val values.Ref) bool {
	it.pathIndex = 0
	key := values.ToKey(val)
	if at, ok := it.seen[key]; ok {
		it.containsValue = it.getBaseValue(val)
		it.result.depth = at.depth
		it.result.val = val
		return true
	}
	for it.Next(ctx) {
		if values.ToKey(it.Result()) == key {
			return true
		}
	}
	return false
}

func (it *Recursive) NextPath(ctx context.Context) bool {
	if it.pathIndex+1 >= len(it.pathMap[values.ToKey(it.containsValue)]) {
		return false
	}
	it.pathIndex++
	return true
}

func (it *Recursive) Close() error {
	err := it.subIt.Close()
	if err != nil {
		return err
	}
	err = it.nextIt.Close()
	if err != nil {
		return err
	}
	it.seen = nil
	return it.err
}

func (it *Recursive) Size() (int64, bool) {
	return it.Stats().Size, false
}

func (it *Recursive) Stats() IteratorStats {
	base := NewFixed()
	base.Add(Int64Node(20))
	fanoutit := it.morphism(base)
	fanoutStats := fanoutit.Stats()
	subitStats := it.subIt.Stats()

	size := int64(math.Pow(float64(subitStats.Size*fanoutStats.Size), 5))
	return IteratorStats{
		NextCost:     subitStats.NextCost + fanoutStats.NextCost,
		ContainsCost: (subitStats.NextCost+fanoutStats.NextCost)*(size/10) + subitStats.ContainsCost,
		Size:         size,
		Next:         it.runstats.Next,
		Contains:     it.runstats.Contains,
		ContainsNext: it.runstats.ContainsNext,
	}
}

func (it *Recursive) String() string {
	return "Recursive"
}
