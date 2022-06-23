package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/sha3"
)

// makeTLSCert generates a deterministic TLS certificate from a shared passphrase
// to be shared and used by all brokers. The certificate is returned in serialized
// format as NSQ does not support being used as a library and being fed the cert
// directly from Go.
func MakeTLSCert(passphrase string) ([]byte, []byte) {
	// Generate the certificate key
	seed := pbkdf2.Key([]byte(passphrase), nil, 4096, 32, sha3.New256)
	priv := ed25519.NewKeyFromSeed(seed)

	blob, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		panic(err) // key was just generated, it must be ok
	}
	pemPriv := pem.EncodeToMemory(&pem.Block{Type: "ES PRIVATE KEY", Bytes: blob})

	// Generate the self-signed permanent certificate
	template := x509.Certificate{
		SerialNumber: new(big.Int),                         // Nice, complicated, universally "unique" serial number
		NotAfter:     time.Unix(31415926535, 0),            // Permanent certificate, never expires
		IPAddresses:  []net.IP{net.IPv4zero, net.IPv6zero}, // Authenticate all IP addresses with it
	}
	blob, err = x509.CreateCertificate(nil, &template, &template, priv.Public(), priv)
	if err != nil {
		panic(err)
	}
	pemCert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: blob})

	return pemCert, pemPriv
}

// makeTLSConfig coverts a binary certificate and private key into a Go specific
// TLS config to be used as client and server authentication and encryption.
func MakeTLSConfig(cert []byte, key []byte) *tls.Config {
	certificate, err := tls.X509KeyPair(cert, key)
	if err != nil {
		panic(err) // cert and key was generated by makeTLSCert, must be ok
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS13,

		Certificates:       []tls.Certificate{certificate},
		InsecureSkipVerify: true,

		// VerifyPeerCertificate is the actual certification validation since we
		// need to skip IP address validation as we have a wildcard certificate
		VerifyPeerCertificate: func(certificates [][]byte, _ [][]*x509.Certificate) error {
			// We know we have at least one certificate since we're initiating a
			// TLS session and the server must authenticate itself.
			cert, err := x509.ParseCertificate(certificates[0])
			if err != nil {
				return err
			}
			// We only use Ed25519 curves, discard any connections not speaking it
			pub, ok := cert.PublicKey.(ed25519.PublicKey)
			if !ok {
				return errors.New("invalid public key type")
			}
			// The certificate has the right crypto, authenticate it
			if !bytes.Equal(pub, certificate.PrivateKey.(ed25519.PrivateKey).Public().(ed25519.PublicKey)) {
				return errors.New("unexpected server key")
			}
			// Public key authorized, validate the self-signed certificate
			return cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)
		},
	}
}
