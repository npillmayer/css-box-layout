package text

type TextPos = uint64
type TextRange struct{ Start, End TextPos }
type TextSourceID uint32

type TextRef struct {
	Source TextSourceID
	Range  TextRange
}

type TextSource interface {
	ID() TextSourceID
	LenBytes() uint64
}
