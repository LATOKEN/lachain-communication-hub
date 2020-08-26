package utils

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ChainId = 41
)

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
	sig[64] = (sig[64] - 36) / 2 / ChainId // Transform V

	rpk, err := crypto.Ecrecover(dataHash, sig)
	if err != nil {
		return nil, err
	}

	pub, err := crypto.UnmarshalPubkey(rpk)

	return pub, nil
}
