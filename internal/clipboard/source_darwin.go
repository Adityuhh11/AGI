package clipboard

// The Mac clipboard (NSPasteboard: its change count, the types programs add to say "keep this private") is milestone M2. Until then
// the history sees an empty clipboard that never changes, so nothing is kept and nothing breaks.

// NewSource is the clipboard as the history sees it.
func NewSource() Source { return unavailableSource{} }

type unavailableSource struct{}

func (unavailableSource) Sequence() uint32    { return 0 }
func (unavailableSource) Read() (Clip, error) { return Clip{}, nil }
