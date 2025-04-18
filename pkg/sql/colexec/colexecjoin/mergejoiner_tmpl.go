// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

// {{/*
//go:build execgen_template

//
// This file is the execgen template for mergejoiner.eg.go. It's formatted in a
// special way, so it's both valid Go and a valid text/template input. This
// permits editing this file with editor support.
//
// */}}

package colexecjoin

import (
	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/col/coldataext"
	"github.com/cockroachdb/cockroach/pkg/col/typeconv"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecerror"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/duration"
	"github.com/cockroachdb/cockroach/pkg/util/json"
	"github.com/cockroachdb/errors"
)

// Workaround for bazel auto-generated code. goimports does not automatically
// pick up the right packages when run within the bazel sandbox.
var (
	_ = typeconv.DatumVecCanonicalTypeFamily
	_ apd.Context
	_ = coldataext.CompareDatum
	_ duration.Duration
	_ tree.AggType
	_ json.JSON
)

// {{/*
// Declarations to make the template compile properly.

// _GOTYPE is the template variable.
type _GOTYPE interface{}

// _CANONICAL_TYPE_FAMILY is the template variable.
const _CANONICAL_TYPE_FAMILY = types.UnknownFamily

// _TYPE_WIDTH is the template variable.
const _TYPE_WIDTH = 0

// _ASSIGN_EQ is the template equality function for assigning the first input
// to the result of the second input == the third input.
func _ASSIGN_EQ(_, _, _, _, _, _ interface{}) int {
	colexecerror.InternalError(errors.AssertionFailedf(""))
}

// _ASSIGN_CMP is the template equality function for assigning the first input
// to the result of comparing the second input to the third input which returns
// an integer. That integer is:
// - negative if left < right
// - zero if left == right
// - positive if left > right.
func _ASSIGN_CMP(_, _, _, _, _ interface{}) int {
	colexecerror.VectorizedInternalPanic("")
}

// _L_SEL_IND is the template type variable for the loop variable that
// is either curLIdx or lSel[curLIdx] depending on whether we're in a
// selection or not.
const _L_SEL_IND = 0

// _R_SEL_IND is the template type variable for the loop variable that
// is either curRIdx or rSel[curRIdx] depending on whether we're in a
// selection or not.
const _R_SEL_IND = 0

// _SEL_ARG is used in place of the string "$sel", since that isn't valid go
// code.
const _SEL_ARG = 0

// _JOIN_TYPE is used in place of the string "$.JoinType", since that isn't
// valid go code.
const _JOIN_TYPE = 0

// */}}

type mergeJoin_JOIN_TYPE_STRINGOp struct {
	*mergeJoinBase
}

var _ colexecop.Operator = &mergeJoin_JOIN_TYPE_STRINGOp{}

