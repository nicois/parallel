package dispatch

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"strings"
)

func CsvGenerator(ctx context.Context, cancel context.CancelCauseFunc, in io.Reader) iter.Seq[RenderArgs] {
	r := csv.NewReader(in)
	return func(yield func(RenderArgs) bool) {
		header, err := r.Read()
		if err != nil {
			logger.Error("oops", slog.Any("error", err))
			cancel(fmt.Errorf("could not parse the header line of what should be a CSV file: %w", err))
			return
		}
		lineNumber := 0
		for {
			lineNumber += 1
			record, err := r.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				// just log the problem and continue
				logger.Warn("could not parse a line", slog.Any("error", err))
				continue
			}
			if len(record) != len(header) {
				logger.Warn("Unexpected number of columns",
					slog.Int("line number", lineNumber),
					slog.Int("header size", len(header)),
					slog.Int("record size", len(record)))
			}
			result := make(map[string]string)
			for i, h := range header {
				result[strings.TrimSpace(h)] = strings.TrimSpace(record[i])
			}
			if !yield(result) {
				return
			}
		}
	}
}
