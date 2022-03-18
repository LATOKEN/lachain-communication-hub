package main_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"lachain-communication-hub/config"
	"lachain-communication-hub/peer_service"
	"lachain-communication-hub/utils"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/juju/loggo"
	p2p_crypto "github.com/libp2p/go-libp2p-core/crypto"
	p2p_peer "github.com/libp2p/go-libp2p-core/peer"
	"github.com/magiconair/properties/assert"
)

var log = loggo.GetLogger("builder.go")

func registerBootstrap(prv p2p_crypto.PrivKey, port string) {
	id, _ := p2p_peer.IDFromPrivateKey(prv)
	bootstrapAddress := p2p_peer.Encode(id) + "@127.0.0.1" + port
	config.ChainId = byte(41)
	config.SetBootstrapAddress(bootstrapAddress)

	log.Debugf("Register Bootstrap address: %s", bootstrapAddress)
}

func makeServerPeer(priv_key p2p_crypto.PrivKey, network string, version int32, minPeerVersion int32, handler func([]byte)) (*peer_service.PeerService, []byte) {
	p := peer_service.New(priv_key, network, version, minPeerVersion, handler)

	var id []byte
	for {
		id = p.GetId()
		if id != nil {
			break
		}
	}

	prv, err := crypto.GenerateKey()
	if err != nil {
		log.Errorf("could not GenerateKey: %v", err)
	}
	pub := crypto.CompressPubkey(&prv.PublicKey)

	fmt.Println("pubKey", hex.EncodeToString(pub))

	signature, err := utils.LaSign(id, prv, 41)
	if err != nil {
		panic(err)
	}

	if !p.SetSignature(signature) {
		panic("Init failed")
	}

	return p, pub
}

func TestSingleSend(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	done := make(chan bool)

	goldenMessage := []byte("dfstrdfgcrjtdg")

	handler := func(msg []byte) {
		log.Infof("received message: %s", string(msg))
		log.Infof("len, %v", len(goldenMessage))
		log.Infof("len, %v", len(msg))
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
		}
		assert.Equal(t, msg, goldenMessage)
		done <- true
	}

	p1, _ := makeServerPeer(priv_key1, "unittest", 1, 0, func([]byte) {})
	defer p1.Stop()
	p2, pub2 := makeServerPeer(priv_key2, "unittest", 1, 0, handler)
	defer p2.Stop()
	p1.SendMessageToPeer(hex.EncodeToString(pub2), goldenMessage)

	ticker := time.NewTicker(time.Minute)
	select {
	case <-done:
		ticker.Stop()
		log.Infof("Finished")
	case <-ticker.C:
		log.Errorf("Failed to receive message in time")
		t.Error("Failed to receive message in time")
	}
}

func TestMassSend2Nodes(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	done := make(chan bool)
	fail := make(chan bool)

	goldenMessage := []byte("fgdghfghfhgghfhj")
	counter := 0

	handler := func(msg []byte) {
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
			fail <- true
		}
		assert.Equal(t, msg, goldenMessage)
		if counter++; counter == 10000 {
			done <- true
		}
	}

	p1, _ := makeServerPeer(priv_key1, "unittest", 1, 0, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, "unittest", 1, 0, handler)
	defer p2.Stop()

	pub2str := hex.EncodeToString(pub2)
	for j := 0; j < 100; j++ {
		go func() {
			for i := 0; i < 100; i++ {
				p1.SendMessageToPeer(pub2str, goldenMessage)
			}
		}()
	}

	ticker := time.NewTicker(time.Minute)
	select {
	case <-done:
		ticker.Stop()
		log.Infof("Finished")
	case <-fail:
		ticker.Stop()
		log.Errorf("Failed to process nessages")
	case <-ticker.C:
		log.Errorf("Failed to receive all messages in time")
		t.Error("Failed to receive message in time")
	}
}

func TestReconnect2Nodes(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	done := make(chan bool)
	fail := make(chan bool)

	goldenMessage := []byte("fgdghfhjgjkjkh")
	counter := 0

	handler := func(msg []byte) {
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
			fail <- true
		}
		assert.Equal(t, msg, goldenMessage)
		if counter++; counter == 10000 {
			done <- true
		}
	}

	p1, _ := makeServerPeer(priv_key1, "unittest", 1, 0, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, "unittest", 1, 0, handler)
	defer p2.Stop()

	pub2str := hex.EncodeToString(pub2)
	for i := 0; i < 10000; i++ {
		p1.SendMessageToPeer(pub2str, goldenMessage)
	}

	go func() {
		//for {
		time.Sleep(1000 * time.Millisecond)
		p2.Stop()
		p2, _ = makeServerPeer(priv_key2, "unittest", 1, 0, handler)
		//}
	}()

	ticker := time.NewTicker(10 * time.Minute)
	select {
	case <-done:
		ticker.Stop()
		log.Infof("Finished")
	case <-fail:
		ticker.Stop()
		log.Errorf("Failed to process messages")
	case <-ticker.C:
		log.Errorf("Failed to receive all messages in time")
		t.Error("Failed to receive message in time")
	}
}

