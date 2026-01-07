package dispatch

type Yes struct {
	Line []byte
}

func (y Yes) Read(p []byte) (int, error) {
	if len(p) < len(y.Line) {
		copy(p, y.Line[:len(p)])
		return len(p), nil
	}
	copy(p, y.Line)
	return len(y.Line), nil
}
