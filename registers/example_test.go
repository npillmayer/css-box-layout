package registers_test

import (
	"fmt"

	"github.com/npillmayer/css-box-layout/layout"
	"github.com/npillmayer/css-box-layout/registers"
)

type exampleMaterial struct {
	id   layout.BoxID
	text string
}

func (m exampleMaterial) GetBoxID() layout.BoxID { return m.id }
func (m exampleMaterial) String() string         { return m.text }

func ExampleRegisterBank() {
	var bank registers.RegisterBank
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
