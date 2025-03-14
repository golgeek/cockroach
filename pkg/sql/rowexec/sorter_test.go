// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package rowexec

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/randgen"
	"github.com/cockroachdb/cockroach/pkg/sql/rowcontainer"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/distsqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

func TestSorter(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	v := [6]rowenc.EncDatum{}
	for i := range v {
		v[i] = randgen.IntEncDatum(i)
	}

	asc := encoding.Ascending
	desc := encoding.Descending

	testCases := []struct {
		name     string
		spec     execinfrapb.SorterSpec
		post     execinfrapb.PostProcessSpec
		types    []*types.T
		input    rowenc.EncDatumRows
		expected rowenc.EncDatumRows
	}{
		{
			name: "SortAll",
			// No specified input ordering and unspecified limit.
			spec: execinfrapb.SorterSpec{
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 0, Direction: asc},
						{ColIdx: 1, Direction: desc},
						{ColIdx: 2, Direction: asc},
					}),
			},
			types: types.ThreeIntCols,
			input: rowenc.EncDatumRows{
				{v[1], v[0], v[4]},
				{v[3], v[4], v[1]},
				{v[4], v[4], v[4]},
				{v[3], v[2], v[0]},
				{v[4], v[4], v[5]},
				{v[3], v[3], v[0]},
				{v[0], v[0], v[0]},
			},
			expected: rowenc.EncDatumRows{
				{v[0], v[0], v[0]},
				{v[1], v[0], v[4]},
				{v[3], v[4], v[1]},
				{v[3], v[3], v[0]},
				{v[3], v[2], v[0]},
				{v[4], v[4], v[4]},
				{v[4], v[4], v[5]},
			},
		}, {
			name: "SortLimit",
			// No specified input ordering but specified limit.
			spec: execinfrapb.SorterSpec{
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 0, Direction: asc},
						{ColIdx: 1, Direction: asc},
						{ColIdx: 2, Direction: asc},
					}),
				Limit: 4,
			},
			types: types.ThreeIntCols,
			input: rowenc.EncDatumRows{
				{v[3], v[3], v[0]},
				{v[3], v[4], v[1]},
				{v[1], v[0], v[4]},
				{v[0], v[0], v[0]},
				{v[4], v[4], v[4]},
				{v[4], v[4], v[5]},
				{v[3], v[2], v[0]},
			},
			expected: rowenc.EncDatumRows{
				{v[0], v[0], v[0]},
				{v[1], v[0], v[4]},
				{v[3], v[2], v[0]},
				{v[3], v[3], v[0]},
			},
		}, {
			name: "SortOffset",
			// No specified input ordering but specified offset and limit.
			spec: execinfrapb.SorterSpec{
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 0, Direction: asc},
						{ColIdx: 1, Direction: asc},
						{ColIdx: 2, Direction: asc},
					}),
				Limit: 4,
			},
			post:  execinfrapb.PostProcessSpec{Offset: 2},
			types: types.ThreeIntCols,
			input: rowenc.EncDatumRows{
				{v[3], v[3], v[0]},
				{v[3], v[4], v[1]},
				{v[1], v[0], v[4]},
				{v[0], v[0], v[0]},
				{v[4], v[4], v[4]},
				{v[4], v[4], v[5]},
				{v[3], v[2], v[0]},
			},
			expected: rowenc.EncDatumRows{
				{v[3], v[2], v[0]},
				{v[3], v[3], v[0]},
			},
		}, {
			name: "SortMatchOrderingNoLimit",
			// Specified match ordering length but no specified limit.
			spec: execinfrapb.SorterSpec{
				OrderingMatchLen: 2,
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 0, Direction: asc},
						{ColIdx: 1, Direction: asc},
						{ColIdx: 2, Direction: asc},
					}),
			},
			types: types.ThreeIntCols,
			input: rowenc.EncDatumRows{
				{v[0], v[1], v[2]},
				{v[0], v[1], v[0]},
				{v[1], v[0], v[5]},
				{v[1], v[1], v[5]},
				{v[1], v[1], v[4]},
				{v[3], v[4], v[3]},
				{v[3], v[4], v[2]},
				{v[3], v[5], v[1]},
				{v[4], v[4], v[5]},
				{v[4], v[4], v[4]},
			},
			expected: rowenc.EncDatumRows{
				{v[0], v[1], v[0]},
				{v[0], v[1], v[2]},
				{v[1], v[0], v[5]},
				{v[1], v[1], v[4]},
				{v[1], v[1], v[5]},
				{v[3], v[4], v[2]},
				{v[3], v[4], v[3]},
				{v[3], v[5], v[1]},
				{v[4], v[4], v[4]},
				{v[4], v[4], v[5]},
			},
		}, {
			name: "SortInputOrderingNoLimit",
			// Specified input ordering but no specified limit.
			spec: execinfrapb.SorterSpec{
				OrderingMatchLen: 2,
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 1, Direction: asc},
						{ColIdx: 2, Direction: asc},
						{ColIdx: 3, Direction: asc},
					}),
			},
			types: []*types.T{types.Int, types.Int, types.Int, types.Int},
			input: rowenc.EncDatumRows{
				{v[1], v[1], v[2], v[5]},
				{v[0], v[1], v[2], v[4]},
				{v[0], v[1], v[2], v[3]},
				{v[1], v[1], v[2], v[2]},
				{v[1], v[2], v[2], v[5]},
				{v[0], v[2], v[2], v[4]},
				{v[0], v[2], v[2], v[3]},
				{v[1], v[2], v[2], v[2]},
			},
			expected: rowenc.EncDatumRows{
				{v[1], v[1], v[2], v[2]},
				{v[0], v[1], v[2], v[3]},
				{v[0], v[1], v[2], v[4]},
				{v[1], v[1], v[2], v[5]},
				{v[1], v[2], v[2], v[2]},
				{v[0], v[2], v[2], v[3]},
				{v[0], v[2], v[2], v[4]},
				{v[1], v[2], v[2], v[5]},
			},
		}, {
			name: "SortInputOrderingAlreadySorted",
			spec: execinfrapb.SorterSpec{
				OrderingMatchLen: 2,
				OutputOrdering: execinfrapb.ConvertToSpecOrdering(
					colinfo.ColumnOrdering{
						{ColIdx: 1, Direction: asc},
						{ColIdx: 2, Direction: asc},
						{ColIdx: 3, Direction: asc},
					}),
			},
			types: []*types.T{types.Int, types.Int, types.Int, types.Int},
			input: rowenc.EncDatumRows{
				{v[1], v[1], v[2], v[2]},
				{v[0], v[1], v[2], v[3]},
				{v[0], v[1], v[2], v[4]},
				{v[1], v[1], v[2], v[5]},
				{v[1], v[2], v[2], v[2]},
				{v[0], v[2], v[2], v[3]},
				{v[0], v[2], v[2], v[4]},
				{v[1], v[2], v[2], v[5]},
			},
			expected: rowenc.EncDatumRows{
				{v[1], v[1], v[2], v[2]},
				{v[0], v[1], v[2], v[3]},
				{v[0], v[1], v[2], v[4]},
				{v[1], v[1], v[2], v[5]},
				{v[1], v[2], v[2], v[2]},
				{v[0], v[2], v[2], v[3]},
				{v[0], v[2], v[2], v[4]},
				{v[1], v[2], v[2], v[5]},
			},
		},
	}

	// Test with several memory limits:
	memLimits := []struct {
		bytes    int64
		expSpill bool
	}{
		// Use the default limit.
		{bytes: 0, expSpill: false},
		// Immediately switch to disk.
		{bytes: 1, expSpill: true},
		// A memory limit that should not be hit; the processor will
		// not use disk.
		{bytes: 1 << 20, expSpill: false},
	}

	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			for _, memLimit := range memLimits {
				name := fmt.Sprintf("MemLimit=%d", memLimit.bytes)
				t.Run(name, func(t *testing.T) {
					ctx := context.Background()
					st := cluster.MakeTestingClusterSettings()
					tempEngine, _, err := storage.NewTempEngine(ctx, base.DefaultTestTempStorageConfig(st), base.DefaultTestStoreSpec, nil /* statsCollector */)
					if err != nil {
						t.Fatal(err)
					}
					defer tempEngine.Close()

					evalCtx := eval.MakeTestingEvalContext(st)
					defer evalCtx.Stop(ctx)
					diskMonitor := execinfra.NewTestDiskMonitor(ctx, st)
					defer diskMonitor.Stop(ctx)
					flowCtx := execinfra.FlowCtx{
						EvalCtx: &evalCtx,
						Mon:     evalCtx.TestingMon,
						Cfg: &execinfra.ServerConfig{
							Settings:    cluster.MakeTestingClusterSettings(),
							TempStorage: tempEngine,
						},
						DiskMonitor: diskMonitor,
					}
					// Override the default memory limit. This will result in using
					// a memory row container which will hit this limit and fall
					// back to using a disk row container.
					flowCtx.Cfg.TestingKnobs.MemoryLimitBytes = memLimit.bytes

					in := distsqlutils.NewRowBuffer(c.types, c.input, distsqlutils.RowBufferArgs{})
					out := &distsqlutils.RowBuffer{}

					s, err := newSorter(context.Background(), &flowCtx, 0 /* processorID */, &c.spec, in, &c.post)
					if err != nil {
						t.Fatal(err)
					}
					s.Run(context.Background(), out)
					if !out.ProducerClosed() {
						t.Fatalf("output RowReceiver not closed")
					}

					var retRows rowenc.EncDatumRows
					for {
						row := out.NextNoMeta(t)
						if row == nil {
							break
						}
						retRows = append(retRows, row)
					}

					expStr := c.expected.String(c.types)
					retStr := retRows.String(c.types)
					if expStr != retStr {
						t.Errorf("invalid results; expected:\n   %s\ngot:\n   %s",
							expStr, retStr)
					}

					// Check whether the DiskBackedRowContainer spilled to disk.
					spilled := s.(rowsAccessor).getRows().Spilled()
					if memLimit.expSpill != spilled {
						t.Errorf("expected spill to disk=%t, found %t", memLimit.expSpill, spilled)
					}
					if spilled {
						if scp, ok := s.(*sortChunksProcessor); ok {
							if scp.rows.(*rowcontainer.DiskBackedRowContainer).UsingDisk() {
								t.Errorf("expected chunks processor to reset to use memory")
							}
						}
					}
				})
			}
		})
	}
}

