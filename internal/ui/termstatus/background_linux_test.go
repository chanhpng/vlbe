package termstatus

import (
	"os"
	"testing"

	rtest "github.com/chanhpng/vlbe/internal/test"
)

func TestIsProcessBackground(t *testing.T) {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		t.Skipf("can't open terminal: %v", err)
	}

	_, err = isProcessBackground(tty.Fd())
	rtest.OK(t, err)

	_ = tty.Close()
}
