package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"log"

	f_note "github.com/transparency-dev/formats/note"
)

func main() {

	rawkey := `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEApTPPKyRu6Q3PoMkeY+/sNni0biF4vnVTYRHbVNA31mY=
-----END PUBLIC KEY-----`

	block, _ := pem.Decode([]byte(rawkey))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Fatalln("Failed to parse public key:", err)
	}

	pubkey, ok := key.(ed25519.PublicKey)
	if !ok {
		log.Fatalln("Invalid public key, not ed25519")
	}
	
	encoded := base64.StdEncoding.EncodeToString(
		// The algorithm byte is expected to be algEd25519, not algEd25519CosignatureV1.
		append([]byte{1}, pubkey...))

	verifier, err := f_note.NewVerifierForCosignatureV1("witness+00000000+" + encoded)
	if err != nil {
		log.Fatalln("Failed to create verifier:", err)
	}

	msg := `developers.google.com/android/binary_transparency/google1p/0
5
xqaCEZhTGZUuKNA7MpSyxe1WXiiESB2p+crAyzM+Vxo=
`
	sig, err := base64.StdEncoding.DecodeString("r0xBJQAAAABnkyoLuHgN5KlIUUpZV9hulwsFcgqV5v1CyeIhBZWdkfOJT0ims1LqbYygO5MEl91CGha5sP79K57P7lIGrz7Qtx8eBw==")
	if err != nil {
		log.Fatalln("Failed to decode signature:", err)
	}

	result := verifier.Verify([]byte(msg), sig[4:])
	if !result {
		log.Fatalln("Failed to verify signature")
	}
}
