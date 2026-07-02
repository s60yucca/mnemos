//go:build !windows

package autopilot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type daemonLeader struct {
	file *os.File
}

func acquireDaemonLeadership(dataDir string) (*daemonLeader, bool, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, false, fmt.Errorf("create data dir: %w", err)
	}
	path := filepath.Join(dataDir, "autopilot.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open autopilot lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("lock autopilot: %w", err)
	}
	if err := file.Truncate(0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, false, fmt.Errorf("truncate autopilot lock: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, false, fmt.Errorf("seek autopilot lock: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
		return nil, false, fmt.Errorf("write autopilot lock: %w", err)
	}
	return &daemonLeader{file: file}, true, nil
}

func (l *daemonLeader) release() {
	if l == nil || l.file == nil {
		return
	}
	_ = syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	_ = l.file.Close()
	l.file = nil
}
