package utils

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/juju/loggo"
)

const (
	ChainId = 41
)

var log = loggo.GetLogger("utils")

func LaSign(data []byte, prv *ecdsa.PrivateKey) ([]byte, error) {

	dataHash := crypto.Keccak256(data)
	signature, err := crypto.Sign(dataHash, prv)

	if err != nil {
		return nil, err
	}
	signature[64] = ChainId*2 + 35 + signature[64]
	return signature, nil
}

func EcRecover(data, sig []byte) (*ecdsa.PublicKey, error) {
	dataHash := crypto.Keccak256(data)
	if len(sig) != 65 {
		return nil, fmt.Errorf("signature must be 65 bytes long")
	}
	recSig := make([]byte, 65)
	copy(recSig, sig)
	recSig[64] = (sig[64] - 36) / 2 / ChainId // Transform V

	rpk, err := crypto.Ecrecover(dataHash, recSig)
	if err != nil {
		return nil, err
	}

	pub, err := crypto.UnmarshalPubkey(rpk)
	if err != nil {
		return nil, err
	}

	return pub, nil
}

func PublicKeyToHexString(publicKey *ecdsa.PublicKey) string {
	return hex.EncodeToString(crypto.CompressPubkey(publicKey))
}

func PublicKeyToBytes(publicKey *ecdsa.PublicKey) []byte {
	return crypto.CompressPubkey(publicKey)
}

func HexToPublicKey(publicKey string) *ecdsa.PublicKey {
	publicKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		log.Errorf("can't decode public key: %s", publicKey)
	}

	pub, err := crypto.DecompressPubkey(publicKeyBytes)
	if err != nil {
		log.Errorf("can't unmarshal public key: %s", publicKey)
	}

	return pub
}
