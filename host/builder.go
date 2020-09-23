package host

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p"
	circuit "github.com/libp2p/go-libp2p-circuit"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
	conf "lachain-communication-hub/config"
	"lachain-communication-hub/types"
	"os"
	"strconv"
)

var prvPathPrefix = "./ChainLachain/prv"
var log = loggo.GetLogger("builder")

func GetPrivateKeyForHost(postfix string) crypto.PrivKey {
	var prv crypto.PrivKey
	prvPath := prvPathPrefix + postfix + ".txt"
	if _, err := os.Stat(prvPath); os.IsNotExist(err) {
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

		_, err = f.WriteString(prvHex)
		if err != nil {
			panic(err)
		}

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

func GenerateKey(count int) {
	prvPathPrefix = "./prv_"
	for i := 1; i <= count; i++ {
		myId, _ := peer.IDFromPrivateKey(GetPrivateKeyForHost("h" + strconv.Itoa(i)))
		fmt.Println(strconv.Itoa(i) + ". " + myId.Pretty())
	}
}

func BuildNamedHost(typ int, postfix string) core.Host {

	prvKeyOpt := func(c *config.Config) error {
		c.PeerKey = GetPrivateKeyForHost(postfix)
		return nil
	}

	myId, _ := peer.IDFromPrivateKey(GetPrivateKeyForHost(postfix))

	switch typ {
	case types.Peer:
		externalIP := conf.GetP2PExternalIP()
		if externalIP == "" {
			log.Warningf("External IP not defined, Peers might not be able to resolve this node if behind NAT")
		}
		var listenAddrs libp2p.Option

		// set port if we are bootstrap
		for i, id := range conf.GetBootstrapIDs() {
			if id == myId {
				listenAddrs = libp2p.ListenAddrs(conf.GetBootstrapMultiaddrs()[i])
			}
		}
		if listenAddrs == nil {
			listenAddrs = libp2p.ListenAddrs()
		}
		host, err := libp2p.New(
			context.Background(),
			listenAddrs,
			libp2p.EnableRelay(circuit.OptHop),
			prvKeyOpt,
		)
		if err != nil {
			panic(err)
		}
		return host
	default:
		return nil
	}
}