// {{/*
// This code snippet is the "meat" of the probing phase.
func _PROBE_SWITCH(_JOIN_TYPE joinTypeInfo, _SEL_PERMUTATION selPermutation) { // */}}
	// {{define "probeSwitch" -}}
	// {{$sel := $.SelPermutation}}
	lNulls := lVec.Nulls()
	rNulls := rVec.Nulls()
	switch lVec.CanonicalTypeFamily() {
	// {{range $overload := $.Global.Overloads}}
	case _CANONICAL_TYPE_FAMILY:
		switch colType.Width() {
		// {{range .WidthOverloads}}
		case _TYPE_WIDTH:
			lKeys := lVec.TemplateType()
			rKeys := rVec.TemplateType()
			var (
				lGroup, rGroup   group
				cmp              int
				match            bool
				lVal, rVal       _GOTYPE
				lSelIdx, rSelIdx int
			)

			for o.groups.nextGroupInCol(&lGroup, &rGroup) {
				curLIdx := lGroup.rowStartIdx
				curRIdx := rGroup.rowStartIdx
				curLEndIdx := lGroup.rowEndIdx
				curREndIdx := rGroup.rowEndIdx
				_LEFT_UNMATCHED_GROUP_SWITCH(_JOIN_TYPE)
				_RIGHT_UNMATCHED_GROUP_SWITCH(_JOIN_TYPE)
				// Expand or filter each group based on the current equality column.
				for curLIdx < curLEndIdx && curRIdx < curREndIdx {
					cmp = 0
					lNull := lNulls.NullAt(_L_SEL_IND)
					rNull := rNulls.NullAt(_R_SEL_IND)

					// {{if _JOIN_TYPE.IsSetOp}}
					// {{/*
					//     Set operations allow null equality, so we handle
					//     NULLs first.
					// */}}
					if lNull {
						// {{/* If we have NULL on the left, then it is smaller than the right value. */}}
						cmp--
					}
					if rNull {
						// {{/* If we have NULL on the right, then it is smaller than the left value. */}}
						cmp++
					}
					var nullMatch bool
					// {{/* Remove unused warning for some code paths of INTERSECT ALL join. */}}
					_ = nullMatch
					// If we have a NULL match, it will take precedence over
					// cmp value set above.
					nullMatch = lNull && rNull
					// {{else}}
					// {{/*
					//     Non-set operation joins do not allow null equality,
					//     so if either value is NULL, the tuples are not
					//     matches.
					// */}}
					curLIdxInc := 0
					if lNull {
						_NULL_FROM_LEFT_SWITCH(_JOIN_TYPE)
						curLIdxInc = 1
					}
					curRIdxInc := 0
					if rNull {
						_NULL_FROM_RIGHT_SWITCH(_JOIN_TYPE)
						curRIdxInc = 1
					}
					if lNull || rNull {
						curLIdx += curLIdxInc
						curRIdx += curRIdxInc
						continue
					}
					// {{end}}

					needToCompare := true
					// {{if _JOIN_TYPE.IsSetOp}}
					// For set operation joins we have already set 'cmp' to
					// correct value above if we have a null value at least
					// on one side.
					needToCompare = !lNull && !rNull
					// {{end}}
					if needToCompare {
						lSelIdx = _L_SEL_IND
						lVal = lKeys.Get(lSelIdx)
						rSelIdx = _R_SEL_IND
						rVal = rKeys.Get(rSelIdx)
						_ASSIGN_CMP(cmp, lVal, rVal, lKeys, rKeys)
					}

					if cmp == 0 {
						// Find the length of the groups on each side.
						lGroupLength, rGroupLength := 1, 1
						// If a group ends before the end of the probing batch,
						// then we know it is complete.
						lComplete := curLEndIdx < o.proberState.lLength
						rComplete := curREndIdx < o.proberState.rLength
						beginLIdx, beginRIdx := curLIdx, curRIdx
						curLIdx++
						curRIdx++

						// Find the length of the group on the left.
						for curLIdx < curLEndIdx {
							// {{if _JOIN_TYPE.IsSetOp}}
							if nullMatch {
								// {{/*
								//     We have a NULL match, so we only
								//     extend the left group if we have a
								//     NULL element.
								// */}}
								if !lNulls.NullAt(_L_SEL_IND) {
									lComplete = true
									break
								}
							} else
							// {{end}}
							{
								if lNulls.NullAt(_L_SEL_IND) {
									lComplete = true
									break
								}
								lSelIdx = _L_SEL_IND
								newLVal := lKeys.Get(lSelIdx)
								_ASSIGN_EQ(match, newLVal, lVal, _, lKeys, lKeys)
								if !match {
									lComplete = true
									break
								}
							}
							lGroupLength++
							curLIdx++
						}

						// Find the length of the group on the right.
						for curRIdx < curREndIdx {
							// {{if _JOIN_TYPE.IsSetOp}}
							if nullMatch {
								// {{/*
								//     We have a NULL match, so we only
								//     extend the right group if we have a
								//     NULL element.
								// */}}
								if !rNulls.NullAt(_R_SEL_IND) {
									rComplete = true
									break
								}
							} else
							// {{end}}
							{
								if rNulls.NullAt(_R_SEL_IND) {
									rComplete = true
									break
								}
								rSelIdx = _R_SEL_IND
								newRVal := rKeys.Get(rSelIdx)
								_ASSIGN_EQ(match, newRVal, rVal, _, rKeys, rKeys)
								if !match {
									rComplete = true
									break
								}
							}
							rGroupLength++
							curRIdx++
						}

						// Last equality column and either group is incomplete.
						if lastEqCol && (!lComplete || !rComplete) {
							// Store the state about the buffered group.
							o.startLeftBufferedGroup(lSel, beginLIdx, lGroupLength)
							o.bufferedGroup.leftGroupStartIdx = beginLIdx
							o.proberState.lIdx = lGroupLength + beginLIdx
							o.appendToRightBufferedGroup(rSel, beginRIdx, rGroupLength)
							o.proberState.rIdx = rGroupLength + beginRIdx
							o.groups.finishedCol()
							return
						}

						if !lastEqCol {
							o.groups.addGroupsToNextCol(beginLIdx, lGroupLength, beginRIdx, rGroupLength)
						} else {
							// {{if _JOIN_TYPE.IsLeftSemi}}
							leftSemiGroupLength := lGroupLength
							// {{if _JOIN_TYPE.IsSetOp}}
							// For INTERSECT ALL join we add a left semi group
							// of length min(lGroupLength, rGroupLength).
							if rGroupLength < lGroupLength {
								leftSemiGroupLength = rGroupLength
							}
							// {{end}}
							o.groups.addLeftSemiGroup(beginLIdx, leftSemiGroupLength)
							// {{else if _JOIN_TYPE.IsRightSemi}}
							o.groups.addRightSemiGroup(beginRIdx, rGroupLength)
							// {{else if _JOIN_TYPE.IsLeftAnti}}
							// {{if _JOIN_TYPE.IsSetOp}}
							// For EXCEPT ALL join we add (lGroupLength - rGroupLength) number
							// (if positive) of unmatched left groups.
							for leftUnmatchedTupleIdx := beginLIdx + rGroupLength; leftUnmatchedTupleIdx < beginLIdx+lGroupLength; leftUnmatchedTupleIdx++ {
								// Right index here doesn't matter.
								o.groups.addLeftUnmatchedGroup(leftUnmatchedTupleIdx, beginRIdx)
							}
							// {{else}}
							// With LEFT ANTI join, we are only interested in unmatched tuples
							// from the left, and all tuples in the current group have a match.
							// {{end}}
							// {{else if _JOIN_TYPE.IsRightAnti}}
							// With RIGHT ANTI join, we are only interested in unmatched tuples
							// from the right, and all tuples in the current group have a match.
							// {{else}}
							// Neither group ends with the batch, so add the group to the
							// circular buffer.
							o.groups.addGroupsToNextCol(beginLIdx, lGroupLength, beginRIdx, rGroupLength)
							// {{end}}
						}
					} else { // mismatch
						// The line below is a compact form of the following:
						//   incrementLeft :=
						//    (cmp < 0 && o.left.directions[eqColIdx] == execinfrapb.Ordering_Column_ASC) ||
						//	  (cmp > 0 && o.left.directions[eqColIdx] == execinfrapb.Ordering_Column_DESC).
						incrementLeft := cmp < 0 == (o.left.directions[eqColIdx] == execinfrapb.Ordering_Column_ASC)
						if incrementLeft {
							curLIdx++
							_INCREMENT_LEFT_SWITCH(_JOIN_TYPE, _SEL_ARG)
						} else {
							curRIdx++
							_INCREMENT_RIGHT_SWITCH(_JOIN_TYPE, _SEL_ARG)
						}
					}
				}
				_PROCESS_NOT_LAST_GROUP_IN_COLUMN_SWITCH(_JOIN_TYPE)
				// Both o.proberState.lIdx and o.proberState.rIdx should point
				// to the last tuples that have been fully processed in their
				// respective batches. This is the case when we've just finished
				// the last equality column or the current column is such that
				// all tuples were filtered out.
				if lastEqCol || !o.groups.hasGroupForNextCol() {
					o.proberState.lIdx = curLIdx
					o.proberState.rIdx = curRIdx
				}
			}
			// {{end}}
		}
		// {{end}}
	default:
		colexecerror.InternalError(errors.AssertionFailedf("unhandled type %s", colType))
	}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet processes an unmatched group from the left.
func _LEFT_UNMATCHED_GROUP_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "leftUnmatchedGroupSwitch" -}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Unmatched groups are not possible with INNER, LEFT SEMI, RIGHT SEMI, and
	// INTERSECT ALL joins (the latter has IsLeftSemi == true), so there is
	// nothing to do here.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	if lGroup.unmatched {
		// The row already does not have a match, so we don't need to do any
		// additional processing.
		o.groups.addLeftUnmatchedGroup(curLIdx, curRIdx)
		if lastEqCol && curLIdx >= o.proberState.lIdx {
			o.proberState.lIdx = curLIdx + 1
		}
		continue
	}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	// {{/*
	// Unmatched groups from the left are not possible with RIGHT OUTER and
	// RIGHT ANTI joins, so there is nothing to do here.
	// */}}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet processes an unmatched group from the right.
