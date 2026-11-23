package registers_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/registers"
)

// testMaterial has no methods; payloads need no identity interface.
type testMaterial uint64

const (
	paragraph testMaterial = iota + 1
	block
	tail
	leftItem
	rightItem
	newItem
	p1
	p2
	p3
	n1
	o1
	local
	note
)

// opaqueMaterial also exercises payloads which are not comparable.
type opaqueMaterial struct {
	testMaterial
	payload any
}

func assertRegister[T any](t *testing.T, r registers.Register[T], want ...T) {
	t.Helper()
	if r.Len() != len(want) {
		t.Fatalf("Len() = %d, want %d", r.Len(), len(want))
	}
	for i, expected := range want {
		first, ok := r.First()
		if !ok || !reflect.DeepEqual(first, expected) {
			t.Fatalf("First() at %d = (%#v, %v), want %#v", i, first, ok, expected)
		}
		item, rest, ok := r.Take()
		if !ok || !reflect.DeepEqual(item, expected) {
			t.Fatalf("Take() at %d = (%#v, _, %v), want %#v", i, item, ok, expected)
		}
		if r.Len() != len(want)-i {
			t.Fatal("Take changed its receiver")
		}
		r = rest
	}
	var zero T
	if first, ok := r.First(); !reflect.DeepEqual(first, zero) || ok {
		t.Fatalf("empty First() = (%#v, %v)", first, ok)
	}
	if item, rest, ok := r.Take(); !reflect.DeepEqual(item, zero) || ok || rest.Len() != 0 {
		t.Fatalf("empty Take() = (%#v, %#v, %v)", item, rest, ok)
	}
}

func TestRegisterVersions(t *testing.T) {
	var empty registers.Register[testMaterial]
	one := empty.Append(paragraph)
	original := one.Append(block).Append(tail)
	_, rest, _ := original.Take()
	left := rest.Append(leftItem)
	right := rest.Append(rightItem)
	_, drained, _ := one.Take()
	reused := drained.Append(newItem)

	assertRegister(t, empty)
	assertRegister(t, one, paragraph)
	assertRegister(t, original, paragraph, block, tail)
	assertRegister(t, rest, block, tail)
	assertRegister(t, left, block, tail, leftItem)
	assertRegister(t, right, block, tail, rightItem)
	assertRegister(t, drained)
	assertRegister(t, reused, newItem)
}

func TestRegisterPrependVersions(t *testing.T) {
	var empty registers.Register[testMaterial]
	one := empty.Prepend(paragraph)
	base := one.Append(block).Append(tail)
	_, rest, _ := base.Take()
	_, rest, _ = rest.Take()
	left := rest.Prepend(leftItem)
	right := rest.Prepend(rightItem)
	repeated := rest.Prepend(p1).Prepend(p2)
	mixed := left.Append(newItem)
	item, restored, ok := left.Take()
	if !ok || item != leftItem {
		t.Fatalf("Take after Prepend = (%v, _, %v)", item, ok)
	}
	_, drained, _ := one.Take()
	reused := drained.Prepend(newItem)

	assertRegister(t, empty)
	assertRegister(t, one, paragraph)
	assertRegister(t, base, paragraph, block, tail)
	assertRegister(t, rest, tail)
	assertRegister(t, left, leftItem, tail)
	assertRegister(t, right, rightItem, tail)
	assertRegister(t, repeated, p2, p1, tail)
	assertRegister(t, mixed, leftItem, tail, newItem)
	assertRegister(t, restored, tail)
	assertRegister(t, drained)
	assertRegister(t, reused, newItem)
}

