package life

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var t = GinkgoT

func TestLife(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Life Suite")
}
