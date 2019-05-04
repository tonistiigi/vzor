// +build !kvm

package sbox

import (
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/ptrace"
)

func newPlatform() (platform.Platform, error) {
	p, err := ptrace.New()
	if err != nil {
		return nil, err
	}
	return p, nil
}
