package host

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p/config"
	ma "github.com/multiformats/go-multiaddr"
	conf "lachain-communication-hub/config"
	"lachain-communication-hub/types"
	"os"
)

var prvPathPrefix = "./prv"
var log = loggo.GetLogger("builder")

func GetPrivateKeyForHost(postfix string) crypto.PrivKey {
	var prv crypto.PrivKey
	prvPath := prvPathPrefix + postfix + ".txt"
	if _, err := os.Stat(prvPath); os.IsNotExist(err) {
		log.Debugf("Generating private key")
		prv, _, err = crypto.GenerateECDSAKeyPair(rand.Reader)
		if err != nil {
			panic(err)
		}

		prvBytes, err := crypto.MarshalPrivateKey(prv)
		if err != nil {
			panic(err)
		}

		prvHex := hex.EncodeToString(prvBytes)

		f, err := os.Create(prvPath)
		if err != nil {
			panic(err)
		}

		n3, err := f.WriteString(prvHex)
		if err != nil {
			panic(err)
		}
		log.Tracef("wrote %d bytes\n", n3)

		f.Sync()
		f.Close()
	} else {
		f, err := os.Open(prvPath)
		if err != nil {
			panic(err)
		}

		b2 := make([]byte, 250)
		_, err = f.Read(b2)
		if err != nil {
			panic(err)
		}

		prvBytes, err := hex.DecodeString(string(b2))
		if err != nil {
			panic(err)
		}

		prv, err = crypto.UnmarshalPrivateKey(prvBytes)
		if err != nil {
			panic(err)
		}
	}

	return prv
}

func BuildNamedHost(typ int, postfix string) core.Host {

	prvKeyOpt := func(c *config.Config) error {
		c.PeerKey = GetPrivateKeyForHost(postfix)
		return nil
	}

	switch typ {
	case types.Peer:
		externalIP := conf.GetP2PExternalIP()
		if externalIP == "" {
			log.Warningf("External IP not defined, Peers might not be able to resolve this node if behind NAT")
		}
		host, err := libp2p.New(
			context.Background(),
			libp2p.ListenAddrs(),
			prvKeyOpt,
		)
		if err != nil {
			panic(err)
		}
		return host
	case types.Relay:
		addrOpt := func(c *config.Config) error {
			c.ListenAddrs = []ma.Multiaddr{conf.GetRelayMultiaddr()}
			return nil
		}
		host, err := libp2p.New(context.Background(), libp2p.ListenAddrs(conf.GetRelayMultiaddr()), libp2p.EnableRelay(circuit.OptHop), prvKeyOpt, addrOpt)
		if err != nil {
			panic(err)
		}
		return host
	default:
		return nil
	}
}