func TestOpaqueMaterial(t *testing.T) {
	p := &opaqueMaterial{testMaterial: 1, payload: "prepared"}
	var typedNil *opaqueMaterial
	items := []any{
		nil, typedNil, p,
		opaqueMaterial{testMaterial: 1, payload: []int{1, 2}}, // Same ID, distinct material.
		opaqueMaterial{testMaterial: 3, payload: map[string]int{"block": 3}},
	}
	for _, operation := range []string{"append", "prepend"} {
		t.Run(operation, func(t *testing.T) {
			var r registers.Register[any]
			var bank registers.RegisterBank[any]
			for i := range items {
				if operation == "prepend" {
					item := items[len(items)-1-i]
					r = r.Prepend(item)
					bank = bank.Prepend("", item)
				} else {
					r = r.Append(items[i])
					bank = bank.Append("", items[i])
				}
			}
			assertRegister(t, r, items...)
			for i, expected := range items {
				first, ok := bank.First("")
				if !ok || !reflect.DeepEqual(first, expected) {
					t.Fatalf("First at %d = (%#v, %v), want %#v", i, first, ok, expected)
				}
				item, rest, ok := bank.Take("")
				if !ok || !reflect.DeepEqual(item, expected) {
					t.Fatalf("Take at %d = (%#v, _, %v), want %#v", i, item, ok, expected)
				}
				registerItem, remaining, _ := r.Take()
				if i == 2 && (item != p || registerItem != p) {
					t.Fatal("payload pointer identity was not preserved")
				}
				r, bank = remaining, rest
			}
			if bank.Len() != 0 {
				t.Fatal("drained bank is not empty")
			}
		})
	}
}

// Exercise concrete, unconstrained Ts as well as the interface-valued payloads
// above. Zero payloads must remain distinguishable from missing items.
func checkConcretePayloads[T any](t *testing.T, items ...T) {
	t.Helper()
	for _, prepend := range []bool{false, true} {
		var r registers.Register[T]
		var b registers.RegisterBank[T]
		for i := range items {
			if prepend {
				r = r.Prepend(items[len(items)-1-i])
				b = b.Prepend("", items[len(items)-1-i])
			} else {
				r = r.Append(items[i])
				b = b.Append("", items[i])
			}
		}
		assertRegister(t, r, items...)
		assertBank(t, b, map[registers.RegisterID][]T{"": items})
		original := b
		for _, want := range items {
			first, ok := b.First("")
			if !ok || !reflect.DeepEqual(first, want) {
				t.Fatalf("stored payload lost: %#v %v", first, ok)
			}
			item, next, ok := b.Take("")
			if !ok || !reflect.DeepEqual(item, want) {
				t.Fatalf("stored payload not taken: %#v %v", item, ok)
			}
			b = next
		}
		assertBank(t, b, nil)
		if _, ok := b.Lookup(""); ok {
			t.Fatal("drained name retained")
		}
		assertBank(t, original, map[registers.RegisterID][]T{"": items})
	}
}

func TestConcreteGenericPayloads(t *testing.T) {
	t.Run("integer", func(t *testing.T) { checkConcretePayloads(t, 0, 7, 0, 7) })
	t.Run("string", func(t *testing.T) { checkConcretePayloads(t, "", "body", "", "body") })
	t.Run("value", func(t *testing.T) {
		type value struct{ Tree, Box uint64 }
		checkConcretePayloads(t, value{}, value{1, 7}, value{2, 7}, value{1, 7})
	})
	t.Run("slice", func(t *testing.T) { checkConcretePayloads(t, []int(nil), []int{}, []int{1, 2}, []int(nil)) })
	t.Run("map", func(t *testing.T) {
		checkConcretePayloads(t, map[string]int(nil), map[string]int{}, map[string]int{"x": 1})
	})
	t.Run("noncomparable-struct", func(t *testing.T) {
		type value struct {
			Words   []string
			Indices map[string]int
		}
		checkConcretePayloads(t, value{}, value{[]string{"a"}, map[string]int{"a": 0}})
	})
	t.Run("pointer", func(t *testing.T) {
		type value struct{ Text string }
		var nilValue *value
		p := &value{Text: "shared immutable data"}
		checkConcretePayloads(t, nilValue, p, nilValue)
		b := registers.RegisterBank[*value]{}.Append("body", p)
		got, _, ok := b.Take("body")
		if !ok || got != p {
			t.Fatal("payload reference was cloned")
		}
	})
}
