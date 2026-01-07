package dispatch

import (
	"bufio"
	"context"
	"io"
	"iter"
	"strings"
)

func LineReader(reader io.Reader, cancel context.CancelCauseFunc) iter.Seq[string] {
	return func(yield func(string) bool) {
		r := bufio.NewReader(reader)
		for {
			text, err := r.ReadString('\n')
			text = strings.TrimRight(text, "\n")
			if err != nil {
				if err == io.EOF {
					return
				}
				if cancel != nil {
					cancel(err)
				}
				return
			}
			if len(text) == 0 {
				continue
			}
			if !yield(text) {
				return
			}
		}
	}
}
