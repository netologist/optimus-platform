package suite_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2ESuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Optimus E2E Suite")
}
