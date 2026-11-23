package registers_test

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/registers"
)

type exampleMaterial struct {
	id   uint64
	text string
}

func (m exampleMaterial) String() string { return m.text }

func ExampleRegisterBank() {
	var bank registers.RegisterBank[exampleMaterial]
	bank = bank.Append("body", exampleMaterial{1, "prepared paragraph"})
	bank = bank.Append("body", exampleMaterial{2, "unset block"})
	bank = bank.Append("notes", exampleMaterial{3, "footnote"})

	// A trial consumes a whole item without changing its checkpoint.
	checkpoint := bank
	item, trial, ok := bank.Take("body")
	fmt.Println(item, ok)
	first, _ := checkpoint.First("body")
	fmt.Println("checkpoint:", first)

	// Publish the trial after successful placement.
	bank = trial
	first, _ = bank.First("body")
	fmt.Println("remaining:", first)
	first, _ = bank.First("notes")
	fmt.Println("notes:", first)
	// Output:
	// prepared paragraph true
	// checkpoint: prepared paragraph
	// remaining: unset block
	// notes: footnote
}

func ExampleRegister_Prepend() {
	original := registers.Register[exampleMaterial]{}.Append(exampleMaterial{1, "paragraph"})
	updated := original.Prepend(exampleMaterial{2, "introduction"})
	item, rest, _ := updated.Take()
	fmt.Println("prepended:", item)
	first, _ := rest.First()
	fmt.Println("remaining:", first)
	first, _ = original.First()
	fmt.Println("original:", first)
	// Output:
	// prepended: introduction
	// remaining: paragraph
	// original: paragraph
}

func ExampleRegisterBank_Prepend() {
	bank := registers.RegisterBank[exampleMaterial]{}.Append("body", exampleMaterial{1, "paragraph"}).
		Append("body", exampleMaterial{2, "next block"})
	item, trial, _ := bank.Take("body")
	// Restore the unchanged whole item in a derived version. Registers do not
	// split or prepare material, and the original bank remains unchanged.
	trial = trial.Prepend("body", item)
	first, _ := trial.First("body")
	fmt.Println("trial:", first)
	first, _ = bank.First("body")
	fmt.Println("checkpoint:", first)
	_, trial, _ = trial.Take("body")
	first, _ = trial.First("body")
	fmt.Println("then:", first)
	// Output:
	// trial: paragraph
	// checkpoint: paragraph
	// then: next block
}

func ExampleRegister_zeroValue() {
	var r registers.Register[int]
	item, ok := r.First()
	fmt.Println(item, ok) // Empty, not a stored zero.
	r = r.Append(0)
	item, rest, ok := r.Take()
	fmt.Println(item, ok, rest.Len())
	// Output:
	// 0 false
	// 0 true 0
}