func _RIGHT_UNMATCHED_GROUP_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "rightUnmatchedGroupSwitch" -}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Unmatched groups are not possible with INNER, LEFT SEMI, RIGHT SEMI, and
	// INTERSECT ALL joins (the latter has IsLeftSemi == true), so there is
	// nothing to do here.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	// {{/*
	// Unmatched groups from the right are not possible with LEFT OUTER, LEFT
	// ANTI, and EXCEPT ALL joins (the latter has IsLeftAnti == true), so there
	// is nothing to do here.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	if rGroup.unmatched {
		// The row already does not have a match, so we don't need to do any
		// additional processing.
		o.groups.addRightUnmatchedGroup(curLIdx, curRIdx)
		if lastEqCol && curRIdx >= o.proberState.rIdx {
			o.proberState.rIdx = curRIdx + 1
		}
		continue
	}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet decides what to do if we encounter null in the equality
// column from the left input. Note that the case of Null equality *must* be
// checked separately.
func _NULL_FROM_LEFT_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "nullFromLeftSwitch" -}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Nulls coming from the left input are ignored in INNER, LEFT SEMI, and
	// RIGHT SEMI joins.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	o.groups.addLeftUnmatchedGroup(curLIdx, curRIdx)
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	// {{/*
	// Nulls coming from the left input are ignored in RIGHT OUTER and RIGHT
	// ANTI joins.
	// */}}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet decides what to do if we encounter null in the equality
