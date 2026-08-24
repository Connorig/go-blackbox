package zaplog

import (
	"errors"
	"syscall"
	"testing"
)

// TestIgnorableSyncErrors 验证终端/关闭句柄/管道错误被识别为可忽略。
func TestIgnorableSyncErrorsExtended(t *testing.T) {
	if !isIgnorableSyncError(syscall.EINVAL) {
		t.Fatal("EINVAL must be ignorable")
	}
	if !isIgnorableSyncError(syscall.ENOTTY) {
		t.Fatal("ENOTTY must be ignorable")
	}
	if !isIgnorableSyncError(syscall.EBADF) {
		t.Fatal("EBADF must be ignorable (closed stdout during shutdown)")
	}
	if !isIgnorableSyncError(syscall.EPIPE) {
		t.Fatal("EPIPE must be ignorable (closed pipe)")
	}
	if isIgnorableSyncError(errors.New("disk failure")) {
		t.Fatal("unrelated sync errors must not be ignored")
	}
}
