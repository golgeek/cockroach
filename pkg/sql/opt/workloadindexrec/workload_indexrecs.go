// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package workloadindexrec

import (
	"context"
	"regexp"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/idxtype"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlstats/persistedsqlstats/sqlstatsutil"
	"github.com/cockroachdb/errors"
)

// FindWorkloadRecs finds index recommendations for the whole workload after the
// timestamp ts within the space budget represented by budgetBytes.
func FindWorkloadRecs(
	ctx context.Context, evalCtx *eval.Context, ts *tree.DTimestampTZ,
) ([]WorkloadIndexRec, error) {
	cis, dis, err := collectIndexRecs(ctx, evalCtx, ts)
	if err != nil {
		return nil, err
	}

	trieMap := buildTrieForIndexRecs(cis)
	newCis, err := extractIndexCovering(trieMap)
	if err != nil {
		return nil, err
	}

	// Since we collect all the indexes represented by the leaf nodes, all the
	// indexes with "DROP INDEX" has been covered, so we can directly drop all of
	// them without duplicates.
	var disMap = make(map[tree.TableIndexName][]uint64)
	for _, indx := range dis {
		di := indx.index
		for _, index := range di.IndexList {
			disMap[*index] = append(disMap[*index], indx.fingerprintId)
		}
	}

	for index := range disMap {
		dropCmd := tree.DropIndex{
			IndexList: []*tree.TableIndexName{&index},
		}
		newCis = append(newCis, WorkloadIndexRec{
			Index:          dropCmd.String() + ";",
			FingerprintIds: disMap[index],
		})
	}

	return newCis, nil
}

// collectIndexRecs collects all the index recommendations stored in the
// system.statement_statistics with the time later than ts.
func collectIndexRecs(
	ctx context.Context, evalCtx *eval.Context, ts *tree.DTimestampTZ,
) ([]createIndex, []dropIndex, error) {
	query := `SELECT index_recommendations, fingerprint_id FROM system.statement_statistics
						 WHERE (statistics -> 'statistics' ->> 'lastExecAt')::TIMESTAMPTZ > $1
						 AND array_length(index_recommendations, 1) > 0;`
	indexRecs, err := evalCtx.Planner.QueryIteratorEx(ctx, "get-candidates-for-workload-indexrecs",
		sessiondata.NoSessionDataOverride, query, ts.Time)
	if err != nil {
		return nil, nil, err
	}

	var p parser.Parser
	var cis []createIndex
	var dis []dropIndex
	var ok bool

	// The index recommendation starts with "creation", "replacement" or
	// "alteration".
	var r = regexp.MustCompile(`\s*(creation|replacement|alteration)\s*:\s*(.*)`)

	for ok, err = indexRecs.Next(ctx); ; ok, err = indexRecs.Next(ctx) {
		if err != nil {
			err = errors.CombineErrors(err, indexRecs.Close())
			indexRecs = nil
			return cis, dis, err
		}

		if !ok {
			break
		}

		indexes := tree.MustBeDArray(indexRecs.Cur()[0])
		fingerprintId, e := sqlstatsutil.DatumToUint64(indexRecs.Cur()[1])
		if e != nil {
			return cis, dis, err
		}
		for _, index := range indexes.Array {
			indexStr, ok := index.(*tree.DString)
			if !ok {
				err = errors.CombineErrors(errors.Newf("%s is not a string!", index.String()), indexRecs.Close())
				indexRecs = nil
				return cis, dis, err
			}

			indexStrArr := r.FindStringSubmatch(string(*indexStr))
			if indexStrArr == nil {
				err = errors.CombineErrors(errors.Newf("%s is not a valid index recommendation!", string(*indexStr)), indexRecs.Close())
				indexRecs = nil
				return cis, dis, err
			}

			// Since Alter index recommendation only makes invisible indexes visible,
			// so we skip it for now.
			if indexStrArr[1] == "alteration" {
				continue
			}

			stmts, err := p.Parse(indexStrArr[2])
			if err != nil {
				err = errors.CombineErrors(errors.Newf("%s is not a valid index operation!", indexStrArr[2]), indexRecs.Close())
				indexRecs = nil
				return cis, dis, err
			}

			for _, stmt := range stmts {
				switch stmt := stmt.AST.(type) {
				case *tree.CreateIndex:
					// Ignore all the inverted, vector, partial, sharded, etc. indexes right now.
					if stmt.Type == idxtype.FORWARD && stmt.Predicate == nil && stmt.Sharded == nil {
						cis = append(cis, createIndex{fingerprintId: fingerprintId, index: *stmt})
					}
				case *tree.DropIndex:
					dis = append(dis, dropIndex{fingerprintId: fingerprintId, index: *stmt})
				}
			}
		}
	}

	return cis, dis, nil
}

// WorkloadIndexRec contains an index recommendation and the fingerprint ids
// that the index is recommended for.
type WorkloadIndexRec struct {
	Index          string
	FingerprintIds []uint64
}

// createIndex contains a recommended tree.CreateIndex and the fingerprint id
// that the index is recommended for.
type createIndex struct {
	fingerprintId uint64
	index         tree.CreateIndex
}

// dropIndex contains a recommended tree.DropIndex and the fingerprint id
// that the index is recommended for.
type dropIndex struct {
	fingerprintId uint64
	index         tree.DropIndex
}

// buildTrieForIndexRecs builds the relation among all the indexRecs by a trie tree.
func buildTrieForIndexRecs(cis []createIndex) map[tree.TableName]*indexTrie {
	trieMap := make(map[tree.TableName]*indexTrie)
	for _, idx := range cis {
		ci := idx.index
		if _, ok := trieMap[ci.Table]; !ok {
			trieMap[ci.Table] = NewTrie()
		}

		trieMap[ci.Table].Insert(ci.Columns, ci.Storing, idx.fingerprintId)
	}
	return trieMap
}

// extractIndexCovering pushes down the storing part of the internal nodes: find
// whether it is covered by some leaf nodes. If yes, discard it; Otherwise,
// assign it to the shallowest leaf node. Then extractIndexCovering collects all
// the indexes represented by the leaf node.
func extractIndexCovering(tm map[tree.TableName]*indexTrie) ([]WorkloadIndexRec, error) {
	for _, t := range tm {
		t.RemoveStorings()
	}
	for _, t := range tm {
		t.AssignStoring()
	}
	var wcis []WorkloadIndexRec
	for table, trie := range tm {
		indexedColsArray, storingColsArray := collectAllLeavesForTable(trie)
		// The length of indexedCols and storingCols must be equal
		if len(indexedColsArray) != len(storingColsArray) {
			return nil, errors.Newf("The length of indexedColsArray and storingColsArray after collecting leaves from table %s is not equal!", table)
		}
		for i, indexedCols := range indexedColsArray {
			cisIndexedCols := make([]tree.IndexElem, len(indexedCols.indexedColumns))
			for j, col := range indexedCols.indexedColumns {
				cisIndexedCols[j] = tree.IndexElem{
					Column:    col.column,
					Direction: col.direction,
				}
				// Recover the ASC to Default direction.
				if col.direction == tree.Ascending {
					cisIndexedCols[j].Direction = tree.DefaultDirection
				}
			}
			index := tree.CreateIndex{
				Table:   table,
				Columns: cisIndexedCols,
				Storing: storingColsArray[i],
			}
			wcis = append(wcis, WorkloadIndexRec{
				Index:          index.String() + ";",
				FingerprintIds: indexedCols.fingerprints,
			})
		}
	}
	return wcis, nil
}