// column from the right input. Note that the case of Null equality *must* be
// checked separately.
func _NULL_FROM_RIGHT_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "nullFromRightSwitch" -}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Nulls coming from the right input are ignored in INNER, LEFT SEMI, and
	// RIGHT SEMI joins.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	// {{/*
	// Nulls coming from the right input are ignored in LEFT OUTER and LEFT
	// ANTI joins.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	o.groups.addRightUnmatchedGroup(curLIdx, curRIdx)
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet decides what to do when - while looking for a match
// between two inputs - we need to advance the left side, i.e. it decides how
// to handle an unmatched tuple from the left.
func _INCREMENT_LEFT_SWITCH(_JOIN_TYPE joinTypeInfo, _SEL_PERMUTATION selPermutation) { // */}}
	// {{define "incrementLeftSwitch" -}}
	// {{$sel := $.SelPermutation}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Unmatched tuple from the left source is not outputted in INNER, LEFT
	// SEMI, RIGHT SEMI, and INTERSECT ALL joins (the latter has
	// IsLeftSemi == true).
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	// All the rows on the left within the current group will not get a match on
	// the right, so we're adding each of them as a left unmatched group.
	o.groups.addLeftUnmatchedGroup(curLIdx-1, curRIdx)
	for curLIdx < curLEndIdx {
		// {{/*
		//     EXCEPT ALL join allows NULL equality, so we have special
		//     treatment of NULLs.
		// */}}
		// {{if _JOIN_TYPE.IsSetOp}}
		newLValNull := lNulls.NullAt(_L_SEL_IND)
		if lNull != newLValNull {
			// We have a null mismatch, so we've reached the end of the current
			// group on the left.
			break
		} else if newLValNull && lNull {
			nullMatch = true
		} else {
			nullMatch = false
		}
		// {{else}}
		if lNulls.NullAt(_L_SEL_IND) {
			break
		}
		// {{end}}

		// {{if _JOIN_TYPE.IsSetOp}}
		// {{/*
		//     We have checked for null equality above and set nullMatch to the
		//     correct value. If it is true, then both the old and the new
		//     values are NULL, so there is no further comparison needed.
		// */}}
		if !nullMatch {
			// {{end}}
			lSelIdx = _L_SEL_IND
			// {{with .Global}}
			newLVal := lKeys.Get(lSelIdx)
			_ASSIGN_EQ(match, newLVal, lVal, _, lKeys, lKeys)
			// {{end}}
			if !match {
				break
			}
			// {{if _JOIN_TYPE.IsSetOp}}
		}
		// {{end}}
		o.groups.addLeftUnmatchedGroup(curLIdx, curRIdx)
		curLIdx++
	}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	// {{/*
	// Unmatched tuple from the left source is not outputted in RIGHT OUTER
	// and RIGHT ANTI joins.
	// */}}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet decides what to do when - while looking for a match
// between two inputs - we need to advance the right side, i.e. it decides how
// to handle an unmatched tuple from the right.
func _INCREMENT_RIGHT_SWITCH(_JOIN_TYPE joinTypeInfo, _SEL_PERMUTATION selPermutation) { // */}}
	// {{define "incrementRightSwitch" -}}
	// {{$sel := $.SelPermutation}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Unmatched tuple from the right source is not outputted in INNER, LEFT
	// SEMI, RIGHT SEMI, and INTERSECT ALL joins (the latter has
	// IsLeftSemi == true).
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	// {{/*
	// Unmatched tuple from the right source is not outputted in LEFT OUTER,
	// LEFT ANTI, and EXCEPT ALL joins (the latter has IsLeftAnti == true).
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	// All the rows on the right within the current group will not get a match on
	// the left, so we're adding each of them as a right unmatched group.
	o.groups.addRightUnmatchedGroup(curLIdx, curRIdx-1)
	for curRIdx < curREndIdx {
		if rNulls.NullAt(_R_SEL_IND) {
			break
		}
		rSelIdx = _R_SEL_IND
		// {{with .Global}}
		newRVal := rKeys.Get(rSelIdx)
		_ASSIGN_EQ(match, newRVal, rVal, _, rKeys, rKeys)
		// {{end}}
		if !match {
			break
		}
		o.groups.addRightUnmatchedGroup(curLIdx, curRIdx)
		curRIdx++
	}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{/*
// This code snippet processes all but last groups in a column after we have
// reached the end of either the left or right group.
func _PROCESS_NOT_LAST_GROUP_IN_COLUMN_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "processNotLastGroupInColumnSwitch" -}}
	// {{if or $.JoinType.IsInner (or $.JoinType.IsLeftSemi $.JoinType.IsRightSemi)}}
	// {{/*
	// Nothing to do here since an unmatched tuple is omitted.
	// */}}
	// {{end}}
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	if !o.groups.isLastGroupInCol() {
		// The current group is not the last one within the column, so it cannot be
		// extended into the next batch, and we need to process it right now. Any
		// unprocessed row in the left group will not get a match, so each one of
		// them becomes a new unmatched group with a corresponding null group.
		for curLIdx < curLEndIdx {
			o.groups.addLeftUnmatchedGroup(curLIdx, curRIdx)
			curLIdx++
		}
	}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	if !o.groups.isLastGroupInCol() {
		// The current group is not the last one within the column, so it cannot be
		// extended into the next batch, and we need to process it right now. Any
		// unprocessed row in the right group will not get a match, so each one of
		// them becomes a new unmatched group with a corresponding null group.
		for curRIdx < curREndIdx {
			o.groups.addRightUnmatchedGroup(curLIdx, curRIdx)
			curRIdx++
		}
	}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

// {{range $sel := $.SelPermutations}}
func (o *mergeJoin_JOIN_TYPE_STRINGOp) probeBodyLSel_IS_L_SELRSel_IS_R_SEL() {
	lSel := o.proberState.lBatch.Selection()
	rSel := o.proberState.rBatch.Selection()
	for eqColIdx := 0; eqColIdx < len(o.left.eqCols); eqColIdx++ {
		leftColIdx := o.left.eqCols[eqColIdx]
		rightColIdx := o.right.eqCols[eqColIdx]
		lVec := o.proberState.lBatch.ColVec(int(leftColIdx))
		rVec := o.proberState.rBatch.ColVec(int(rightColIdx))
		colType := o.left.sourceTypes[leftColIdx]
		lastEqCol := eqColIdx == len(o.left.eqCols)-1
		_PROBE_SWITCH(_JOIN_TYPE, _SEL_ARG)
		// Look at the groups associated with the next equality column by moving
		// the circular buffer pointer up.
		o.groups.finishedCol()
	}
}

// {{end}}

// {{/*
// This code snippet builds the output corresponding to the left side (i.e. is
// the main body of buildLeftGroupsFromBatch()).
func _LEFT_SWITCH(_JOIN_TYPE joinTypeInfo, _HAS_SELECTION bool) { // */}}
	// {{define "leftSwitch" -}}
	var srcNulls *coldata.Nulls
	if src != nil {
		srcNulls = src.Nulls()
	}
	outNulls := out.Nulls()
	switch input.canonicalTypeFamilies[colIdx] {
	// {{range $.Global.Overloads}}
	case _CANONICAL_TYPE_FAMILY:
		switch input.sourceTypes[colIdx].Width() {
		// {{range .WidthOverloads}}
		case _TYPE_WIDTH:
			var srcCol _GOTYPESLICE
			if src != nil {
				srcCol = src.TemplateType()
			}
			outCol := out.TemplateType()

			// Loop over every group.
			for ; o.builderState.left.groupsIdx < len(leftGroups); o.builderState.left.groupsIdx++ {
				leftGroup := &leftGroups[o.builderState.left.groupsIdx]
				// {{if _JOIN_TYPE.IsLeftAnti}}
				// {{/*
				// With LEFT ANTI and EXCEPT ALL joins (the latter has
				// IsLeftAnti == true) we want to emit output corresponding only to
				// unmatched tuples, so we're skipping all "matched" groups.
				// */}}
				if !leftGroup.unmatched {
					continue
				}
				// {{end}}
				// If curSrcStartIdx is uninitialized, start it at the group's start idx.
				// Otherwise continue where we left off.
				if o.builderState.left.curSrcStartIdx == zeroMJCPCurSrcStartIdx {
					o.builderState.left.curSrcStartIdx = leftGroup.rowStartIdx
				}
				// Loop over every row in the group.
				for ; o.builderState.left.curSrcStartIdx < leftGroup.rowEndIdx; o.builderState.left.curSrcStartIdx++ {
					// Repeat each row numRepeats times.
					// {{if _HAS_SELECTION}}
					srcStartIdx := sel[o.builderState.left.curSrcStartIdx]
					// {{else}}
					srcStartIdx := o.builderState.left.curSrcStartIdx
					// {{end}}

					// {{/* repeatsLeft will always be positive. */}}
					repeatsLeft := leftGroup.numRepeats - o.builderState.left.numRepeatsIdx
					toAppend := repeatsLeft
					if outStartIdx+toAppend > o.outputCapacity {
						toAppend = o.outputCapacity - outStartIdx
						if toAppend == 0 {
							// {{/*
							//     We reached the capacity of the output, so
							//     exit or move onto the next column.
							// */}}
							if lastSrcCol {
								return
							}
							o.builderState.left.setBuilderColumnState(initialBuilderState)
							continue LeftColLoop
						}
					}

					// {{if or _JOIN_TYPE.IsRightOuter _JOIN_TYPE.IsRightAnti}}
					// {{/*
					// Null groups on the left can only occur with RIGHT OUTER and FULL
					// OUTER (for both of which IsRightOuter is true) and RIGHT ANTI joins.
					// For other joins, we're omitting this check.
					// */}}
					if leftGroup.nullGroup {
						outNulls.SetNullRange(outStartIdx, outStartIdx+toAppend)
					} else
					// {{end}}
					{
						if srcNulls.NullAt(srcStartIdx) {
							outNulls.SetNullRange(outStartIdx, outStartIdx+toAppend)
						} else {
							// {{if not .IsBytesLike}}
							// {{if .Sliceable}}
							outCol := outCol[outStartIdx:]
							_ = outCol[toAppend-1]
							// {{end}}
							val := srcCol.Get(srcStartIdx)
							// {{end}}
							for i := 0; i < toAppend; i++ {
								// {{if .IsBytesLike}}
								outCol.Copy(srcCol, outStartIdx+i, srcStartIdx)
								// {{else}}
								// {{if .Sliceable}}
								// {{/*
								//     For the sliceable types, we sliced outCol
								//     to start at outStartIdx, so we use index
								//     i directly.
								// */}}
								//gcassert:bce
								outCol.Set(i, val)
								// {{else}}
								// {{/*
								//     For the non-sliceable types, outCol
								//     vector is the original one (i.e. without
								//     an adjustment), so we need to add
								//     outStartIdx to set the element at the
								//     correct index.
								// */}}
								outCol.Set(outStartIdx+i, val)
								// {{end}}
								// {{end}}
							}
						}
					}
					outStartIdx += toAppend

					if toAppend < repeatsLeft {
						// We didn't materialize all the rows in the group so save state and
						// move to the next column.
						o.builderState.left.numRepeatsIdx += toAppend
						if lastSrcCol {
							return
						}
						o.builderState.left.setBuilderColumnState(initialBuilderState)
						continue LeftColLoop
					}

					o.builderState.left.numRepeatsIdx = zeroMJCPNumRepeatsIdx
				}
				o.builderState.left.curSrcStartIdx = zeroMJCPCurSrcStartIdx
			}
			o.builderState.left.groupsIdx = zeroMJCPGroupsIdx
			// {{end}}
		}
		// {{end}}
	default:
		colexecerror.InternalError(errors.AssertionFailedf("unhandled type %s", input.sourceTypes[colIdx].String()))
	}
	// {{end}}
	// {{/*
}

// */}}

// buildLeftGroupsFromBatch takes a []group and expands each group into the
// output by repeating each row in the group numRepeats times. For example,
// given an input table:
//
//	L1 |  L2
//	--------
//	1  |  a
//	1  |  b
//
// and leftGroups = [{startIdx: 0, endIdx: 2, numRepeats: 3}]
// then buildLeftGroupsFromBatch expands this to
//
//	L1 |  L2
//	--------
//	1  |  a
//	1  |  a
//	1  |  a
//	1  |  b
//	1  |  b
//	1  |  b
//
// Note: this is different from buildRightGroupsFromBatch in that each row of
// group is repeated numRepeats times, instead of a simple copy of the group as
// a whole.
// SIDE EFFECTS: writes into o.output.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) buildLeftGroupsFromBatch(
	leftGroups []group, input *mergeJoinInput, batch coldata.Batch, destStartIdx int,
) {
	sel := batch.Selection()
	initialBuilderState := o.builderState.left
	o.unlimitedAllocator.PerformOperation(
		o.output.ColVecs()[:len(input.sourceTypes)],
		func() {
			// Loop over every column.
		LeftColLoop:
			for colIdx := range input.sourceTypes {
				lastSrcCol := colIdx == len(input.sourceTypes)-1
				outStartIdx := destStartIdx
				out := o.output.ColVec(colIdx)
				var src *coldata.Vec
				if batch.Length() > 0 {
					src = batch.ColVec(colIdx)
				}
				if sel != nil {
					_LEFT_SWITCH(_JOIN_TYPE, true)
				} else {
					_LEFT_SWITCH(_JOIN_TYPE, false)
				}
				o.builderState.left.setBuilderColumnState(initialBuilderState)
			}
			o.builderState.left.reset()
		},
	)
}

