package runk

import (
	"os"

	"github.com/pkg/errors"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/kvm"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/ptrace"
)

func newPlatform() (platform.Platform, error) {
	f, err := os.Open("/dev/kvm")
	if err == nil {
		p, err := kvm.New(f)
		if err == nil {
			return p, nil
		}
	}

	p, err := ptrace.New()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	return p, nil
}
