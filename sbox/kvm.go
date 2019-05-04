// +build kvm

package sbox

import (
	"os"

	"gvisor.googlesource.com/gvisor/pkg/sentry/platform"
	"gvisor.googlesource.com/gvisor/pkg/sentry/platform/kvm"
)

func newPlatform() (platform.Platform, error) {
	f, err := os.Open("/dev/kvm")
	if err != nil {
		return nil, err
	}
	p, err := kvm.New(f)
	if err != nil {
		return nil, err
	}
	return p, nil
}
