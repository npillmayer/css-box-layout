package registers

import "maps"

// RegisterID names a flow within a bank. Every string, including "", is valid.
type RegisterID string

// RegisterBank is an immutable collection of named, nonempty registers.
// Its zero value is empty. Missing names behave as empty registers; taking the
// last item removes the name from the returned bank.
//
// Updates copy the map, sharing unchanged registers and payloads. This is intended
// for a small number of named flows: Append costs O(number of registers + queue
// length), and a successful Take costs O(number of registers).
type RegisterBank struct {
	registers map[RegisterID]Register
}

// Len returns the number of nonempty registers, not the total number of items.
func (b RegisterBank) Len() int {
	return len(b.registers)
}

// Lookup returns the register named id. If absent, it returns an empty register
// and false. The returned register can be used independently of the bank;
// operations on it do not update the bank.
func (b RegisterBank) Lookup(id RegisterID) (Register, bool) {
	r, ok := b.registers[id]
	return r, ok
}

// First returns the first item in the named register without consuming it,
// or (nil, false) if the name is absent.
func (b RegisterBank) First(id RegisterID) (FlowMaterial, bool) {
	return b.registers[id].First()
}

// Append returns a bank with item appended to the named register, creating it
// if absent. The receiver and all its registers remain unchanged.
func (b RegisterBank) Append(id RegisterID, item FlowMaterial) RegisterBank {
	regs := maps.Clone(b.registers)
	if regs == nil {
		regs = make(map[RegisterID]Register)
	}
	regs[id] = b.registers[id].Append(item)
	return RegisterBank{registers: regs}
}

// Take returns the first item in the named register and the remaining bank,
// leaving b unchanged. If the name is absent, it returns (nil, b, false).
func (b RegisterBank) Take(id RegisterID) (FlowMaterial, RegisterBank, bool) {
	item, rest, ok := b.registers[id].Take()
	if !ok {
		return nil, b, false
	}
	if rest.Len() == 0 && b.Len() == 1 {
		return item, RegisterBank{}, true
	}
	regs := maps.Clone(b.registers)
	if rest.Len() == 0 {
		delete(regs, id)
	} else {
		regs[id] = rest
	}
	return item, RegisterBank{registers: regs}, true
}
