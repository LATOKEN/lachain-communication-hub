package main

import "C"
import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	configI "lachain-communication-hub/config"
	"lachain-communication-hub/peer_service"
	"strings"
	"time"
	"unsafe"

	"github.com/libp2p/go-libp2p-core/crypto"
)

const bufSize = 10000

type ConfigData struct {
	PrivateKey string `json: "private_key"`
	Idx        uint32 `json: "index"`
	Port       uint32 `json: "port"`
	Bootstraps string `json: "bootstraps"`
	Repeats    uint32 `json: "repeatCount"`
	Network    string `json: "network"`
}

func BroadcastMessage(msg []byte, thisNode *RBCEmulation) {
	SendMessage(unsafe.Pointer(&ZeroPub[0]), C.int(len(ZeroPub)), unsafe.Pointer(&msg[0]), C.int(len(msg)))
	// send it to yourself too
	thisNode.Process(msg)
}

type RBCEmulation struct {
	era        uint32
	idx        uint32
	peerNumber uint32
	vals       []bool
	echos      map[uint32][]bool
	readys     []bool
}

func NewRBCEmulation(idx uint32, participants uint32) *RBCEmulation {
	ret := &RBCEmulation{
		era:        0,
		idx:        idx,
		peerNumber: participants,
		vals:       make([]bool, participants, participants),
		echos:      make(map[uint32][]bool),
		readys:     make([]bool, participants, participants),
	}
	for i := uint32(0); i < participants; i++ {
		ret.echos[i] = make([]bool, participants, participants)
	}
	return ret
}

func (p *RBCEmulation) BroadcastValMessage() {
	data := map[string]uint32{"type": 0, "from": p.idx, "era": p.era}
	datamsg, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d: send val message\n", p.idx)
	BroadcastMessage(datamsg, p)
}

func (p *RBCEmulation) Process(msg []byte) {
	var data map[string]uint32
	err := json.Unmarshal(msg, &data)
	if err != nil {
		panic(err)
	}
	if data["era"] != p.era {
		fmt.Printf("Invalid era: %d, %d\n", data["era"], p.era)
	}
	switch data["type"] {
	case 0:
		p.vals[data["from"]] = true
		echo := map[string]uint32{"type": 1, "from": p.idx, "val": data["from"], "era": p.era}
		echomsg, err := json.Marshal(echo)
		if err != nil {
			fmt.Println("error:", err)
		}
		fmt.Printf("%d: send echo message for %d \n", p.idx, data["from"])
		BroadcastMessage(echomsg, p)
		break
	case 1:
		p.echos[data["from"]][data["val"]] = true
		ready := true
		for _, echos := range p.echos {
			for _, val := range echos {
				if !val {
					ready = false
				}
			}
		}
		if ready {
			readydata := map[string]uint32{"type": 2, "from": p.idx, "era": p.era}
			readymsg, err := json.Marshal(readydata)
			if err != nil {
				fmt.Println("error:", err)
			}
			fmt.Printf("%d: send ready message\n", p.idx)
			BroadcastMessage(readymsg, p)
		}
		break
	case 2:
		p.readys[data["from"]] = true
		break
	}
}

func (p *RBCEmulation) IsReady() bool {
	for _, value := range p.readys {
		if !value {
			return false
		}
	}
	return true
}

func getPeersCount(peers string) uint32 {
	var data = strings.Split(peers, ",")
	return uint32(len(data))
}

//export TestStartHub
func TestStartHub(bootstrapAddress string, privKeyHex string, network string) {
	mutex.Lock()
	defer mutex.Unlock()
	// Hardcode chainId
	configI.ChainId = byte(41)
	configI.SetBootstrapAddress(bootstrapAddress)
	prvBytes, err := hex.DecodeString(string(privKeyHex))
	if err != nil {
		panic(err)
	}
	prv, err2 := crypto.UnmarshalPrivateKey(prvBytes)
	if err2 != nil {
		panic(err2)
	}
	localPeer = peer_service.New(prv, network, 1, 0, ProcessMessage)
}

func main() {
	var configPath string

	flag.StringVar(&configPath, "config", "testhubconfig0.json", "Config path,  defaiult value is testhubconfig0.json")
	flag.Parse()

	// read file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}

	// parse json
	var config = ConfigData{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		panic(err)
	}

	// start hub
	TestStartHub(config.Bootstraps, config.PrivateKey, config.Network)

	// start RBC emulation
	participants := getPeersCount(config.Bootstraps)
	protocol := NewRBCEmulation(config.Idx, participants)

	for i := uint32(0); i < config.Repeats; i++ {
		protocol.BroadcastValMessage()

		// process messages until get all of them
		buffer := make([]byte, bufSize, bufSize)
		for {
			msgCount := int(GetMessages(unsafe.Pointer(&buffer[0]), bufSize))
			if msgCount == -1 {
				continue
			}
			ptr := uint32(0)
			for i := 0; i < msgCount; i++ {
				msgLen := binary.LittleEndian.Uint32(buffer[ptr : ptr+4])
				ptr += 4
				protocol.Process(buffer[ptr : ptr+msgLen])
				ptr += msgLen
			}
			if protocol.IsReady() {
				break
			}
		}
		fmt.Printf("%d hub completed era %d\n", config.Idx, i)
		protocol.era++
	}
	// wait for all messages are sent
	time.Sleep(3 * time.Second)

	// stop hub
	StopHub()

	fmt.Printf("%d hub finished\n", config.Idx)
}
