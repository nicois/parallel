package dispatch

import (
	"context"
	"io"
	"iter"
	"strings"
)

func SimpleLineGenerator(ctx context.Context, cancel context.CancelCauseFunc, in io.Reader) iter.Seq[RenderArgs] {
	return func(yield func(RenderArgs) bool) {
		for text := range LineReader(in, cancel) {
			text = strings.TrimSpace(text)
			if !yield(map[string]string{"value": text}) {
				return
			}
		}
	}
}
