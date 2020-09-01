package utils

import (
	"github.com/glendc/go-external-ip"
)

func IPLookup() string {
	// Create the default consensus,
	// using the default configuration and no logger.
	consensus := externalip.DefaultConsensus(nil, nil)
	// Get your IP,
	// which is never <nil> when err is <nil>.
	ip, err := consensus.ExternalIP()
	if err != nil {
		panic(err)
	}
	return ip.String()
}
