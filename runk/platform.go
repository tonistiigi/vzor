package runk

import (
	"os"

	"github.com/pkg/errors"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/kvm"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/ptrace"
)

func newPlatform(gvp GVisorPlatform) (platform.Platform, error) {
	if gvp == "" {
		gvp = Ptrace
		if _, err := os.Stat("/dev/kvm"); err == nil {
			gvp = KVM
		}
	}

	switch gvp {
	case KVM:
		f, err := os.Open("/dev/kvm")
		if err != nil {
			return nil, errors.WithStack(err)
		}
		return kvm.New(f)
	case Ptrace:
		return ptrace.New()
	default:
		return nil, errors.Errorf("could not set up gVisor platform (platform=%q)", gvp)
	}
}