// TestSortInvalidLimit verifies that a top-k sorter will never be created with
// an invalid k-parameter.
func TestSortInvalidLimit(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	spec := execinfrapb.SorterSpec{}
	spec.Limit = 0

	t.Run("KZeroNoTopK", func(t *testing.T) {
		ctx := context.Background()
		st := cluster.MakeTestingClusterSettings()
		evalCtx := eval.MakeTestingEvalContext(st)
		defer evalCtx.Stop(ctx)
		diskMonitor := execinfra.NewTestDiskMonitor(ctx, st)
		defer diskMonitor.Stop(ctx)
		flowCtx := execinfra.FlowCtx{
			EvalCtx: &evalCtx,
			Mon:     evalCtx.TestingMon,
			Cfg: &execinfra.ServerConfig{
				Settings: st,
			},
			DiskMonitor: diskMonitor,
		}

		post := execinfrapb.PostProcessSpec{}
		in := distsqlutils.NewRowBuffer([]*types.T{types.Int}, rowenc.EncDatumRows{}, distsqlutils.RowBufferArgs{})
		proc, err := newSorter(
			context.Background(), &flowCtx, 0, &spec, in, &post,
		)
		if err != nil {
			t.Fatal(err)
		}
		if _, sortAll := proc.(*sortAllProcessor); !sortAll {
			t.Fatalf("expected *sortAllProcessor, got %T", proc)
		}
		// Allow for the processor cleanup.
		proc.Run(ctx, &distsqlutils.RowBuffer{})
	})

	t.Run("KZero", func(t *testing.T) {
		var k uint64
		// All arguments apart from spec and post are not necessary.
		if _, err := newSortTopKProcessor(
			context.Background(), nil, 0, &spec, nil, nil, k,
		); !testutils.IsError(err, errSortTopKZeroK.Error()) {
			t.Fatalf("unexpected error %v, expected %v", err, errSortTopKZeroK)
		}
	})
}

