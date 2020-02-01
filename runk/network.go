package runk

import (
	"fmt"

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
	"gvisor.dev/gvisor/pkg/tcpip/transport/raw"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"

	_ "gvisor.dev/gvisor/pkg/sentry/socket/netlink"
	_ "gvisor.dev/gvisor/pkg/sentry/socket/netlink/route"
	_ "gvisor.dev/gvisor/pkg/sentry/socket/unix"
)

func netStack(clock tcpip.Clock, uniqueID stack.UniqueID, network Network) (inet.Stack, error) {
	if network == NetHost {
		return hostinet.NewStack(), nil
	}
	netProtos := []stack.NetworkProtocol{ipv4.NewProtocol(), ipv6.NewProtocol(), arp.NewProtocol()}
	transProtos := []stack.TransportProtocol{tcp.NewProtocol(), udp.NewProtocol(), icmp.NewProtocol4()}
	s := netstack.Stack{stack.New(stack.Options{
		NetworkProtocols:   netProtos,
		TransportProtocols: transProtos,
		Clock:              clock,
		Stats:              netstack.Metrics,
		HandleLocal:        true,
		// Enable raw sockets for users with sufficient
		// privileges.
		RawFactory: raw.EndpointFactory{},
		UniqueID:   uniqueID,
	})}
	if err := s.Stack.SetTransportProtocolOption(tcp.ProtocolNumber, tcp.SACKEnabled(true)); err != nil {
		return nil, errors.Errorf("failed to enable SACK: %v", err)
	}
	// Set default TTLs as required by socket/netstack.
	s.Stack.SetNetworkProtocolOption(ipv4.ProtocolNumber, tcpip.DefaultTTLOption(netstack.DefaultTTL))
	s.Stack.SetNetworkProtocolOption(ipv6.ProtocolNumber, tcpip.DefaultTTLOption(netstack.DefaultTTL))

	// Enable Receive Buffer Auto-Tuning.
	if err := s.Stack.SetTransportProtocolOption(tcp.ProtocolNumber, tcpip.ModerateReceiveBufferOption(true)); err != nil {
		return nil, fmt.Errorf("SetTransportProtocolOption failed: %v", err)
	}

	s.FillDefaultIPTables()
	return &s, nil
}
