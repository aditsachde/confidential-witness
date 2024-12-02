package main

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"github.com/transparency-dev/witness/monitoring"
	"github.com/transparency-dev/witness/omniwitness"
	"golang.org/x/mod/sumdb/note"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func main() {
	o_ctx := context.Background()
	meta := getMetadata(o_ctx)

	// Keygen
	// The key is seeded by a fixed seed. See getSeed for details.
	skey, vkey, err := note.GenerateKey(getSeed(o_ctx, meta), getName(meta))
	if err != nil {
		log.Fatalln("Failed to generate key:", err)
	}
	log.Println("public key:", vkey)

	// Serve the public key on port 8080 so that it is actually accessible somewhere.
	// Confidential spaces disable logs on production workloads.
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, vkey)
		})
		http.ListenAndServe(":8080", nil)
	}()

	o_operatorConfig := omniwitness.OperatorConfig{
		WitnessKey: skey,
	}

	// Persistence
	// TOFU on startup
	o_p := NewPersistence()

	// Listener
	var o_httpListener net.Listener
	o_httpListener, err = net.Listen("tcp", ":80")
	if err != nil {
		log.Fatalln("Failed to start listener:", err)
	}

	// Outbound
	var o_httpClient *http.Client = &http.Client{}

	// Metrics
	monitoring.SetMetricFactory(monitoring.InertMetricFactory{})

	// Start
	log.Println("starting server...")
	err = omniwitness.Main(o_ctx, o_operatorConfig, o_p, o_httpListener, o_httpClient)
	log.Fatalln("Omniwitness exited:", err)

}

// Witness metadata
type Meta struct {
	zone string
	name string
	key  string
}

// Returns metadata from the environment
func getMetadata(ctx context.Context) Meta {
	var meta Meta

	if metadata.OnGCE() {
		zone, err := metadata.ZoneWithContext(ctx)
		if err != nil {
			log.Fatalln("Failed to get zone:", err)
		}
		meta.zone = zone
	} else {
		meta.zone = "dev"
	}

	meta.name = os.Getenv("WITNESS_NAME")
	if meta.name == "" {
		log.Fatalln("WITNESS_NAME not set")
	}

	meta.key = os.Getenv("WITNESS_KEY")
	if meta.key == "" {
		log.Fatalf("Environment variable WITNESS_KEY is not set or is empty")
	}

	return meta
}

// Returns a name for the witness that is unique to the zone
// to allow for multiple witnesses with the same configuration
func getName(meta Meta) string {
	return meta.name + "-" + meta.zone
}

// getSeed returns a fixed seed for the key generation process.
// The seed is the signature of the name using the private key stored in Cloud KMS.
// This ensures that the key is unique to the witness and is bound to the key stored in KMS.
//
// This is necessary because omniwitness requires a key that resides in memory due
// to the way it initializes the underlying note signers, which is not possible for a
// key in Cloud KMS. This could be fixed by extending OperatorConfig with a []note.Signer.
func getSeed(ctx context.Context, meta Meta) io.Reader {
	sig, err := signAsymmetric(ctx, getName(meta), meta.key)
	if err != nil {
		log.Fatalln("Failed to sign message:", err)
	}

	// Truncate signature to 32 bytes
	var seed [32]byte
	copy(seed[:], sig)

	return bytes.NewReader(seed[:])
}

// signAsymmetric will sign a plaintext message using a saved asymmetric private
// key stored in Cloud KMS.
func signAsymmetric(ctx context.Context, message string, key string) ([]byte, error) {
	// Create the client.
	client, err := kms.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create kms client: %w", err)
	}
	defer client.Close()

	// Convert the message into bytes. Cryptographic plaintexts and
	// ciphertexts are always byte arrays.
	plaintext := []byte(message)

	// Optional but recommended: Compute digest's CRC32C.
	crc32c := func(data []byte) uint32 {
		t := crc32.MakeTable(crc32.Castagnoli)
		return crc32.Checksum(data, t)

	}
	dataCRC32C := crc32c(plaintext)

	// Build the signing request.
	//
	// Note: Key algorithms will require a varying hash function. For example,
	// EC_SIGN_P384_SHA384 requires SHA-384.
	req := &kmspb.AsymmetricSignRequest{
		Name:       key,
		Data:       plaintext,
		DataCrc32C: wrapperspb.Int64(int64(dataCRC32C)),
	}

	// Call the API.
	result, err := client.AsymmetricSign(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to sign digest: %w", err)
	}

	// Optional, but recommended: perform integrity verification on result.
	// For more details on ensuring E2E in-transit integrity to and from Cloud KMS visit:
	// https://cloud.google.com/kms/docs/data-integrity-guidelines
	if result.VerifiedDataCrc32C == false {
		return nil, fmt.Errorf("AsymmetricSign: request corrupted in-transit 1")
	}
	if result.Name != req.Name {
		return nil, fmt.Errorf("AsymmetricSign: request corrupted in-transit 2")
	}
	if int64(crc32c(result.Signature)) != result.SignatureCrc32C.Value {
		return nil, fmt.Errorf("AsymmetricSign: response corrupted in-transit 3")
	}

	return result.Signature, nil
}
