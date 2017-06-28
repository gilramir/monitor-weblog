package collator

import (
	"log"
	"testing"

	. "gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	TestingT(t)
}

type MySuite struct {
	tmpDir string
}

var _ = Suite(&MySuite{})

func (s *MySuite) SetUpSuite(c *C) {
	// Create a temp dir which will be removed automatically
	s.tmpDir = c.MkDir()
}
