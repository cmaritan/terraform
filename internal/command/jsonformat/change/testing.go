package change

import (
	"github.com/zclconf/go-cty/cty"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/hashicorp/terraform/internal/plans"
)

type ValidateChangeFunc func(t *testing.T, change Change)

func validateChange(t *testing.T, change Change, expectedAction plans.Action, expectedReplace bool) {
	if change.replace != expectedReplace || change.action != expectedAction {
		t.Errorf("\nreplace:\n\texpected:%t\n\tactual:%t\naction:\n\texpected:%s\n\tactual:%s", expectedReplace, change.replace, expectedAction, change.action)
	}
}

func ValidatePrimitive(before, after interface{}, ctyType cty.Type, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		primitive, ok := change.renderer.(*primitiveRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		beforeDiff := cmp.Diff(primitive.before, before)
		afterDiff := cmp.Diff(primitive.after, after)
		if len(beforeDiff) > 0 || len(afterDiff) > 0 {
			t.Errorf("before diff: (%s), after diff: (%s)", beforeDiff, afterDiff)
		}

		if !ctyType.Equals(primitive.t) {
			t.Errorf("expected type %s but got type %s", ctyType.FriendlyName(), primitive.t.FriendlyName())
		}
	}
}

func ValidateObject(attributes map[string]ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		object, ok := change.renderer.(*objectRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if !object.overrideNullSuffix {
			t.Errorf("created the wrong type of object renderer")
		}

		validateObject(t, object, attributes)
	}
}

func ValidateNestedObject(attributes map[string]ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		object, ok := change.renderer.(*objectRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if object.overrideNullSuffix {
			t.Errorf("created the wrong type of object renderer")
		}

		validateObject(t, object, attributes)
	}
}

func validateObject(t *testing.T, object *objectRenderer, attributes map[string]ValidateChangeFunc) {
	if len(object.attributes) != len(attributes) {
		t.Errorf("expected %d attributes but found %d attributes", len(attributes), len(object.attributes))
	}

	var missing []string
	for key, expected := range attributes {
		actual, ok := object.attributes[key]
		if !ok {
			missing = append(missing, key)
		}

		if len(missing) > 0 {
			continue
		}

		expected(t, actual)
	}

	if len(missing) > 0 {
		t.Errorf("missing the following attributes: %s", strings.Join(missing, ", "))
	}
}

func ValidateMap(elements map[string]ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		m, ok := change.renderer.(*mapRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if len(m.elements) != len(elements) {
			t.Errorf("expected %d elements but found %d elements", len(elements), len(m.elements))
		}

		var missing []string
		for key, expected := range elements {
			actual, ok := m.elements[key]
			if !ok {
				missing = append(missing, key)
			}

			if len(missing) > 0 {
				continue
			}

			expected(t, actual)
		}

		if len(missing) > 0 {
			t.Errorf("missing the following elements: %s", strings.Join(missing, ", "))
		}
	}
}

func ValidateList(elements []ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		list, ok := change.renderer.(*listRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if !list.displayContext {
			t.Errorf("created the wrong type of list renderer")
		}

		validateList(t, list, elements)
	}
}

func ValidateNestedList(elements []ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		list, ok := change.renderer.(*listRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if list.displayContext {
			t.Errorf("created the wrong type of list renderer")
		}

		validateList(t, list, elements)
	}
}

func validateList(t *testing.T, list *listRenderer, elements []ValidateChangeFunc) {
	if len(list.elements) != len(elements) {
		t.Fatalf("expected %d elements but found %d elements", len(elements), len(list.elements))
	}

	for ix := 0; ix < len(elements); ix++ {
		elements[ix](t, list.elements[ix])
	}
}

func ValidateSet(elements []ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		set, ok := change.renderer.(*setRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if len(set.elements) != len(elements) {
			t.Fatalf("expected %d elements but found %d elements", len(elements), len(set.elements))
		}

		for ix := 0; ix < len(elements); ix++ {
			elements[ix](t, set.elements[ix])
		}
	}
}

