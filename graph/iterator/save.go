package iterator

import (
	"context"
	"fmt"

	"github.com/cayleygraph/cayley/graph/values"
)

var (
	_ Iterator = (*Save)(nil)
	_ Tagger   = (*Save)(nil)
)

func Tag(it Iterator, tag string) Iterator {
	if s, ok := it.(Tagger); ok {
		s.AddTags(tag)
		return s
	}
	return NewSave(it, tag)
}

func NewSave(on Iterator, tags ...string) *Save {
	s := &Save{uid: NextUID(), it: on}
	s.AddTags(tags...)
	return s
}

type Save struct {
	uid       uint64
	tags      []string
	fixedTags map[string]values.Ref
	it        Iterator
}

func (it *Save) String() string {
	return fmt.Sprintf("Save(%v, %v)", it.tags, it.fixedTags)
}

// Add a tag to the iterator.
func (it *Save) AddTags(tag ...string) {
	it.tags = append(it.tags, tag...)
}

func (it *Save) AddFixedTag(tag string, value values.Ref) {
	if it.fixedTags == nil {
		it.fixedTags = make(map[string]values.Ref)
	}
	it.fixedTags[tag] = value
}

// Tags returns the tags held in the tagger. The returned value must not be mutated.
func (it *Save) Tags() []string {
	return it.tags
}

// Fixed returns the fixed tags held in the tagger. The returned value must not be mutated.
func (it *Save) FixedTags() map[string]values.Ref {
	return it.fixedTags
}

func (it *Save) CopyFromTagger(st Tagger) {
	it.tags = append(it.tags, st.Tags()...)

	fixed := st.FixedTags()
	if len(fixed) == 0 {
		return
	}
	if it.fixedTags == nil {
		it.fixedTags = make(map[string]values.Ref, len(fixed))
	}
	for k, v := range fixed {
		it.fixedTags[k] = v
	}
}

func (it *Save) TagResults(dst map[string]values.Ref) {
	it.it.TagResults(dst)

	v := it.Result()
	for _, tag := range it.tags {
		dst[tag] = v
	}

	for tag, value := range it.fixedTags {
		dst[tag] = value
	}
}

func (it *Save) Result() values.Ref {
	return it.it.Result()
}

func (it *Save) Next(ctx context.Context) bool {
	return it.it.Next(ctx)
}

func (it *Save) NextPath(ctx context.Context) bool {
	return it.it.NextPath(ctx)
}

func (it *Save) Contains(ctx context.Context, v values.Ref) bool {
	return it.it.Contains(ctx, v)
}

func (it *Save) Err() error {
	return it.it.Err()
}

func (it *Save) Reset() {
	it.it.Reset()
}

func (it *Save) Stats() IteratorStats {
	return it.it.Stats()
}

func (it *Save) Size() (int64, bool) {
	return it.it.Size()
}

func (it *Save) SubIterators() []Generic {
	return []Generic{it.it}
}

func (it *Save) Close() error {
	return it.it.Close()
}

func (it *Save) UID() uint64 {
	return it.uid
}
