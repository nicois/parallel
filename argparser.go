package parallel

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"iter"
	"strings"
)

type (
	RenderedCommand []string
	RenderArgs      map[string]string
)

type TemplateArgParser struct {
	command []*template.Template
	reader  io.Reader
}

// Generator will process the incoming data stream, generating rendered commands
// until either it runs out of input or the context is cancelled. If a fatal error
// occurs which prevents continuing to process the data stream, cancel the context and exit.
// Non-fatal errors should return in an empty command being returned (as well as logging the error)
type Generator func(context.Context, context.CancelCauseFunc, io.Reader) iter.Seq[RenderArgs]

func ParseCommandline(command []string) ([]*template.Template, error) {
	result := make([]*template.Template, len(command))
	for i, part := range command {
		if t, err := template.New("ArgParser").Parse(part); err == nil {
			result[i] = t
		} else {
			return nil, err
		}
	}
	return result, nil
}

func Render(command []*template.Template, args RenderArgs) (RenderedCommand, error) {
	result := make([]string, 0, len(command))
	for _, part := range command {
		var sb strings.Builder
		err := part.Execute(&sb, args)
		if err != nil {
			return nil, fmt.Errorf("could not render %v with %q: %w", part, args, err)
		}
		result = append(result, sb.String())
	}
	return result, nil
}
