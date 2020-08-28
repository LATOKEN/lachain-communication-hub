package config

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/utils"
)

const RelayAddr = "/ip4/95.217.215.141/tcp/37646"
const GRPCPort = ":50001"

var ipLookup = true

func GetRelayMultiaddr() ma.Multiaddr {
	relayMultiaddr, err := ma.NewMultiaddr(RelayAddr)
	if err != nil {
		panic(err)
	}

	return relayMultiaddr
}

func GetRelayID() peer.ID {
	id, err := peer.Decode("QmUodkW8hRWayaRTBb12bfosNvPTq5gQhAso6M3Xk42SUh")
	if err != nil {
		panic(err)
	}

	return id
}

func DisableIpLookup() {
	ipLookup = false
}

func GetP2PExternalIP() string {
	if !ipLookup {
		return ""
	}
	return utils.IPLookup()
}
