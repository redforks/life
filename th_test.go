package life

import . "github.com/onsi/gomega"

var (
	slog string
)

func appendLog(msg string) {
	slog += msg + "\n"
}

func assertLog(expected string) {
	Ω(slog).Should(Equal(expected))
	slog = ""
}

func newLogFunc(msg string) func() {
	return func() {
		appendLog(msg)
	}
}
