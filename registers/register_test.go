package registers_test

import (
	"reflect"
	"testing"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
)

// testMaterial carries the identity required by FlowMaterial.
type testMaterial layout.BoxID

func (m testMaterial) GetBoxID() layout.BoxID { return layout.BoxID(m) }

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

var (
	_ registers.FlowMaterial = testMaterial(0)
	_ registers.FlowMaterial = (*opaqueMaterial)(nil)
)

func assertRegister(t *testing.T, r registers.Register, want ...registers.FlowMaterial) {
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
	if first, ok := r.First(); first != nil || ok {
		t.Fatalf("empty First() = (%#v, %v)", first, ok)
	}
	if item, rest, ok := r.Take(); item != nil || ok || rest.Len() != 0 {
		t.Fatalf("empty Take() = (%#v, %#v, %v)", item, rest, ok)
	}
}

func TestRegisterVersions(t *testing.T) {
	var empty registers.Register
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
	var empty registers.Register
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
	items := []registers.FlowMaterial{
		nil, typedNil, p,
		opaqueMaterial{testMaterial: 1, payload: []int{1, 2}}, // Same BoxID, distinct material.
		opaqueMaterial{testMaterial: 3, payload: map[string]int{"block": 3}},
	}
	for _, operation := range []string{"append", "prepend"} {
		t.Run(operation, func(t *testing.T) {
			var r registers.Register
			var bank registers.RegisterBank
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
