package differ

import (
	"reflect"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/internal/command/jsonformat/change"
	"github.com/hashicorp/terraform/internal/command/jsonprovider"
	"github.com/hashicorp/terraform/internal/plans"
)

func (v Value) computeAttributeChangeAsSet(elementType cty.Type) change.Change {
	var elements []change.Change
	current := v.getDefaultActionForIteration()
	v.processSet(false, func(value Value) {
		element := value.ComputeChange(elementType)
		elements = append(elements, element)
		current = compareActions(current, element.GetAction())
	})
	return change.New(change.Set(elements), current, v.replacePath())
}

func (v Value) computeAttributeChangeAsNestedSet(attributes map[string]*jsonprovider.Attribute) change.Change {
	var elements []change.Change
	current := v.getDefaultActionForIteration()
	v.processSet(true, func(value Value) {
		element := value.ComputeChange(attributes)
		elements = append(elements, element)
		current = compareActions(current, element.GetAction())
	})
	return change.New(change.Set(elements), current, false)
}

func (v Value) computeBlockChangesAsSet(block *jsonprovider.Block) ([]change.Change, plans.Action) {
	var elements []change.Change
	current := v.getDefaultActionForIteration()
	v.processSet(true, func(value Value) {
		element := value.ComputeChange(block)
		elements = append(elements, element)
		current = compareActions(current, element.GetAction())
	})
	return elements, current
}

func (v Value) processSet(propagateReplace bool, process func(value Value)) {
	sliceValue := v.asSlice()

	foundInBefore := make(map[int]int)
	foundInAfter := make(map[int]int)

	// O(n^2) operation here to find matching pairs in the set, so we can make
	// the display look pretty. There might be a better way to do this, so look
	// here for potential optimisations.

	for ix := 0; ix < len(sliceValue.Before); ix++ {
		matched := false
		for jx := 0; jx < len(sliceValue.After); jx++ {
			if _, ok := foundInAfter[jx]; ok {
				// We've already found a match for this after value.
				continue
			}

			child := sliceValue.getChild(ix, jx, propagateReplace)
			if reflect.DeepEqual(child.Before, child.After) && child.isBeforeSensitive() == child.isAfterSensitive() && !anyUnknown(child.Unknown) {
				matched = true
				foundInBefore[ix] = jx
				foundInAfter[jx] = ix
			}
		}

		if !matched {
			foundInBefore[ix] = -1
		}
	}

	// Now everything in before should be a key in foundInBefore and a value
	// in foundInAfter. If a key is mapped to -1 in foundInBefore it means it
	// does not have an equivalent in foundInAfter and so has been deleted.
	// Everything in foundInAfter has a matching value in foundInBefore, but
	// some values in after may not be in foundInAfter. This means these values
	// are newly created.

	for ix := 0; ix < len(sliceValue.Before); ix++ {
		if jx := foundInBefore[ix]; jx >= 0 {
			child := sliceValue.getChild(ix, jx, propagateReplace)
			process(child)
			continue
		}
		child := sliceValue.getChild(ix, len(sliceValue.After), propagateReplace)
		process(child)
	}

	for jx := 0; jx < len(sliceValue.After); jx++ {
		if _, ok := foundInAfter[jx]; ok {
			// Then this value was handled in the previous for loop.
			continue
		}
		child := sliceValue.getChild(len(sliceValue.Before), jx, propagateReplace)
		process(child)
	}
}
