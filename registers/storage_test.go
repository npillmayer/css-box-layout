package registers

import "testing"

// The persistent representation deliberately shares unmodified storage while
// copying only updates. These assertions also guard against retaining drained
// queues or copying consumed prefixes into fresh queues after generic conversion.
func TestPersistentStorage(t *testing.T) {
	base := Register[int]{}.Append(1).Append(2).Append(3)
	_, rest, _ := base.Take()
	if &base.queue[0] != &rest.queue[0] || base.head != 0 || rest.head != 1 {
		t.Fatal("Take did not preserve shared read-only storage")
	}
	for _, next := range []Register[int]{rest.Append(4), rest.Prepend(4)} {
		if len(next.queue) != 3 || next.head != 0 || &next.queue[0] == &base.queue[0] {
			t.Fatal("update reused old storage or retained consumed prefix")
		}
	}
	if base.queue[0] != 1 || base.queue[1] != 2 || base.queue[2] != 3 {
		t.Fatal("update mutated earlier queue")
	}
	_, tail, _ := rest.Take()
	_, empty, _ := tail.Take()
	if empty.queue != nil || empty.head != 0 {
		t.Fatal("drained queue retains storage")
	}
	_, stillEmpty, ok := empty.Take()
	if ok || stillEmpty.queue != nil || stillEmpty.head != 0 {
		t.Fatal("empty Take changed zero state")
	}

	bank := RegisterBank[int]{}.Append("body", 1).Append("body", 2).Append("notes", 3)
	derived := bank.Append("body", 4)
	originalNotes := bank.registers["notes"]
	derivedNotes := derived.registers["notes"]
	if &originalNotes.queue[0] != &derivedNotes.queue[0] {
		t.Fatal("unchanged named register was copied")
	}
	for _, id := range []RegisterID{"body", "body", "notes"} {
		_, bank, _ = bank.Take(id)
	}
	if bank.registers != nil {
		t.Fatal("drained bank retains map/storage")
	}
	if derived.Len() != 2 {
		t.Fatal("draining altered other version")
	}
}
