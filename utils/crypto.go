package utils

import (
	"crypto/ecdsa"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/juju/loggo"
)

var log = loggo.GetLogger("utils")

func LaSign(data []byte, prv *ecdsa.PrivateKey, chainId byte) ([]byte, error) {

	dataHash := crypto.Keccak256(data)
	signature, err := crypto.Sign(dataHash, prv)
	if err != nil {
		return nil, err
	}

	if chainId < 110 {
		signature[64] = chainId*2 + 35 + signature[64]
		return signature, nil
	}
	result := make([]byte, 66)
	encodedRecId := uint32(chainId)*2 + 35 + uint32(signature[64])
	recIdBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(recIdBytes, encodedRecId)
	copy(result[0:64], signature[0:64])
	// only 2 first bytes contains non-zero value because chainId is byte, so it is less than 256
	result[64] = recIdBytes[1]
	result[65] = recIdBytes[0]
	return result, nil
}

func recoverEncodedRecIdFromSignature(sig []byte) (int, error) {
	if len(sig) != 65 && len(sig) != 66 {
		return -1, fmt.Errorf("signature must be 65 bytes long")
	}
	if len(sig) == 65 {
		return int(sig[64]), nil
	}
	slice := make([]byte, 4)
	slice[0] = sig[65]
	slice[1] = sig[64]
	return int(binary.LittleEndian.Uint32(slice)), nil
}

func EcRecover(data, sig []byte, chainId byte) (*ecdsa.PublicKey, error) {
	dataHash := crypto.Keccak256(data)
	if len(sig) != 65 && len(sig) != 66 {
		return nil, fmt.Errorf("signature must be 65 or 66 bytes long")
	}
	recSig := make([]byte, 65)
	copy(recSig, sig[0:64])
	encodedRecId, err := recoverEncodedRecIdFromSignature(sig)
	if err != nil {
		return nil, err
	}
	recId := (encodedRecId - 36) / 2 / int(chainId) // Transform V
	recSig[64] = byte(recId)

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
	return BytesToHex(crypto.CompressPubkey(publicKey))
}

func PublicKeyToBytes(publicKey *ecdsa.PublicKey) []byte {
	return crypto.CompressPubkey(publicKey)
}

func HexToPublicKey(publicKey string) *ecdsa.PublicKey {
	publicKeyBytes := HexToBytes(publicKey)

	pub, err := crypto.DecompressPubkey(publicKeyBytes)
	if err != nil {
		log.Errorf("can't unmarshal public key: %s", publicKey)
	}

	return pub
}

func HexToBytes(publicKey string) []byte {
	publicKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		log.Errorf("can't decode public key: %s", publicKey)
	}

	return publicKeyBytes
}

func BytesToHex(publicKey []byte) string {
	return hex.EncodeToString(publicKey)
}
