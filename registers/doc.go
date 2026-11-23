// Package registers provides immutable queues for page-layout flow material.
//
// Registers hold material awaiting placement into a sequence of frames or regions,
// such as prepared paragraphs and unset blocks. They store whole items; measuring,
// splitting and placing those items belong to the page-layout consumer, not this
// package or the normal CSS box-layout passes.
//
// Append adds at the back and Take consumes from the front, providing FIFO order.
// Prepend explicitly inserts at the front, for example to return unfinished
// material ahead of the remaining queue. Repeated prepends reverse insertion order.
//
// Register[T] and RegisterBank[T] are persistent values: Append, Prepend and Take return
// new values without changing any earlier version. A layout trial can keep its
// input bank as a checkpoint and publish the returned bank only when the trial succeeds.
// Their zero values are ready to use, and returned values must be retained to
// observe changes. T may be any type; storage neither inspects payloads nor
// requires identity methods. First/Take return the zero T and false when empty.
// A stored zero T (including nil for a nil-capable T) is an ordinary item with
// true; only the boolean distinguishes absence. Duplicate payloads are allowed.
//
// Published values can be read and used to derive independent versions concurrently.
// This does not synchronize assignment to a shared Register or RegisterBank variable;
// callers must coordinate publication of a new current version themselves. Payloads
// are copied as T values, not deep-copied. Referenced data is shared and must
// remain immutable while any version uses it. The generic constraint cannot
// enforce this ownership rule. Register persistence does not make a mutable
// payload or an execution/preparation session safe to share.
package registers