var twoColOrdering = execinfrapb.ConvertToSpecOrdering(colinfo.ColumnOrdering{
	{ColIdx: 0, Direction: encoding.Ascending},
	{ColIdx: 1, Direction: encoding.Ascending},
})

// BenchmarkSortAll times how long it takes to sort an input of varying length.
func BenchmarkSortAll(b *testing.B) {
	defer log.Scope(b).Close(b)
	const numCols = 2

	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := eval.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)
	diskMonitor := execinfra.NewTestDiskMonitor(ctx, st)
	defer diskMonitor.Stop(ctx)
	flowCtx := execinfra.FlowCtx{
		EvalCtx: &evalCtx,
		Mon:     evalCtx.TestingMon,
		Cfg: &execinfra.ServerConfig{
			Settings: st,
		},
		DiskMonitor: diskMonitor,
	}

	rng, _ := randutil.NewTestRand()
	spec := execinfrapb.SorterSpec{OutputOrdering: twoColOrdering}
	post := execinfrapb.PostProcessSpec{}

	for _, numRows := range []int{1 << 4, 1 << 8, 1 << 12, 1 << 16} {
		b.Run(fmt.Sprintf("rows=%d", numRows), func(b *testing.B) {
			input := execinfra.NewRepeatableRowSource(types.TwoIntCols, randgen.MakeRandIntRows(rng, numRows, numCols))
			b.SetBytes(int64(numRows * numCols * 8))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				s, err := newSorter(
					context.Background(), &flowCtx, 0 /* processorID */, &spec, input, &post,
				)
				if err != nil {
					b.Fatal(err)
				}
				s.Run(context.Background(), &rowDisposer{})
				input.Reset()
			}
		})
	}
}

