package main_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"lachain-communication-hub/config"
	"lachain-communication-hub/peer"
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
	config.SetBootstrapAddress(bootstrapAddress)

	log.Debugf("Register Bootstrap address: %s", bootstrapAddress)
}

func makeServerPeer(priv_key p2p_crypto.PrivKey) (*peer.Peer, []byte) {
	p := peer.New(priv_key)

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

	signature, err := utils.LaSign(id, prv)
	if err != nil {
		panic(err)
	}

	if !p.Register(signature) {
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

	p1, _ := makeServerPeer(priv_key1)
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2)
	defer p2.Stop()

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

	p2.SetStreamHandlerFn(handler)
	p1.SendMessageToPeer(hex.EncodeToString(pub2), goldenMessage, true)

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

	p1, _ := makeServerPeer(priv_key1)
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2)
	defer p2.Stop()

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

	p2.SetStreamHandlerFn(handler)
	pub2str := hex.EncodeToString(pub2)
	for j := 0; j < 100; j++ {
		go func() {
			for i := 0; i < 100; i++ {
				p1.SendMessageToPeer(pub2str, goldenMessage, true)
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

	p1, _ := makeServerPeer(priv_key1)
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2)
	defer p2.Stop()

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

	p2.SetStreamHandlerFn(handler)
	pub2str := hex.EncodeToString(pub2)
	for i := 0; i < 10000; i++ {
		p1.SendMessageToPeer(pub2str, goldenMessage, true)
	}

	go func() {
		for {
			time.Sleep(100 * time.Millisecond)
			p2.Stop()
			p2, _ = makeServerPeer(priv_key2)
			p2.SetStreamHandlerFn(handler)
		}
	}()

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

func TestBigMessage(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	p1, _ := makeServerPeer(priv_key1)
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2)
	defer p2.Stop()

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

	p2.SetStreamHandlerFn(handler)
	p1.SendMessageToPeer(hex.EncodeToString(pub2), goldenMessage, true)

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
	loggo.ConfigureLoggers("<root>=TRACE")

	repeatCount := 10

	peers := make([]*peer.Peer, 0)
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
			p, k := makeServerPeer(priv_key)
			p.SetStreamHandlerFn(handler)
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
		if p.SendMessageToPeer(pkstr, goldenMessage, true) {
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