func ValidateBlock(attributes map[string]ValidateChangeFunc, blocks map[string][]ValidateChangeFunc, mapBlocks map[string]map[string]ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		block, ok := change.renderer.(*blockRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if len(block.attributes) != len(attributes) || len(block.blocks)+len(block.mapBlocks) != len(blocks)+len(mapBlocks) {
			t.Errorf("expected %d attributes and %d blocks but found %d attributes and %d blocks", len(attributes), len(blocks)+len(mapBlocks), len(block.attributes), len(block.blocks)+len(block.mapBlocks))
		}

		var missingAttributes []string
		var missingBlocks []string

		for key, expected := range attributes {
			actual, ok := block.attributes[key]
			if !ok {
				missingAttributes = append(missingAttributes, key)
			}

			if len(missingAttributes) > 0 {
				continue
			}

			expected(t, actual)
		}

		for key, expected := range blocks {
			actual, ok := block.blocks[key]
			if !ok {
				missingBlocks = append(missingBlocks, key)
			}

			if len(missingAttributes) > 0 || len(missingBlocks) > 0 {
				continue
			}

			if len(expected) != len(actual) {
				t.Errorf("expected %d blocks for %s but found %d", len(expected), key, len(actual))
			}

			for ix := range expected {
				expected[ix](t, actual[ix])
			}
		}

		for key, expectedBlocks := range mapBlocks {
			actualBlocks, ok := block.mapBlocks[key]
			if !ok {
				missingBlocks = append(missingBlocks, key)
			}

			if len(missingAttributes) > 0 || len(missingBlocks) > 0 {
				continue
			}

			if len(expectedBlocks) != len(actualBlocks) {
				t.Fatalf("expected %d map blocks for %s but found %d", len(expectedBlocks), key, len(actualBlocks))
			}

			var missing []string
			for key, expected := range expectedBlocks {
				actual, ok := actualBlocks[key]
				if !ok {
					missing = append(missing, key)
				}

				if len(missing) > 0 {
					continue
				}

				expected(t, actual)
			}

			if len(missing) > 0 {
				t.Fatalf("missing the following map blocks for %s: %s", key, strings.Join(missing, ", "))
			}

		}

		if len(missingAttributes) > 0 || len(missingBlocks) > 0 {
			t.Errorf("missing the following attributes: %s, and the following blocks: %s", strings.Join(missingAttributes, ", "), strings.Join(missingBlocks, ", "))
		}
	}
}

func ValidateTypeChange(before, after ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		typeChange, ok := change.renderer.(*typeChangeRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		before(t, typeChange.before)
		after(t, typeChange.after)
	}
}

func ValidateSensitive(before, after interface{}, beforeSensitive, afterSensitive bool, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		sensitive, ok := change.renderer.(*sensitiveRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if beforeSensitive != sensitive.beforeSensitive || afterSensitive != sensitive.afterSensitive {
			t.Errorf("before or after sensitive values don't match:\n\texpected; before: %t after: %t\n\tactual; before: %t, after: %t", beforeSensitive, afterSensitive, sensitive.beforeSensitive, sensitive.afterSensitive)
		}

		beforeDiff := cmp.Diff(sensitive.before, before)
		afterDiff := cmp.Diff(sensitive.after, after)

		if len(beforeDiff) > 0 || len(afterDiff) > 0 {
			t.Errorf("before diff: (%s), after diff: (%s)", beforeDiff, afterDiff)
		}
	}
}

func ValidateComputed(before ValidateChangeFunc, action plans.Action, replace bool) ValidateChangeFunc {
	return func(t *testing.T, change Change) {
		validateChange(t, change, action, replace)

		computed, ok := change.renderer.(*computedRenderer)
		if !ok {
			t.Fatalf("invalid renderer type: %T", change.renderer)
		}

		if before == nil {
			if computed.before.renderer != nil {
				t.Fatalf("did not expect a before renderer, but found one")
			}
			return
		}

		if computed.before.renderer == nil {
			t.Fatalf("expected a before renderer, but found none")
		}

		before(t, computed.before)
	}
}
