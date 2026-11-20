package registers_test

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/npillmayer/css-box-layout/registers"
)

func assertBank(t *testing.T, bank registers.RegisterBank, want map[registers.RegisterID][]registers.FlowMaterial) {
	t.Helper()
	if bank.Len() != len(want) {
		t.Fatalf("bank Len() = %d, want %d", bank.Len(), len(want))
	}
	for id, items := range want {
		r, ok := bank.Lookup(id)
		if !ok {
			t.Fatalf("Lookup(%q) missing", id)
		}
		assertRegister(t, r, items...)
	}
	if r, ok := bank.Lookup("missing"); ok || r.Len() != 0 {
		t.Fatal("absent register must return empty, false")
	}
	if item, ok := bank.First("missing"); ok || item != nil {
		t.Fatal("absent First must return nil, false")
	}
	if item, rest, ok := bank.Take("missing"); ok || item != nil || rest.Len() != bank.Len() {
		t.Fatal("absent Take must return nil, unchanged bank, false")
	}
}

func TestBankVersions(t *testing.T) {
	var empty registers.RegisterBank
	base := empty.Append("body", p1).Append("body", p2).Append("notes", n1)
	item, trial, ok := base.Take("body")
	if !ok || item != p1 {
		t.Fatalf("Take(body) = (%v, _, %v)", item, ok)
	}
	item, drainedNotes, ok := trial.Take("notes")
	if !ok || item != n1 {
		t.Fatalf("Take(notes) = (%v, _, %v)", item, ok)
	}
	if _, ok := drainedNotes.Lookup("notes"); ok {
		t.Fatal("drained name still present")
	}
	_, drained, _ := drainedNotes.Take("body")
	reused := drained.Append("body", newItem)
	branch := base.Append("body", p3).Append("other", o1)
	lookup, _ := base.Lookup("body")
	derived := lookup.Append(local)
	_, unchanged, _ := base.Take("missing")

	assertBank(t, empty, nil)
	assertBank(t, base, map[registers.RegisterID][]registers.FlowMaterial{"body": {p1, p2}, "notes": {n1}})
	assertBank(t, trial, map[registers.RegisterID][]registers.FlowMaterial{"body": {p2}, "notes": {n1}})
	assertBank(t, drainedNotes, map[registers.RegisterID][]registers.FlowMaterial{"body": {p2}})
	assertBank(t, drained, nil)
	assertBank(t, reused, map[registers.RegisterID][]registers.FlowMaterial{"body": {newItem}})
	assertBank(t, branch, map[registers.RegisterID][]registers.FlowMaterial{"body": {p1, p2, p3}, "notes": {n1}, "other": {o1}})
	assertBank(t, unchanged, map[registers.RegisterID][]registers.FlowMaterial{"body": {p1, p2}, "notes": {n1}})
	assertRegister(t, derived, p1, p2, local)
}

// Branch from arbitrary checkpoints and compare every retained version with an
// independent FIFO model. This exercises interleaved appends, takes and rollbacks.
func TestBankCheckpointHistory(t *testing.T) {
	type snapshot struct {
		bank  registers.RegisterBank
		model map[registers.RegisterID][]registers.FlowMaterial
	}
	history := []snapshot{{}}
	rng := rand.New(rand.NewPCG(13, 29))
	ids := []registers.RegisterID{"", "body", "notes"}
	for step := 0; step < 200; step++ {
		prior := history[rng.IntN(len(history))]
		model := make(map[registers.RegisterID][]registers.FlowMaterial)
		for id, items := range prior.model {
			model[id] = append([]registers.FlowMaterial(nil), items...)
		}
		id := ids[rng.IntN(len(ids))]
		var next registers.RegisterBank
		if rng.IntN(2) == 0 {
			next = prior.bank.Append(id, testMaterial(step+100))
			model[id] = append(model[id], testMaterial(step+100))
		} else {
			item, rest, ok := prior.bank.Take(id)
			next = rest
			items := model[id]
			if len(items) == 0 {
				if ok || item != nil {
					t.Fatal("Take from empty model succeeded")
				}
			} else {
				if !ok || item != items[0] {
					t.Fatalf("Take = (%v, _, %v), want %v", item, ok, items[0])
				}
				if len(items) == 1 {
					delete(model, id)
				} else {
					model[id] = items[1:]
				}
			}
		}
		history = append(history, snapshot{next, model})
		for _, version := range history {
			assertBank(t, version.bank, version.model)
		}
	}
}

func TestConcurrentBranches(t *testing.T) {
	base := registers.RegisterBank{}.Append("body", paragraph).Append("body", block).Append("notes", note)
	r, _ := base.Lookup("body")
	var wg sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 50; iteration++ {
				branch := base.Append("body", testMaterial(worker+100)).Append("local", testMaterial(iteration+200))
				item, branch, ok := branch.Take("body")
				if !ok || item != paragraph {
					t.Error("concurrent branch lost its first item")
					return
				}
				_, branch, _ = branch.Take("notes")
				assertBank(t, branch, map[registers.RegisterID][]registers.FlowMaterial{"body": {block, testMaterial(worker + 100)}, "local": {testMaterial(iteration + 200)}})
				_, tail, _ := r.Take()
				assertRegister(t, tail.Append(testMaterial(worker+100)), block, testMaterial(worker+100))
				assertRegister(t, r, paragraph, block)
			}
		}()
	}
	wg.Wait()
	assertBank(t, base, map[registers.RegisterID][]registers.FlowMaterial{"body": {paragraph, block}, "notes": {note}})
}
