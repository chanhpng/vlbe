//go:build darwin || freebsd || linux || solaris
// +build darwin freebsd linux solaris

package restic

import (
	"os"
	"testing"

	rtest "github.com/chanhpng/vlbe/internal/test"
	"github.com/pkg/xattr"
)

func TestIsListxattrPermissionError(t *testing.T) {
	xerr := &xattr.Error{
		Op:   "xattr.list",
		Name: "test",
		Err:  os.ErrPermission,
	}
	err := handleXattrErr(xerr)
	rtest.Assert(t, err != nil, "missing error")
	rtest.Assert(t, IsListxattrPermissionError(err), "expected IsListxattrPermissionError to return true for %v", err)

	xerr.Err = os.ErrNotExist
	err = handleXattrErr(xerr)
	rtest.Assert(t, err != nil, "missing error")
	rtest.Assert(t, !IsListxattrPermissionError(err), "expected IsListxattrPermissionError to return false for %v", err)
}