// BenchmarkSortLimit times how long it takes to sort a fixed size input with
// varying limits.
func BenchmarkSortLimit(b *testing.B) {
	defer log.Scope(b).Close(b)
	const numCols = 2

	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := eval.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)
	diskMonitor := execinfra.NewTestDiskMonitor(ctx, st)
	defer diskMonitor.Stop(ctx)
	flowCtx := execinfra.FlowCtx{
		EvalCtx: &evalCtx,
		Mon:     evalCtx.TestingMon,
		Cfg: &execinfra.ServerConfig{
			Settings: st,
		},
		DiskMonitor: diskMonitor,
	}

	rng, _ := randutil.NewTestRand()
	spec := execinfrapb.SorterSpec{OutputOrdering: twoColOrdering}

	const numRows = 1 << 16
	b.Run(fmt.Sprintf("rows=%d", numRows), func(b *testing.B) {
		input := execinfra.NewRepeatableRowSource(types.TwoIntCols, randgen.MakeRandIntRows(rng, numRows, numCols))
		for _, limit := range []int64{1 << 4, 1 << 8, 1 << 12, 1 << 16} {
			spec.Limit = limit
			b.Run(fmt.Sprintf("Limit=%d", limit), func(b *testing.B) {
				b.SetBytes(int64(numRows * numCols * 8))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					s, err := newSorter(
						context.Background(), &flowCtx, 0, /* processorID */
						&spec, input, &execinfrapb.PostProcessSpec{Limit: 0},
					)
					if err != nil {
						b.Fatal(err)
					}
					s.Run(context.Background(), &rowDisposer{})
					input.Reset()
				}
			})

		}
	})
}

// BenchmarkSortChunks times how long it takes to sort an input which is already
// sorted on a prefix.
func BenchmarkSortChunks(b *testing.B) {
	defer log.Scope(b).Close(b)
	const numCols = 2

	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := eval.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)
	diskMonitor := execinfra.NewTestDiskMonitor(ctx, st)
	defer diskMonitor.Stop(ctx)
	flowCtx := execinfra.FlowCtx{
		EvalCtx: &evalCtx,
		Mon:     evalCtx.TestingMon,
		Cfg: &execinfra.ServerConfig{
			Settings: st,
		},
		DiskMonitor: diskMonitor,
	}

	rng, _ := randutil.NewTestRand()
	spec := execinfrapb.SorterSpec{
		OutputOrdering:   twoColOrdering,
		OrderingMatchLen: 1,
	}
	post := execinfrapb.PostProcessSpec{}

	for _, numRows := range []int{1 << 4, 1 << 8, 1 << 12, 1 << 16} {
		for chunkSize := 1; chunkSize <= numRows; chunkSize *= 4 {
			b.Run(fmt.Sprintf("rows=%d,ChunkSize=%d", numRows, chunkSize), func(b *testing.B) {
				rows := randgen.MakeRandIntRows(rng, numRows, numCols)
				for i, row := range rows {
					row[0] = randgen.IntEncDatum(i / chunkSize)
				}
				input := execinfra.NewRepeatableRowSource(types.TwoIntCols, rows)
				b.SetBytes(int64(numRows * numCols * 8))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					s, err := newSorter(context.Background(), &flowCtx, 0 /* processorID */, &spec, input, &post)
					if err != nil {
						b.Fatal(err)
					}
					s.Run(context.Background(), &rowDisposer{})
					input.Reset()
				}
			})
		}
	}
}
