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
	"strings"
)

type Config struct {
	PrivateKey string `json: "private_key"`
	Idx        uint32 `json: "index"`
	Port       uint32 `json: "port"`
	Bootstraps string `json: "bootstraps"`
}

func generateLocalConfig(peerNumber uint, port uint) {
	privateKeys := make([]p2p_crypto.PrivKey, peerNumber, peerNumber)
	for i := uint(0); i < peerNumber; i++ {
		privateKeys[i], _, _ = p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	}

	peerList := ""
	nodePort := uint32(port)
	for i := uint(0); i < peerNumber; i++ {
		id, _ := p2p_peer.IDFromPrivateKey(privateKeys[i])
		bootstrapAddress := fmt.Sprintf("%s@127.0.0.1:%d", p2p_peer.Encode(id), nodePort)
		peerList += bootstrapAddress
		if i != peerNumber -1 {
			peerList += ","
		}
		nodePort++
	}

	nodePort = uint32(port)
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

func generateRemoteConfig(ips string, port uint, sshkeypath string) {
	ipList := strings.Split(ips, ",")
	peerNumber := len(ipList)
	privateKeys := make([]p2p_crypto.PrivKey, peerNumber, peerNumber)
	for i := 0; i < peerNumber; i++ {
		privateKeys[i], _, _ = p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	}

	peerList := ""
	for i := 0; i < peerNumber; i++ {
		id, _ := p2p_peer.IDFromPrivateKey(privateKeys[i])
		bootstrapAddress := fmt.Sprintf("%s@%s:%d", p2p_peer.Encode(id), ipList[i], port)
		peerList += bootstrapAddress
		if i != peerNumber -1 {
			peerList += ","
		}
	}

	for i := 0; i < peerNumber; i++ {
		prvBytes, err := p2p_crypto.MarshalPrivateKey(privateKeys[i])
		if err != nil {
			panic(err)
		}

		data := Config{
			PrivateKey: hex.EncodeToString(prvBytes),
			Idx:        uint32(i),
			Port:       uint32(port),
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
	}

	f, err := os.Create("./deploy.sh")
	if err != nil {
		panic(err)
	}
	for i := 0; i < peerNumber; i++ {
		line := fmt.Sprintf("scp -i %s ./testhub root@%s:testhub\n", sshkeypath, ipList[i])
		f.WriteString(line)
		line = fmt.Sprintf("scp -i %s ./testhubconfig%d.json root@%s:testhubconfig.json\n", sshkeypath, i,
			ipList[i])
		f.WriteString(line)
	}
	f.Sync()
	f.Close()

	f, err = os.Create("./starthubs.sh")
	if err != nil {
		panic(err)
	}
	for i := 0; i < peerNumber; i++ {
		line := fmt.Sprintf("ssh -i %s root@%s './testhub -config ./testhubconfig.json' &\n", sshkeypath, ipList[i])
		f.WriteString(line)
	}
	f.Sync()
	f.Close()

	f, err = os.Create("./remove.sh")
	if err != nil {
		panic(err)
	}
	for i := 0; i < peerNumber; i++ {
		line := fmt.Sprintf("ssh -i %s root@%s 'rm -rf ./testhub'\n", sshkeypath, ipList[i])
		f.WriteString(line)
		line = fmt.Sprintf("ssh -i %s root@%s 'rm -rf ./testhubconfig.json'\n", sshkeypath, ipList[i])
		f.WriteString(line)
	}
	f.Sync()
	f.Close()
}

func main() {
	var peerNumber uint
	var port uint
	var ips string
	var sshkeypath string
	flag.UintVar(&peerNumber, "peerNumber", 4, "Specify number of peers in test, should match number of peers in ips for remote tests, default value is 4", )
	flag.UintVar(&port, "port", 7070, "Base port value for local tests and port for remote hosts, default value is 7070", )
	flag.StringVar(&ips, "ips", "", "Comma-separated list of hosts for remote tests,  empty for local tests, default value is empty", )
	flag.StringVar(&sshkeypath, "key", "", "Path to key file to use in ssh for remote tests, empty for local tests, default value is empty", )
	flag.Parse()

	if len(ips) > 0 {
		generateRemoteConfig(ips, port, sshkeypath)
	} else {
		generateLocalConfig(peerNumber, port)
	}
}
