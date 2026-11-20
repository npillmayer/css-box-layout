package registers

// Register is an immutable FIFO queue of flow material. Its zero value is empty.
// Append copies the remaining queue in O(n) time; First, Take and Len take O(1).
// A partially consumed register shares its backing storage with earlier versions,
// retaining the consumed prefix until the register is emptied or appended to.
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
