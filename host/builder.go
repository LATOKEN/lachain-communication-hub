package host

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	conf "lachain-communication-hub/config"
	"os"
	"strconv"

	"github.com/juju/loggo"
	"github.com/libp2p/go-libp2p"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/config"
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

func BuildNamedHost(priv_key crypto.PrivKey) core.Host {

	prvKeyOpt := func(c *config.Config) error {
		c.PeerKey = priv_key
		return nil
	}

	myId, _ := peer.IDFromPrivateKey(priv_key)

	externalIP := conf.GetP2PExternalIP()
	if externalIP == "" {
		log.Warningf("External IP not defined, Peers might not be able to resolve this node if behind NAT")
	}
	var listenAddrs libp2p.Option

	// set port if we are bootstrap
	for _, addr := range conf.GetBootstrapIDAddresses(myId) {
		listenAddrs = libp2p.ListenAddrs(addr)
	}
	if listenAddrs == nil {
		listenAddrs = libp2p.ListenAddrs()
	}
	host, err := libp2p.New(
		listenAddrs,
		libp2p.EnableRelay(),
		prvKeyOpt,
	)
	if err != nil {
		panic(err)
	}
	return host
}
