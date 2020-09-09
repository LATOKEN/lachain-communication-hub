package config

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/utils"
)

const RelayAddr = "/ip4/95.217.215.141/tcp/41011"
const GRPCPort = ":50001"

var ipLookup = true

func GetBootstrapMultiaddr() ma.Multiaddr {
	relayMultiaddr, err := ma.NewMultiaddr(RelayAddr)
	if err != nil {
		panic(err)
	}

	return relayMultiaddr
}

func GetBootstrapID() peer.ID {
	id, err := peer.Decode("QmaAV3KD9vWhDfrWutZGXy8hMoVU2FtCMirPEPpUPHszAZ")
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
