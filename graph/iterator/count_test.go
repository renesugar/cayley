package iterator

import (
	"context"
	"testing"

	"github.com/cayleygraph/cayley/graph/values"
	"github.com/cayleygraph/cayley/quad"
	"github.com/stretchr/testify/require"
)

func TestCount(t *testing.T) {
	ctx := context.TODO()
	fixed := NewFixed(
		values.PreFetched(quad.String("a")),
		values.PreFetched(quad.String("b")),
		values.PreFetched(quad.String("c")),
		values.PreFetched(quad.String("d")),
		values.PreFetched(quad.String("e")),
	)
	it := NewCount(fixed)
	require.True(t, it.Next(ctx))
	require.Equal(t, quad.Int(5), it.Result())
	require.False(t, it.Next(ctx))
	require.True(t, it.Contains(ctx, quad.Int(5)))
	require.False(t, it.Contains(ctx, quad.Int(3)))

	fixed.Reset()

	fixed2 := NewFixed(
		values.PreFetched(quad.String("b")),
		values.PreFetched(quad.String("d")),
	)
	it = NewCount(NewAnd(fixed, fixed2))
	require.True(t, it.Next(ctx))
	require.Equal(t, quad.Int(2), it.Result())
	require.False(t, it.Next(ctx))
	require.False(t, it.Contains(ctx, quad.Int(5)))
	require.True(t, it.Contains(ctx, quad.Int(2)))
}
