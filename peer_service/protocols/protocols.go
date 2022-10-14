package protocols

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

var protocolFormat = "%s %d %d"

const (
	CommonChannel    = 0
	ValidatorChannel = 1
)

type Protocols struct {
	networkName       string
	version           int32
	minPeerVersion    int32
}

func New(networkName string, version int32, minimalSupportedVersion int32) *Protocols {
	protocols := mew(Protocols)
}

func GetProtcol