// {{/*
// This code snippet builds the output corresponding to the right side (i.e. is
// the main body of buildRightGroupsFromBatch()).
func _RIGHT_SWITCH(_JOIN_TYPE joinTypeInfo, _HAS_SELECTION bool) { // */}}
	// {{define "rightSwitch" -}}
	var srcNulls *coldata.Nulls
	if src != nil {
		srcNulls = src.Nulls()
	}
	outNulls := out.Nulls()
	switch input.canonicalTypeFamilies[colIdx] {
	// {{range $.Global.Overloads}}
	case _CANONICAL_TYPE_FAMILY:
		switch input.sourceTypes[colIdx].Width() {
		// {{range .WidthOverloads}}
		case _TYPE_WIDTH:
			var srcCol _GOTYPESLICE
			if src != nil {
				srcCol = src.TemplateType()
			}
			outCol := out.TemplateType()

			// Loop over every group.
			for ; o.builderState.right.groupsIdx < len(rightGroups); o.builderState.right.groupsIdx++ {
				rightGroup := &rightGroups[o.builderState.right.groupsIdx]
				// Repeat every group numRepeats times.
				for ; o.builderState.right.numRepeatsIdx < rightGroup.numRepeats; o.builderState.right.numRepeatsIdx++ {
					if o.builderState.right.curSrcStartIdx == zeroMJCPCurSrcStartIdx {
						o.builderState.right.curSrcStartIdx = rightGroup.rowStartIdx
					}
					toAppend := rightGroup.rowEndIdx - o.builderState.right.curSrcStartIdx
					if outStartIdx+toAppend > o.outputCapacity {
						toAppend = o.outputCapacity - outStartIdx
					}

					// {{if _JOIN_TYPE.IsLeftOuter}}
					// {{/*
					// Null groups on the right can only occur with LEFT OUTER and FULL
					// OUTER joins for both of which IsLeftOuter is true. For other joins,
					// we're omitting this check.
					// */}}
					if rightGroup.nullGroup {
						outNulls.SetNullRange(outStartIdx, outStartIdx+toAppend)
					} else
					// {{end}}
					{
						// Optimization in the case that group length is 1, use assign
						// instead of copy.
						if toAppend == 1 {
							// {{if _HAS_SELECTION}}
							srcIdx := sel[o.builderState.right.curSrcStartIdx]
							// {{else}}
							srcIdx := o.builderState.right.curSrcStartIdx
							// {{end}}
							if srcNulls.NullAt(srcIdx) {
								outNulls.SetNull(outStartIdx)
							} else {
								// {{if .IsBytesLike}}
								outCol.Copy(srcCol, outStartIdx, srcIdx)
								// {{else}}
								v := srcCol.Get(srcIdx)
								outCol.Set(outStartIdx, v)
								// {{end}}
							}
						} else {
							out.Copy(
								coldata.SliceArgs{
									Src:         src,
									Sel:         sel,
									DestIdx:     outStartIdx,
									SrcStartIdx: o.builderState.right.curSrcStartIdx,
									SrcEndIdx:   o.builderState.right.curSrcStartIdx + toAppend,
								},
							)
						}
					}

					outStartIdx += toAppend

					// If we haven't materialized all the rows from the group, then we are
					// done with the current column.
					if toAppend < rightGroup.rowEndIdx-o.builderState.right.curSrcStartIdx {
						// If it's the last column, save state and return.
						if lastSrcCol {
							o.builderState.right.curSrcStartIdx += toAppend
							return
						}
						// Otherwise, reset to the initial state and begin the next column.
						o.builderState.right.setBuilderColumnState(initialBuilderState)
						continue RightColLoop
					}
					o.builderState.right.curSrcStartIdx = zeroMJCPCurSrcStartIdx
				}
				o.builderState.right.numRepeatsIdx = zeroMJCPNumRepeatsIdx
			}
			o.builderState.right.groupsIdx = zeroMJCPGroupsIdx
			// {{end}}
		}
	// {{end}}
	default:
		colexecerror.InternalError(errors.AssertionFailedf("unhandled type %s", input.sourceTypes[colIdx].String()))
	}
	// {{end}}
	// {{/*
}

// */}}

