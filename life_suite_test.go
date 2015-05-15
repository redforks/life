package life

import (
	. "github.com/onsi/ginkgo"

	"testing"
)

var t = GinkgoT

func TestLife(t *testing.T) {
	RunSpecs(t, "Life Suite")
}
