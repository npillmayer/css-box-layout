package registers

// Register is an immutable queue of flow material with explicit front insertion.
// Its zero value is empty. Append and Prepend copy the remaining queue in O(n)
// time and space; First, Take and Len take O(1).
// A partially consumed register shares its backing storage with earlier versions,
// retaining the consumed prefix until the register is emptied, appended to, or
// prepended to. Earlier versions continue to retain their own storage.
type Register struct {
	// The backing array is never modified after publication.
	queue []FlowMaterial
	head  int
}

// Len returns the number of remaining items.
func (r Register) Len() int {
	return len(r.queue) - r.head
}

// Append returns a register with item added at the end, leaving r unchanged.
func (r Register) Append(item FlowMaterial) Register {
	q := make([]FlowMaterial, r.Len()+1)
	copy(q, r.queue[r.head:])
	q[len(q)-1] = item
	return Register{queue: q}
}

// Prepend returns a register with item before all remaining items, leaving r
// unchanged. Repeated prepends insert in reverse order: Prepend(a).Prepend(b)
// puts b before a. Payloads are shared without inspection or copying.
func (r Register) Prepend(item FlowMaterial) Register {
	q := make([]FlowMaterial, r.Len()+1)
	q[0] = item
	// The consumed prefix may still belong to an earlier version; never reuse it.
	copy(q[1:], r.queue[r.head:])
	return Register{queue: q}
}

// First returns the first item without consuming it, or (nil, false) if empty.
func (r Register) First() (FlowMaterial, bool) {
	if r.Len() == 0 {
		return nil, false
	}
	return r.queue[r.head], true
}

// Take returns the first item and the remaining register, leaving r unchanged.
// If empty, it returns (nil, r, false).
func (r Register) Take() (FlowMaterial, Register, bool) {
	if r.Len() == 0 {
		return nil, r, false
	}
	item := r.queue[r.head]
	if r.Len() == 1 {
		// Do not keep the drained queue's payloads alive through the empty result.
		return item, Register{}, true
	}
	r.head++
	return item, r, true
}