// buildRightGroupsFromBatch takes a []group and repeats each group numRepeats
// times. For example, given an input table:
//
//	R1 |  R2
//	--------
//	1  |  a
//	1  |  b
//
// and rightGroups = [{startIdx: 0, endIdx: 2, numRepeats: 3}]
// then buildRightGroups expands this to
//
//	R1 |  R2
//	--------
//	1  |  a
//	1  |  b
//	1  |  a
//	1  |  b
//	1  |  a
//	1  |  b
//
// Note: this is different from buildLeftGroupsFromBatch in that each group is
// not expanded but directly copied numRepeats times.
// SIDE EFFECTS: writes into o.output.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) buildRightGroupsFromBatch(
	rightGroups []group, colOffset int, input *mergeJoinInput, batch coldata.Batch, destStartIdx int,
) {
	initialBuilderState := o.builderState.right
	sel := batch.Selection()
	o.unlimitedAllocator.PerformOperation(
		o.output.ColVecs()[colOffset:colOffset+len(input.sourceTypes)],
		func() {
			// Loop over every column.
		RightColLoop:
			for colIdx := range input.sourceTypes {
				lastSrcCol := colIdx == len(input.sourceTypes)-1
				outStartIdx := destStartIdx
				out := o.output.ColVec(colIdx + colOffset)
				var src *coldata.Vec
				if batch.Length() > 0 {
					src = batch.ColVec(colIdx)
				}
				if sel != nil {
					_RIGHT_SWITCH(_JOIN_TYPE, true)
				} else {
					_RIGHT_SWITCH(_JOIN_TYPE, false)
				}
				o.builderState.right.setBuilderColumnState(initialBuilderState)
			}
			o.builderState.right.reset()
		})
}

// probe is where we generate the groups slices that are used in the build
// phase. We do this by first assuming that every row in both batches
// contributes to the cross product. Then, with every equality column, we
// filter out the rows that don't contribute to the cross product (i.e. they
// don't have a matching row on the other side in the case of an inner join),
// and set the correct cardinality.
// Note that in this phase, we do this for every group, except the last group
// in the batch.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) probe() {
	o.groups.reset(o.proberState.lIdx, o.proberState.lLength, o.proberState.rIdx, o.proberState.rLength)
	lSel := o.proberState.lBatch.Selection()
	rSel := o.proberState.rBatch.Selection()
	if lSel != nil {
		if rSel != nil {
			o.probeBodyLSeltrueRSeltrue()
		} else {
			o.probeBodyLSeltrueRSelfalse()
		}
	} else {
		if rSel != nil {
			o.probeBodyLSelfalseRSeltrue()
		} else {
			o.probeBodyLSelfalseRSelfalse()
		}
	}
}

// exhaustLeftSource sets up the builder to process any remaining tuples from
// the left source. It should only be called when the right source has been
// exhausted.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) exhaustLeftSource() {
	// {{if or _JOIN_TYPE.IsInner (or _JOIN_TYPE.IsLeftSemi _JOIN_TYPE.IsRightSemi)}}
	// {{/*
	// Remaining tuples from the left source do not have a match, so they are
	// ignored in INNER, LEFT SEMI, RIGHT SEMI, and INTERSECT ALL joins (the
	// latter has IsLeftSemi == true).
	// */}}
	// {{end}}
	// {{if or _JOIN_TYPE.IsLeftOuter _JOIN_TYPE.IsLeftAnti}}
	// The capacity of builder state lGroups and rGroups is always at least 1
	// given the init.
	o.builderState.lGroups = o.builderState.lGroups[:1]
	o.builderState.lGroups[0] = group{
		rowStartIdx: o.proberState.lIdx,
		rowEndIdx:   o.proberState.lLength,
		numRepeats:  1,
		toBuild:     o.proberState.lLength - o.proberState.lIdx,
		unmatched:   true,
	}
	// {{if _JOIN_TYPE.IsLeftOuter}}
	o.builderState.rGroups = o.builderState.rGroups[:1]
	o.builderState.rGroups[0] = group{
		rowStartIdx: o.proberState.lIdx,
		rowEndIdx:   o.proberState.lLength,
		numRepeats:  1,
		toBuild:     o.proberState.lLength - o.proberState.lIdx,
		nullGroup:   true,
	}
	// {{end}}

	o.proberState.lIdx = o.proberState.lLength
	// {{end}}
	// {{if or _JOIN_TYPE.IsRightOuter _JOIN_TYPE.IsRightAnti}}
	// {{/*
	// Remaining tuples from the left source do not have a match, so they are
	// ignored in RIGHT OUTER and RIGHT ANTI joins.
	// */}}
	// {{end}}
}

// exhaustRightSource sets up the builder to process any remaining tuples from
// the right source. It should only be called when the left source has been
// exhausted.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) exhaustRightSource() {
	// {{if or _JOIN_TYPE.IsRightOuter _JOIN_TYPE.IsRightAnti}}
	// The capacity of builder state lGroups and rGroups is always at least 1
	// given the init.
	// {{if _JOIN_TYPE.IsRightOuter}}
	o.builderState.lGroups = o.builderState.lGroups[:1]
	o.builderState.lGroups[0] = group{
		rowStartIdx: o.proberState.rIdx,
		rowEndIdx:   o.proberState.rLength,
		numRepeats:  1,
		toBuild:     o.proberState.rLength - o.proberState.rIdx,
		nullGroup:   true,
	}
	// {{end}}
	o.builderState.rGroups = o.builderState.rGroups[:1]
	o.builderState.rGroups[0] = group{
		rowStartIdx: o.proberState.rIdx,
		rowEndIdx:   o.proberState.rLength,
		numRepeats:  1,
		toBuild:     o.proberState.rLength - o.proberState.rIdx,
		unmatched:   true,
	}

	o.proberState.rIdx = o.proberState.rLength
	// {{else}}
	// Remaining tuples from the right source do not have a match, so they are
	// ignored in all joins except for RIGHT OUTER and FULL OUTER.
	// {{end}}
}

