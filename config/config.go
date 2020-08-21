package config

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

const RelayAddr = "/ip4/95.217.215.141/tcp/37645"

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
