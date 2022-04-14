package config

import (
	"lachain-communication-hub/utils"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

var RelayAddrs []string
var RelayIds []string

var RelayAddrsVal []string
var RelayIdsVal []string

var ipLookup = true

var ChainId = byte(0)
var lock = sync.Mutex{}

func SetBootstrapAddress(addressesString string, peerType string) {
	lock.Lock()
	defer lock.Unlock()

	if len(addressesString) == 0 {
		return
	}
	var addresses = strings.Split(addressesString, ",")
	for _, address := range addresses {
		var parts = strings.Split(address, "@")
		if len(parts) != 2 {
			panic("cannot parse address: " + address)
		}
		var ipParts = strings.Split(parts[1], ":")
		if len(ipParts) != 2 {
			panic("cannot parse address: " + address)
		}
		if peerType == "Validator" {
			RelayAddrsVal = append(RelayAddrsVal, "/ip4/"+ipParts[0]+"/tcp/"+ipParts[1])
			RelayIdsVal = append(RelayIdsVal, parts[0])
		} else {
			RelayAddrs = append(RelayAddrs, "/ip4/"+ipParts[0]+"/tcp/"+ipParts[1])
			RelayIds = append(RelayIds, parts[0])
		}
	}
}

func GetBootstrapMultiaddrs(peerType string) []ma.Multiaddr {
	lock.Lock()
	defer lock.Unlock()

	var multiAddrs []ma.Multiaddr

	if peerType == "Validator" {
		for _, addr := range RelayAddrsVal {
			relayMultiaddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				panic(err)
			}
			multiAddrs = append(multiAddrs, relayMultiaddr)
		}
	} else {
		for _, addr := range RelayAddrs {
			relayMultiaddr, err := ma.NewMultiaddr(addr)
			if err != nil {
				panic(err)
			}
			multiAddrs = append(multiAddrs, relayMultiaddr)
		}
	}

	return multiAddrs
}

func GetBootstrapIDs(peerType string) []peer.ID {
	lock.Lock()
	defer lock.Unlock()

	var ids []peer.ID
	if peerType == "Validator" {
		for _, relayId := range RelayIdsVal {
			id, err := peer.Decode(relayId)
			if err != nil {
				panic(err)
			}
			ids = append(ids, id)
		}
	} else {
		for _, relayId := range RelayIds {
			id, err := peer.Decode(relayId)
			if err != nil {
				panic(err)
			}
			ids = append(ids, id)
		}
	}

	return ids
}

func GetBootstrapIDAddresses(peerID peer.ID, peerType string) []ma.Multiaddr {
	lock.Lock()
	defer lock.Unlock()

	var multiAddrs []ma.Multiaddr

	if peerType == "Validator" {
		for i, relayId := range RelayIdsVal {
			id, err := peer.Decode(relayId)
			if err != nil {
				panic(err)
			}
			if id == peerID {
				relayMultiaddrVal, err := ma.NewMultiaddr(RelayAddrsVal[i])
				if err != nil {
					panic(err)
				}
				multiAddrs = append(multiAddrs, relayMultiaddrVal)
			}
		}
	} else {
		for i, relayId := range RelayIds {
			id, err := peer.Decode(relayId)
			if err != nil {
				panic(err)
			}
			if id == peerID {
				relayMultiaddr, err := ma.NewMultiaddr(RelayAddrs[i])
				if err != nil {
					panic(err)
				}
				multiAddrs = append(multiAddrs, relayMultiaddr)
			}
		}
	}

	return multiAddrs
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
