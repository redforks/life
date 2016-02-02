package life

import "github.com/stretchr/testify/assert"

var (
	slog string
)

func appendLog(msg string) {
	slog += msg + "\n"
}

func assertLog(expected string) {
	assert.Equal(t(), expected, slog)
	slog = ""
}

func newLogFunc(msg string) func() {
	return func() {
		appendLog(msg)
	}
}
