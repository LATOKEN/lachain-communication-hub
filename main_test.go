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
	config.SetBootstrapAddress(bootstrapAddress)

	log.Debugf("Register Bootstrap address: %s", bootstrapAddress)
}

func makeServerPeer(priv_key p2p_crypto.PrivKey, handler func([]byte)) (*peer_service.PeerService, []byte) {
	p := peer_service.New(priv_key, handler)

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

	p1, _ := makeServerPeer(priv_key1, func([]byte) {})
	defer p1.Stop()
	p2, pub2 := makeServerPeer(priv_key2, handler)
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

	p1, _ := makeServerPeer(priv_key1, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, handler)
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

	p1, _ := makeServerPeer(priv_key1, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, handler)
	defer p2.Stop()

	pub2str := hex.EncodeToString(pub2)
	for i := 0; i < 10000; i++ {
		p1.SendMessageToPeer(pub2str, goldenMessage)
	}

	go func() {
		//for {
		time.Sleep(1000 * time.Millisecond)
		p2.Stop()
		p2, _ = makeServerPeer(priv_key2, handler)
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

	p1, _ := makeServerPeer(priv_key1, func([]byte) {})
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, handler)
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

func TestMassSend2NodesMulti(t *testing.T) {
	loggo.ConfigureLoggers("<root>=TRACE")

	priv_key1, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)
	priv_key2, _, _ := p2p_crypto.GenerateECDSAKeyPair(rand.Reader)

	registerBootstrap(priv_key1, ":41011")
	registerBootstrap(priv_key2, ":41012")

	done1 := make(chan bool)
	fail1 := make(chan bool)

	done2 := make(chan bool)
	fail2 := make(chan bool)

	goldenMessage := []byte("fgdghfghfhgghfhj")
	counter1 := 0
	counter2 := 0

	handler1 := func(msg []byte) {
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
			fail1 <- true
		}
		assert.Equal(t, msg, goldenMessage)
		if counter1++; counter1 == 10000 {
			done1 <- true
		}
	}

	handler2 := func(msg []byte) {
		if !bytes.Equal(msg, goldenMessage) {
			log.Errorf("bad response")
			fail2 <- true
		}
		assert.Equal(t, msg, goldenMessage)
		if counter2++; counter2 == 10000 {
			done2 <- true
		}
	}

	p1, pub1 := makeServerPeer(priv_key1, handler1)
	defer p1.Stop()

	p2, pub2 := makeServerPeer(priv_key2, handler2)
	defer p2.Stop()

	pub2str := hex.EncodeToString(pub2)
	pub1str := hex.EncodeToString(pub1)

	for j := 0; j < 100; j++ {
		go func() {
			for i := 0; i < 100; i++ {
				p1.SendMessageToPeer(pub2str, goldenMessage)
			}
		}()
	}

	for j := 0; j < 100; j++ {
		go func() {
			for i := 0; i < 100; i++ {
				p2.SendMessageToPeer(pub1str, goldenMessage)
			}
		}()
	}

	ticker := time.NewTicker(time.Minute)

	select {
	case <-done1:
		ticker.Stop()
		log.Infof("Finished")
	case <-fail1:
		ticker.Stop()
		log.Errorf("Failed to process nessages")
	case <-ticker.C:
		log.Errorf("Failed to receive all messages in time")
		t.Error("Failed to receive message in time")
	}

	select {
	case <-done2:
		ticker.Stop()
		log.Infof("Finished")
	case <-fail2:
		ticker.Stop()
		log.Errorf("Failed to process nessages")
	case <-ticker.C:
		log.Errorf("Failed to receive all messages in time")
		t.Error("Failed to receive message in time")
	}

	fmt.Printf("%d counter1", counter1)
	fmt.Printf("%d counter2", counter2)

}