// numBuiltFromBatch uses the toBuild field of each group and the output
// capacity to determine the output count. Note that as soon as a group is
// materialized partially or fully to output, its toBuild field is updated
// accordingly. The number of tuples that will be built from batch during the
// current iteration is returned.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) numBuiltFromBatch(groups []group) (numBuilt int) {
	outCount := o.builderState.outCount
	for i := 0; i < len(groups); i++ {
		// {{if or _JOIN_TYPE.IsLeftAnti _JOIN_TYPE.IsRightAnti}}
		if !groups[i].unmatched {
			// "Matched" groups are not outputted in LEFT ANTI, RIGHT ANTI,
			// and EXCEPT ALL joins (for the latter IsLeftAnti == true), so
			// they do not contribute to the output count.
			continue
		}
		// {{end}}
		outCount += groups[i].toBuild
		groups[i].toBuild = 0
		if outCount > o.outputCapacity {
			groups[i].toBuild = outCount - o.outputCapacity
			return o.outputCapacity - o.builderState.outCount
		}
	}
	return outCount - o.builderState.outCount
}

// buildFromBatch builds as many output rows as possible from the groups that
// were complete in the probing batches. New rows are put starting at
// o.builderState.outCount position until either the capacity is reached or all
// groups are processed.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) buildFromBatch() {
	outStartIdx := o.builderState.outCount
	// {{if or _JOIN_TYPE.IsRightSemi _JOIN_TYPE.IsRightAnti}}
	numBuilt := o.numBuiltFromBatch(o.builderState.rGroups)
	// {{else}}
	numBuilt := o.numBuiltFromBatch(o.builderState.lGroups)
	// {{end}}
	o.builderState.outCount += numBuilt
	if numBuilt > 0 && len(o.outputTypes) != 0 {
		// We will be actually building the output if we have columns in the output
		// batch (meaning that we're not doing query like 'SELECT count(*) ...')
		// and when builderState.outCount has increased (meaning that we have
		// something to build).
		colOffsetForRightGroups := 0
		// {{if not (or _JOIN_TYPE.IsRightSemi _JOIN_TYPE.IsRightAnti)}}
		o.buildLeftGroupsFromBatch(o.builderState.lGroups, &o.left, o.proberState.lBatch, outStartIdx)
		colOffsetForRightGroups = len(o.left.sourceTypes)
		_ = colOffsetForRightGroups
		// {{end}}
		// {{if not (or _JOIN_TYPE.IsLeftSemi _JOIN_TYPE.IsLeftAnti)}}
		o.buildRightGroupsFromBatch(o.builderState.rGroups, colOffsetForRightGroups, &o.right, o.proberState.rBatch, outStartIdx)
		// {{end}}
	}
}

// transitionIntoBuildingFromBufferedGroup should be called once we have
// non-empty right buffered group in order to setup the buffered group builder.
//
// It will complete the right buffered group (meaning it'll read all batches
// from the right input until either the new group is found or the input is
// exhausted).
//
// For a more detailed explanation and an example please refer to the comment at
// the top of mergejoiner.go.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) transitionIntoBuildingFromBufferedGroup() {
	if o.proberState.rIdx == o.proberState.rLength {
		// The right buffered group might extend into the next batch, so we have
		// to complete it first.
		o.completeRightBufferedGroup()
	}

	o.bufferedGroup.helper.setupLeftBuilder()

	// {{if and _JOIN_TYPE.IsLeftAnti _JOIN_TYPE.IsSetOp}}
	// For EXCEPT ALL joins we build # left tuples - # right tuples output rows
	// (if positive), so we have to discard first numRightTuples rows from the
	// left.
	numSkippedLeft := 0
	for {
		groupLength := o.proberState.lIdx - o.bufferedGroup.leftGroupStartIdx
		if numSkippedLeft+groupLength > o.bufferedGroup.helper.numRightTuples {
			// The current left batch is the first one that contains tuples
			// without a "match".
			break
		}
		numSkippedLeft += groupLength
		var groupFinished bool
		if o.proberState.lIdx < o.proberState.lLength {
			// The group on the left is finished within the current left
			// batch.
			groupFinished = true
		} else {
			// Fetch the next batch from the left input and calculate the
			// boundaries of the buffered group.
			o.continueLeftBufferedGroup()
			groupFinished = o.proberState.lIdx == 0
		}
		if groupFinished {
			// We have less matching tuples on the left than on the right, so we
			// don't emit any output for this buffered group.
			o.bufferedGroup.helper.Reset(o.Ctx)
			o.state = mjEntry
			return
		}
	}
	// We might need to skip some tuples in the current left batch since they
	// still had matches with the right side.
	toSkipInThisBatch := o.bufferedGroup.helper.numRightTuples - numSkippedLeft
	startIdx := o.bufferedGroup.leftGroupStartIdx + toSkipInThisBatch
	// {{else}}
	startIdx := o.bufferedGroup.leftGroupStartIdx
	// {{end}}

	o.bufferedGroup.helper.prepareForNextLeftBatch(o.proberState.lBatch, startIdx, o.proberState.lIdx)
	o.state = mjBuildFromBufferedGroup
}

