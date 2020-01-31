package runk

import (
	"github.com/pkg/errors"
	"gvisor.dev/gvisor/pkg/sentry/inet"
	"gvisor.dev/gvisor/pkg/sentry/socket/hostinet"
	"gvisor.dev/gvisor/pkg/sentry/socket/netstack"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/arp"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv6"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"

	_ "gvisor.dev/gvisor/pkg/sentry/socket/netlink"
	_ "gvisor.dev/gvisor/pkg/sentry/socket/netlink/route"
	_ "gvisor.dev/gvisor/pkg/sentry/socket/unix"
)

func netStack(clock tcpip.Clock, network Network) (inet.Stack, error) {
	if network == NetHost {
		return hostinet.NewStack(), nil
	}
	netProtos := []string{ipv4.ProtocolName, ipv6.ProtocolName, arp.ProtocolName}
	protoNames := []string{tcp.ProtocolName, udp.ProtocolName, icmp.ProtocolName4}
	s := netstack.Stack{stack.New(netProtos, protoNames, stack.Options{
		Clock:       clock,
		Stats:       netstack.Metrics,
		HandleLocal: true,
		Raw:         true,
	})}
	if err := s.Stack.SetTransportProtocolOption(tcp.ProtocolNumber, tcp.SACKEnabled(true)); err != nil {
		return nil, errors.Errorf("failed to enable SACK: %v", err)
	}
	return &s, nil
}
