// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package kvstreamer

import (
	"context"
	"math"
	"strconv"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/concurrency/lock"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
	"github.com/stretchr/testify/require"
)

// TestStreamerMemoryAccounting performs sanity checking on the memory
// accounting done by the streamer.
func TestStreamerMemoryAccounting(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()

	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)

	s := srv.ApplicationLayer()

	codec := s.Codec()

	// Create a table (for which we know the encoding of valid keys) with a
	// single row.
	_, err := db.Exec("CREATE TABLE t (pk PRIMARY KEY, k) AS VALUES (0, 0)")
	require.NoError(t, err)

	// Obtain the TableID.
	r := db.QueryRow("SELECT 't'::regclass::oid")
	var tableID int
	require.NoError(t, r.Scan(&tableID))

	makeGetRequest := func(key int) kvpb.RequestUnion {
		var res kvpb.RequestUnion
		var get kvpb.GetRequest
		var union kvpb.RequestUnion_Get
		makeKey := func(pk int) []byte {
			// These numbers essentially make a key like '/t/primary/key/0'.
			return append(codec.IndexPrefix(uint32(tableID), 1), []byte{byte(136 + pk), 136}...)
		}
		get.Key = makeKey(key)
		union.Get = &get
		res.Value = &union
		return res
	}

	monitor := mon.NewMonitor(mon.Options{
		Name:     mon.MakeName("streamer"),
		Settings: cluster.MakeTestingClusterSettings(),
	})
	monitor.Start(ctx, nil /* pool */, mon.NewStandaloneBudget(math.MaxInt64))
	defer monitor.Stop(ctx)
	acc := monitor.MakeBoundAccount()
	defer acc.Close(ctx)

	getStreamer := func(reverse bool) *Streamer {
		require.Zero(t, acc.Used())
		rootTxn := kv.NewTxn(ctx, s.DB(), s.DistSQLPlanningNodeID())
		leafInputState, err := rootTxn.GetLeafTxnInputState(ctx, nil /* readsTree */)
		if err != nil {
			panic(err)
		}
		leafTxn := kv.NewLeafTxn(ctx, s.DB(), s.DistSQLPlanningNodeID(), leafInputState, nil /* header */)
		metrics := MakeMetrics()
		s := NewStreamer(
			s.DistSenderI().(*kvcoord.DistSender),
			&metrics,
			s.AppStopper(),
			leafTxn,
			func(ctx context.Context, ba *kvpb.BatchRequest) (*kvpb.BatchResponse, error) {
				res, err := leafTxn.Send(ctx, ba)
				if err != nil {
					return nil, err.GoError()
				}
				return res, nil
			},
			cluster.MakeTestingClusterSettings(),
			nil, /* sd */
			lock.WaitPolicy(0),
			math.MaxInt64,
			&acc,
			nil, /* kvPairsRead */
			lock.None,
			lock.Unreplicated,
			reverse,
		)
		s.Init(OutOfOrder, Hints{UniqueRequests: true}, 1 /* maxKeysPerRow */, nil /* diskBuffer */)
		return s
	}

	t.Run("get", func(t *testing.T) {
		acc.Clear(ctx)
		streamer := getStreamer(false /* reverse */)
		defer streamer.Close(ctx)

		// Get the row with pk=0.
		reqs := make([]kvpb.RequestUnion, 1)
		reqs[0] = makeGetRequest(0)
		require.NoError(t, streamer.Enqueue(ctx, reqs))
		results, err := streamer.GetResults(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))
		// 7 is the number of bytes in GetResponse.Value.RawBytes.
		var expectedMemToken = getResponseOverhead + 7
		require.Equal(t, expectedMemToken, results[0].memoryTok.toRelease)
		var expectedUsed = expectedMemToken + resultSize
		require.Equal(t, expectedUsed, acc.Used())
	})

	t.Run("scan", func(t *testing.T) {
		for _, reverse := range []bool{false, true} {
			t.Run("reverse="+strconv.FormatBool(reverse), func(t *testing.T) {
				acc.Clear(ctx)
				streamer := getStreamer(reverse)
				defer streamer.Close(ctx)

				// Scan the row with pk=0.
				reqs := make([]kvpb.RequestUnion, 1)
				reqs[0] = makeScanRequest(codec, uint32(tableID), 0, 1, reverse)
				require.NoError(t, streamer.Enqueue(ctx, reqs))
				results, err := streamer.GetResults(ctx)
				require.NoError(t, err)
				require.Equal(t, 1, len(results))
				// 29 is usually the number of bytes in
				// ScanResponse.BatchResponse[0]. We choose to hard-code this number
				// rather than consult NumBytes field directly as an additional
				// sanity-check. We also adjust the estimate to account for possible
				// tenant prefix.
				expectedMemToken := scanResponseOverhead + 29 + int64(len(codec.TenantPrefix()))
				var numBytes int64
				if reverse {
					numBytes = results[0].ScanResp.(*kvpb.ReverseScanResponse).NumBytes
				} else {
					numBytes = results[0].ScanResp.(*kvpb.ScanResponse).NumBytes
				}
				if numBytes == 33+int64(len(codec.TenantPrefix())) {
					// For some reason, sometimes it's not 29, but 33, and we do
					// allow for this.
					expectedMemToken += 4
				}
				require.Equal(t, expectedMemToken, results[0].memoryTok.toRelease)
				expectedUsed := expectedMemToken + resultSize
				// This is streamer.numRangesPerScanRequestAccountedFor.
				expectedUsed += 4
				require.Equal(t, expectedUsed, acc.Used())
			})
		}
	})
}

func makeScanRequest(
	codec keys.SQLCodec, tableID uint32, start, end int, reverse bool,
) kvpb.RequestUnion {
	var res kvpb.RequestUnion
	makeKey := func(pk int) []byte {
		// These numbers essentially make a key like '/t/primary/pk'.
		return append(codec.IndexPrefix(tableID, 1), byte(136+pk))
	}
	if reverse {
		var scan kvpb.ReverseScanRequest
		scan.Key = makeKey(start)
		scan.EndKey = makeKey(end)
		scan.ScanFormat = kvpb.BATCH_RESPONSE
		res.Value = &kvpb.RequestUnion_ReverseScan{ReverseScan: &scan}
	} else {
		var scan kvpb.ScanRequest
		scan.Key = makeKey(start)
		scan.EndKey = makeKey(end)
		scan.ScanFormat = kvpb.BATCH_RESPONSE
		res.Value = &kvpb.RequestUnion_Scan{Scan: &scan}
	}
	return res
}
