// Package registers provides immutable FIFO registers for page-layout flow material.
//
// Registers hold material awaiting placement into a sequence of frames or regions,
// such as prepared paragraphs and unset blocks. They store whole items; measuring,
// splitting and placing those items belong to the page-layout consumer, not this
// package or the normal CSS box-layout passes.
//
// Register and RegisterBank are persistent values: Append and Take return new
// values without changing any earlier version. A layout trial can keep its input
// bank as a checkpoint and publish the returned bank only when the trial succeeds.
// Their zero values are ready to use, and returned values must be retained to
// observe changes.
//
// Published values can be read and used to derive independent versions concurrently.
// This does not synchronize assignment to a shared Register or RegisterBank variable;
// callers must coordinate publication of a new current version themselves. Payloads
// are shared, not deep-copied, and must remain immutable while any version uses them.
package registers

import "github.com/npillmayer/css-box-layout/layout"

// FlowMaterial is an item of page-layout content identified by its layout box.
// Consumers use their own concrete types or interfaces for further layout work.
// Values may include references to prepared paragraphs or unset blocks, provided
// those references and their reachable data remain read-only while shared.
// Nil (including a typed nil) is a valid item; the bool returned by First and Take
// distinguishes an item from an empty register.
type FlowMaterial interface {
	GetBoxID() layout.BoxID
}
