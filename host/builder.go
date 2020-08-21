package host

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

func GetPrivateKeyForHost(postfix string) crypto.PrivKey {
	var prv crypto.PrivKey
	prvPath := prvPathPrefix + postfix + ".txt"
	if _, err := os.Stat(prvPath); os.IsNotExist(err) {
		fmt.Println("Generating private key")
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
		fmt.Printf("wrote %d bytes\n", n3)

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
	prv := GetPrivateKeyForHost(postfix)

	prvKeyOpt := func(c *config.Config) error {
		c.PeerKey = prv
		return nil
	}

	switch typ {
	case types.Peer:
		host, err := libp2p.New(context.Background(), libp2p.ListenAddrs(), prvKeyOpt)
		if err != nil {
			panic(err)
		}
		return host
	case types.Relay:
		multiaddr, err := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/37645")
		addrOpt := func(c *config.Config) error {
			c.ListenAddrs = []ma.Multiaddr{multiaddr}
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
