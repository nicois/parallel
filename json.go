package dispatch

import (
	"context"
	"encoding/json"
	"io"
	"iter"
)

func JsonLineGenerator(ctx context.Context, cancel context.CancelCauseFunc, in io.Reader) iter.Seq[RenderArgs] {
	return func(yield func(RenderArgs) bool) {
		for text := range LineReader(in, cancel) {
			result := make(map[string]string)
			err := json.Unmarshal([]byte(text), &result)
			if err != nil {
				// maybe we should just log the problem and continue?
				if cancel != nil {
					cancel(err)
				}
				return
			}
			if !yield(result) {
				return
			}
		}
	}
}
