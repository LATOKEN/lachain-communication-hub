package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	p2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	p2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"os"
)

type Config struct {
	PrivateKey string `json: "private_key"`
	Idx        uint32 `json: "index"`
	Port       uint32 `json: "port"`
	Bootstraps string `json: "bootstraps"`
}


func main() {
	var peerNumber uint
	flag.UintVar(&peerNumber, "peerNumber", 4, "Specify number of peers in test,  default value is 4", )
	flag.Parse()

	privateKeys := make([]p2p_crypto.PrivKey, peerNumber, peerNumber)
	for i := uint(0); i < peerNumber; i++ {
		privateKeys[i], _, _ = p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	}

	peerList := ""
	nodePort := uint32(7070)
	for i := uint(0); i < peerNumber; i++ {
		id, _ := p2p_peer.IDFromPrivateKey(privateKeys[i])
		bootstrapAddress := fmt.Sprintf("%s@127.0.0.1:%d", p2p_peer.Encode(id), nodePort)
		peerList += bootstrapAddress
		if i != peerNumber -1 {
			peerList += ","
		}
		nodePort++
	}

	nodePort = 7070
	for i := uint(0); i < peerNumber; i++ {
		prvBytes, err := p2p_crypto.MarshalPrivateKey(privateKeys[i])
		if err != nil {
			panic(err)
		}

		data := Config{
			PrivateKey: hex.EncodeToString(prvBytes),
			Idx:        uint32(i),
			Port:       nodePort,
			Bootstraps: peerList,
		}

		databin,  err := json.Marshal(data)
		if err != nil {
			panic(err)
		}

		filename := fmt.Sprintf("testhubconfig%d.json", i)
		f, err := os.Create(filename)
		if err != nil {
			panic(err)
		}

		f.Write(databin)
		f.Sync()
		f.Close()

		nodePort++
	}

	f, err := os.Create("./starthubs.sh")
	if err != nil {
		panic(err)
	}
	for i := uint(0); i < peerNumber; i++ {
		line := fmt.Sprintf("./testhub -config ./testhubconfig%d.json &", i)
		f.WriteString(line)
	}
	f.Sync()
	f.Close()

}
