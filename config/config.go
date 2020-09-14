package config

import (
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"lachain-communication-hub/utils"
	"strings"
)

var RelayAddr = "/ip4/95.217.215.141/tcp/41011"
var RelayId = "QmaAV3KD9vWhDfrWutZGXy8hMoVU2FtCMirPEPpUPHszAZ"
var GRPCPort = ":50001"

var ipLookup = true

func SetBootstrapAddress(address string) {
	var parts = strings.Split(address, "@")
	if len(parts) != 2 {
		panic("cannot parse address: " + address)
	}
	var ipParts = strings.Split(parts[1], ":")
	if len(ipParts) != 2 {
		panic("cannot parse address: " + address)
	}
	RelayAddr = "/ip4/" + ipParts[0] + "/tcp/" + ipParts[1]
	RelayId = parts[0]
}

func GetBootstrapMultiaddr() ma.Multiaddr {
	relayMultiaddr, err := ma.NewMultiaddr(RelayAddr)
	if err != nil {
		panic(err)
	}

	return relayMultiaddr
}

func GetBootstrapID() peer.ID {
	id, err := peer.Decode(RelayId)
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
