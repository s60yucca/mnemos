//go:build windows

package autopilot

type daemonLeader struct{}

func acquireDaemonLeadership(dataDir string) (*daemonLeader, bool, error) {
	return &daemonLeader{}, true, nil
}

func (l *daemonLeader) release() {}
