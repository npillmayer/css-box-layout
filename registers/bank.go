package registers

import "maps"

// RegisterID names a flow within a bank. Every string, including "", is valid.
type RegisterID string

// RegisterBank is an immutable collection of named, nonempty registers.
// Its zero value is empty. Missing names behave as empty registers; taking the
// last item removes the name from the returned bank.
//
// Updates copy the map, sharing unchanged registers and payloads. This is intended
// for a small number of named flows: Append and Prepend cost O(number of registers
// + remaining queue length) in time and space, and a successful Take costs
// O(number of registers).
type RegisterBank[T any] struct {
	registers map[RegisterID]Register[T]
}

// Len returns the number of nonempty registers, not the total number of items.
func (b RegisterBank[T]) Len() int {
	return len(b.registers)
}

// Lookup returns the register named id. If absent, it returns an empty register
// and false. The returned register can be used independently of the bank;
// operations on it do not update the bank.
func (b RegisterBank[T]) Lookup(id RegisterID) (Register[T], bool) {
	r, ok := b.registers[id]
	return r, ok
}

// First returns the first item in the named register without consuming it,
// or (zero T, false) if the name is absent.
func (b RegisterBank[T]) First(id RegisterID) (T, bool) {
	return b.registers[id].First()
}

// Append returns a bank with item appended to the named register, creating it
// if absent. The receiver and all its registers remain unchanged.
func (b RegisterBank[T]) Append(id RegisterID, item T) RegisterBank[T] {
	regs := maps.Clone(b.registers)
	if regs == nil {
		regs = make(map[RegisterID]Register[T])
	}
	regs[id] = b.registers[id].Append(item)
	return RegisterBank[T]{registers: regs}
}

// Prepend returns a bank with item before all remaining items in the named
// register, creating it if absent. The receiver and all its registers remain
// unchanged. Len increases only when the name was absent.
func (b RegisterBank[T]) Prepend(id RegisterID, item T) RegisterBank[T] {
	regs := maps.Clone(b.registers)
	if regs == nil {
		regs = make(map[RegisterID]Register[T])
	}
	regs[id] = b.registers[id].Prepend(item)
	return RegisterBank[T]{registers: regs}
}

// Take returns the first item in the named register and the remaining bank,
// leaving b unchanged. If the name is absent, it returns (zero T, b, false).
func (b RegisterBank[T]) Take(id RegisterID) (T, RegisterBank[T], bool) {
	item, rest, ok := b.registers[id].Take()
	if !ok {
		return item, b, false
	}
	if rest.Len() == 0 && b.Len() == 1 {
		return item, RegisterBank[T]{}, true
	}
	regs := maps.Clone(b.registers)
	if rest.Len() == 0 {
		delete(regs, id)
	} else {
		regs[id] = rest
	}
	return item, RegisterBank[T]{registers: regs}, true
}