// buildFromBufferedGroup builds the output based on the current buffered group
// and puts new tuples starting at position b.builderState.outCount. It returns
// true once the output for the buffered group has been fully populated.
//
// It is assumed that transitionIntoBuildingFromBufferedGroup has been called.
//
// For a more detailed explanation and an example please refer to the comment at
// the top of mergejoiner.go.
func (o *mergeJoin_JOIN_TYPE_STRINGOp) buildFromBufferedGroup() (bufferedGroupComplete bool) {
	bg := &o.bufferedGroup
	// Iterate until either we use up the whole capacity of the output batch or
	// we complete the buffered group.
	for {
		if bg.helper.builderState.left.curSrcStartIdx == o.proberState.lLength {
			// The output has been fully built from the current left batch.
			bg.leftBatchDone = true
		}
		if bg.leftBatchDone {
			// The current left batch has been fully processed with regards to
			// the buffered group.
			bg.leftBatchDone = false
			if o.proberState.lIdx < o.proberState.lLength {
				// The group on the left is finished within the current left
				// batch.
				return true
			}
			var skipLeftBufferedGroup bool
			// {{if _JOIN_TYPE.IsRightSemi}}
			// For RIGHT SEMI joins we have already fully built the output based
			// on all tuples in the right buffered group using the match from
			// the current left batch. This allows us to simply skip all tuples
			// that are part of the left buffered group.
			skipLeftBufferedGroup = true
			// {{else if and _JOIN_TYPE.IsLeftSemi _JOIN_TYPE.IsSetOp}}
			if bg.helper.builderState.numEmittedTotal == bg.helper.numRightTuples {
				// For INTERSECT ALL joins we build min(# left tuples, # right
				// tuples), and we have already reached the number of tuples
				// from the right. Thus, we have to skip all tuples from the
				// left that are part of the buffered group since they don't
				// have a match.
				skipLeftBufferedGroup = true
			}
			// {{end}}
			if skipLeftBufferedGroup {
				// Keep fetching the next batch from the left input until we
				// either find the start of the new group or we exhaust the
				// input.
				for o.proberState.lIdx == o.proberState.lLength && o.proberState.lLength > 0 {
					o.continueLeftBufferedGroup()
				}
				return true
			}
			// Fetch the next batch from the left input and calculate the
			// boundaries of the buffered group.
			o.continueLeftBufferedGroup()
			if o.proberState.lIdx == 0 {
				return true
			}
			bg.helper.prepareForNextLeftBatch(
				o.proberState.lBatch, bg.leftGroupStartIdx, o.proberState.lIdx,
			)
		}

		willEmit := bg.helper.canEmit()
		if o.builderState.outCount+willEmit > o.outputCapacity {
			willEmit = o.outputCapacity - o.builderState.outCount
		} else {
			bg.leftBatchDone = true
		}
		if willEmit > 0 && len(o.outputTypes) != 0 {
			// {{if not (or _JOIN_TYPE.IsRightSemi _JOIN_TYPE.IsRightAnti)}}
			bg.helper.buildFromLeftInput(o.Ctx, o.builderState.outCount)
			// {{end}}
			// {{if not (or _JOIN_TYPE.IsLeftSemi _JOIN_TYPE.IsLeftAnti)}}
			bg.helper.buildFromRightInput(o.Ctx, o.builderState.outCount)
			// {{end}}
		}
		o.builderState.outCount += willEmit
		// {{if or (or _JOIN_TYPE.IsInner _JOIN_TYPE.IsLeftOuter) _JOIN_TYPE.IsRightOuter}}
		bg.helper.builderState.numEmittedCurLeftBatch += willEmit
		// {{end}}
		// {{if or (or _JOIN_TYPE.IsRightSemi _JOIN_TYPE.IsRightAnti) (and _JOIN_TYPE.IsLeftSemi _JOIN_TYPE.IsSetOp)}}
		bg.helper.builderState.numEmittedTotal += willEmit
		// {{end}}
		if o.builderState.outCount == o.outputCapacity {
			return false
		}
	}
}

// {{/*
// This code snippet is executed when at least one of the input sources has
// been exhausted. It processes any remaining tuples and then sets up the
// builder.
func _SOURCE_FINISHED_SWITCH(_JOIN_TYPE joinTypeInfo) { // */}}
	// {{define "sourceFinishedSwitch" -}}
	o.builderState.lGroups = o.builderState.lGroups[:0]
	o.builderState.rGroups = o.builderState.rGroups[:0]
	// {{if or $.JoinType.IsLeftOuter $.JoinType.IsLeftAnti}}
	// At least one of the sources is finished. If it was the right one,
	// then we need to emit remaining tuples from the left source with
	// nulls corresponding to the right one. But if the left source is
	// finished, then there is nothing left to do.
	if o.proberState.lIdx < o.proberState.lLength {
		o.exhaustLeftSource()
	}
	// {{end}}
	// {{if or $.JoinType.IsRightOuter $.JoinType.IsRightAnti}}
	// At least one of the sources is finished. If it was the left one,
	// then we need to emit remaining tuples from the right source with
	// nulls corresponding to the left one. But if the right source is
	// finished, then there is nothing left to do.
	if o.proberState.rIdx < o.proberState.rLength {
		o.exhaustRightSource()
	}
	// {{end}}
	// {{end}}
	// {{/*
}

// */}}

func (o *mergeJoin_JOIN_TYPE_STRINGOp) Next() coldata.Batch {
	o.output, _ = o.helper.ResetMaybeReallocate(o.outputTypes, o.output, 0 /* tuplesToBeSet */)
	o.outputCapacity = o.output.Capacity()
	o.bufferedGroup.helper.output = o.output
	o.builderState.outCount = 0
	for {
		switch o.state {
		case mjEntry:
			// If this is the first batch or we're done with the current batch,
			// get the next batch.
			if o.proberState.lBatch == nil || (o.proberState.lLength != 0 && o.proberState.lIdx == o.proberState.lLength) {
				o.cancelChecker.CheckEveryCall()
				o.proberState.lIdx, o.proberState.lBatch = 0, o.InputOne.Next()
				o.proberState.lLength = o.proberState.lBatch.Length()
			}
			if o.proberState.rBatch == nil || (o.proberState.rLength != 0 && o.proberState.rIdx == o.proberState.rLength) {
				o.cancelChecker.CheckEveryCall()
				o.proberState.rIdx, o.proberState.rBatch = 0, o.InputTwo.Next()
				o.proberState.rLength = o.proberState.rBatch.Length()
			}
			if o.sourceFinished() {
				o.state = mjSourceFinished
				break
			}
			o.state = mjProbe

		case mjSourceFinished:
			_SOURCE_FINISHED_SWITCH(_JOIN_TYPE)
			if len(o.builderState.lGroups) == 0 && len(o.builderState.rGroups) == 0 {
				o.state = mjDone
				o.output.SetLength(o.builderState.outCount)
				return o.output
			}
			o.state = mjBuildFromBatch

		case mjProbe:
			o.probe()
			o.builderState.lGroups, o.builderState.rGroups = o.groups.getGroups()
			if len(o.builderState.lGroups) > 0 || len(o.builderState.rGroups) > 0 {
				o.state = mjBuildFromBatch
			} else if o.bufferedGroup.helper.numRightTuples != 0 {
				o.transitionIntoBuildingFromBufferedGroup()
			} else {
				o.state = mjEntry
			}

		case mjBuildFromBatch:
			o.buildFromBatch()
			if o.builderState.outCount == o.outputCapacity {
				o.output.SetLength(o.builderState.outCount)
				return o.output
			}
			if o.bufferedGroup.helper.numRightTuples != 0 {
				o.transitionIntoBuildingFromBufferedGroup()
			} else {
				o.state = mjEntry
			}

		case mjBuildFromBufferedGroup:
			bufferedGroupComplete := o.buildFromBufferedGroup()
			if bufferedGroupComplete {
				o.bufferedGroup.helper.Reset(o.Ctx)
				o.state = mjEntry
			}
			if o.builderState.outCount == o.outputCapacity {
				o.output.SetLength(o.builderState.outCount)
				return o.output
			}

		case mjDone:
			return coldata.ZeroBatch

		default:
			colexecerror.InternalError(errors.AssertionFailedf("unexpected merge joiner state in Next: %v", o.state))
		}
	}
}
