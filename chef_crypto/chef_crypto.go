/* Various cryptographic functions, as needed. */

/*
 * Copyright (c) 2013-2014, Jeremy Bingham (<jbingham@gmail.com>)
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package chef_crypto bundles up crytographic routines for goairdi.
package chef_crypto

import (
	"fmt"
	"crypto/rsa"
	"crypto/rand"
	"encoding/pem"
	"crypto/x509"
	"encoding/base64"
	"math/big"
	"crypto/sha512"
	"encoding/hex"
)

// Creates a pair of private and public keys for a client.
func GenerateRSAKeys() (string, string, error){
	/* Shamelessly borrowed and adapted from some golang-samples */
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	if err := priv.Validate(); err != nil {
		err_str := fmt.Errorf("RSA key validation failed: %s", err)
		return "", "", err_str
	}
	priv_der := x509.MarshalPKCS1PrivateKey(priv)
	/* For some reason chef doesn't label the keys RSA PRIVATE/PUBLIC KEY */
	priv_blk := pem.Block {
		Type: "RSA PRIVATE KEY",
		Headers: nil,
		Bytes: priv_der,
	}
	priv_pem := string(pem.EncodeToMemory(&priv_blk))
	pub := priv.PublicKey
	pub_der, err := x509.MarshalPKIXPublicKey(&pub)
	if err != nil {
		err_str := fmt.Errorf("Failed to get der format for public key: %s", err)
		return "", "", err_str
	}
	pub_blk := pem.Block {
		Type: "PUBLIC KEY",
		Headers: nil,
		Bytes: pub_der,
	}
	pub_pem := string(pem.EncodeToMemory(&pub_blk))
	return priv_pem, pub_pem, nil
}

// Checks that the provided public key is valid.
func ValidatePublicKey(publicKey interface{}) (bool, error) {
	switch publicKey := publicKey.(type) {
		case string:
			// at the moment we don't care about the pub interface
			decPubKey, _ := pem.Decode([]byte(publicKey))
			if decPubKey == nil {
				err := fmt.Errorf("Public key does not validate")
				return false, err
			}
			if _, err := x509.ParsePKIXPublicKey(decPubKey.Bytes); err != nil {
				nerr := fmt.Errorf("Public key did not validate: %s", err.Error())
				return false, nerr
			}
			return true, nil
		default:
			err := fmt.Errorf("Public key does not validate")
			return false, err
	}
}

// Decrypt the encrypted header with the client or user's public key for 
// validating requests. This function is informed by chef-golang's 
// privateDecrypt function.
func HeaderDecrypt(pkPem string, data string) ([]byte, error){
	block, _ := pem.Decode([]byte(pkPem))
	if block == nil {
		return nil, fmt.Errorf("Invalid block size for '%s'", pkPem)
	}
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	decData, perr := base64.StdEncoding.DecodeString(data)
	if perr != nil {
		return nil, perr
	}
	/* decData, dderr := pem.Decode(pemData)
	if decData == nil {
		return nil, fmt.Errorf("pem decoding didn't go so well, %s", string(dderr))
	} */
	dec, derr := decrypt(pubKey.(*rsa.PublicKey), decData)
	if derr != nil {
		return nil, derr
	}
	/* skip past the 0xff padding added to the header before encrypting. */
	skip := 0
	for i := 2; i < len(dec); i++{
		if dec[i] == 0xff && dec[i + 1] == 0 {
			skip = i + 2
			break;
		}
	}
	return dec[skip:], nil
}

// There has been discussion of renaming this and submitting it along with its
// counterpart in chef-golang to crypto/rsa.
func decrypt(pubKey *rsa.PublicKey, data []byte) ([]byte, error) {
	c := new(big.Int)
	m := new(big.Int)
	m.SetBytes(data)
	e := big.NewInt(int64(pubKey.E))
	c.Exp(m, e, pubKey.N)
	out := c.Bytes()

	return out, nil
}

// SHA512 hash a password string with the provided salt.
func HashPasswd(passwd string, salt []byte) (string, error) {
	if passwd == "" {
		err := fmt.Errorf("Password is empty")
		return "", err
	}
	hashPwByte := sha512.Sum512(append(salt, []byte(passwd)...))
	hashPw := hex.EncodeToString(hashPwByte[:])
	return hashPw, nil
}

// Generate a new salt for hashing a password.
func GenerateSalt() ([]byte, error) {
	numbytes := 64
	b := make([]byte, numbytes)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}
