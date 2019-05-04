package runk

import (
	"github.com/pkg/errors"
	"gvisor.googlesource.com/gvisor/pkg/sentry/inet"
	"gvisor.googlesource.com/gvisor/pkg/sentry/socket/epsocket"
	"gvisor.googlesource.com/gvisor/pkg/sentry/socket/hostinet"
	"gvisor.googlesource.com/gvisor/pkg/tcpip"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/network/arp"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/stack"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.googlesource.com/gvisor/pkg/tcpip/transport/udp"

	_ "gvisor.googlesource.com/gvisor/pkg/sentry/socket/netlink"
	_ "gvisor.googlesource.com/gvisor/pkg/sentry/socket/netlink/route"
	_ "gvisor.googlesource.com/gvisor/pkg/sentry/socket/unix"
)

func netStack(clock tcpip.Clock, network Network) (inet.Stack, error) {
	if network == NetHost {
		return hostinet.NewStack(), nil
	}
	netProtos := []string{ipv4.ProtocolName, ipv6.ProtocolName, arp.ProtocolName}
	protoNames := []string{tcp.ProtocolName, udp.ProtocolName, icmp.ProtocolName4}
	s := epsocket.Stack{stack.New(netProtos, protoNames, stack.Options{
		Clock:       clock,
		Stats:       epsocket.Metrics,
		HandleLocal: true,
		Raw:         true,
	})}
	if err := s.Stack.SetTransportProtocolOption(tcp.ProtocolNumber, tcp.SACKEnabled(true)); err != nil {
		return nil, errors.Errorf("failed to enable SACK: %v", err)
	}
	return &s, nil
}