func TestBigMessage(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	done := make(chan bool)

	goldenMessage := make([]byte, 6000, 6000)
	for i := 0; i < 6000; i++ {
		goldenMessage[i] = byte(i)
	}

	handler := func(msg []byte) {
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
		}
		assert.Equal(t, msg, goldenMessage)
		done <- true
	}

	p1, _ := makeServerPeer(priv_key1, "unittest", 1, 0, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, "unittest", 1, 0, handler)
	defer p2.Stop()

	p1.SendMessageToPeer(hex.EncodeToString(pub2), goldenMessage)

	ticker := time.NewTicker(time.Minute)
	select {
	case <-done:
		ticker.Stop()
		log.Infof("Finished")
	case <-ticker.C:
		log.Errorf("Failed to receive message in time")
		t.Error("Failed to receive message in time")
	}
}

func TestConcurrentBootstraps(t *testing.T) {
	loggo.ConfigureLoggers("<root>=DEBUG")

	repeatCount := 10

	peers := make([]*peer_service.PeerService, 0)
	pub_keys := make([][]byte, 0)

	inited := make(chan bool)
	counter := make(chan bool)
	done := make(chan bool)
	fail := make(chan bool)

	goldenMessage := []byte("ghdhgfhgfh")

	handler := func(msg []byte) {
		log.Infof("[%s], %x\n", msg, msg)
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
			fail <- true
		} else {
			counter <- true
		}
	}

	for i := 0; i < repeatCount; i++ {
		go func(idx int) {
			priv_key, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
			registerBootstrap(priv_key, fmt.Sprintf(":%d", 62000+idx))
			p, k := makeServerPeer(priv_key, "unittest", 1, 0, handler)
			peers = append(peers, p)
			pub_keys = append(pub_keys, k)
			inited <- true
		}(i)
	}

	defer func() {
		for i := 0; i < repeatCount; i++ {
			peers[i].Stop()
		}
	}()

	// wait for init completion
	for i := 0; i < repeatCount; i++ {
		<-inited
	}

	// send repeatCount - 1 messages
	sent := 0
	for i, p := range peers {
		if i == 0 {
			continue
		}
		pkstr := hex.EncodeToString(pub_keys[i-1])
		if p.SendMessageToPeer(pkstr, goldenMessage) {
			sent++
		}
	}

	// wait for all messages during a minute
	go func() {
		for i := 0; i < sent; i++ {
			<-counter
		}
		done <- true
	}()

	ticker := time.NewTicker(time.Minute)
	select {
	case <-fail:
		ticker.Stop()
		log.Errorf("Incorrect message received")
		t.Error("Incorrect message received")
	case <-done:
		ticker.Stop()
		log.Infof("Finished")
	case <-ticker.C:
		log.Errorf("Failed to receive message in time")
		t.Error("Failed to receive message in time")
	}
}

func TestProtocol(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key3, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key4, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")
	registerBootstrap(priv_key3, ":41013")
	registerBootstrap(priv_key4, ":41014")

	done := make(chan bool)
	wrong := make(chan bool)

	goldenMessage := []byte("dfstrdfgcrjtdg")

	handler := func(msg []byte) {
		done <- true
	}

	wrongHandler := func(msg []byte) {
		wrong <- true
	}

	p1, pub1 := makeServerPeer(priv_key1, "unittest", 10, 9, wrongHandler)
	defer p1.Stop()
	p2, pub2 := makeServerPeer(priv_key2, "unittest", 9, 0, handler)
	defer p2.Stop()
	p3, pub3 := makeServerPeer(priv_key3, "unittest", 8, 0, wrongHandler)
	defer p2.Stop()
	p4, pub4 := makeServerPeer(priv_key4, "wrongnet", 10, 9, wrongHandler)
	defer p2.Stop()
	p1.SendMessageToPeer(hex.EncodeToString(pub2), goldenMessage)
	p1.SendMessageToPeer(hex.EncodeToString(pub3), goldenMessage)
	p1.SendMessageToPeer(hex.EncodeToString(pub4), goldenMessage)
	p3.SendMessageToPeer(hex.EncodeToString(pub1), goldenMessage)
	p4.SendMessageToPeer(hex.EncodeToString(pub1), goldenMessage)

	select {
	case <-done:
		log.Infof("Finished")
	case <-time.After(10 * time.Second):
		log.Errorf("Failed to receive message in time")
		t.Error("Failed to receive message in time")
	}

	select {
	case <-wrong:
		t.Fatal("Wrong peer connected")
	case <-time.After(time.Second):
	}
}
