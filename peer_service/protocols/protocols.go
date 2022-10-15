package protocols

import (
	"errors"
	"fmt"

	"github.com/juju/loggo"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
)

var ProtocolFormat = "%s %d %d"

const (
	CommonChannel    = 0
	ValidatorChannel = 1
)

type Protocols struct {
	networkName    string
	version        int32
	minPeerVersion int32
}

var log = loggo.GetLogger("peer_service")

func New(networkName string, version int32, minimalSupportedVersion int32) *Protocols {
	protocols := new(Protocols)
	protocols.networkName = networkName
	protocols.version = version
	protocols.minPeerVersion = minimalSupportedVersion
	return protocols
}

func (protcols *Protocols) getCommonProtcol() string {
	return fmt.Sprintf(ProtocolFormat, protcols.networkName, protcols.version, CommonChannel)
}

func (protcols *Protocols) getValidatorProtocol() string {
	return fmt.Sprintf(ProtocolFormat, protcols.networkName, protcols.version, ValidatorChannel)
}

func (protcols *Protocols) getProtocol(protocolType int32) (string, error) {
	switch (protocolType) {
	case CommonChannel:
		return protcols.getCommonProtcol(), nil
	case ValidatorChannel:
		return protcols.getValidatorProtocol(), nil
	default:
		log.Errorf("got unregistered protocol type %d", protocolType)
		return "", errors.New("unregistered protocol type")
	}
}

func (protocols *Protocols) getProtocolType(protocol string) (int32, error) {
	var protocolType int32
	var network string
	var version int32
	_, err := fmt.Sscanf(protocol, ProtocolFormat, &network, &version, &protocolType)
	if (err != nil) {
		return -1, err
	}

	switch (protocolType) {
	case CommonChannel:
		return CommonChannel, nil
	case ValidatorChannel:
		return ValidatorChannel, nil
	default:
		log.Errorf("Got unregistered protocol type %d", protocolType)
		return -1, errors.New("unregistered protocol type")
	}
}

func (protocols *Protocols) SetStreamHandlerMatch(
	host *core.Host,
	onConnect func(network.Stream),
) {
	// for now we don't have any complex protocol
	// for any protocol, we need to create a channel with the corresponding protocol

	// setting common channel

	var protocolString string
	protocolString = protocols.getCommonProtcol()
	(*host).SetStreamHandlerMatch(protocol.ID(protocolString), protocols.networkMatcher, onConnect)

	// setting validator channel

	protocolString = protocols.getValidatorProtocol()
	(*host).SetStreamHandlerMatch(protocol.ID(protocolString), protocols.networkMatcher, onConnect)
}

// common network matcher for all protocols
// because we don't have any complex protocol for now
func (protocols *Protocols) networkMatcher(protocol string) bool {
	var network string
	var version int32
	var protocolType int32
	_, err := fmt.Sscanf(protocol, ProtocolFormat, &network, &version, &protocolType)
	if err != nil {
		return false
	}
	if network != protocols.networkName {
		return false
	}
	if version < protocols.minPeerVersion {
		return false
	}

	switch (protocolType) {
	case CommonChannel:
		return true
	case ValidatorChannel:
		return true
	default:
		return false
	}
}
