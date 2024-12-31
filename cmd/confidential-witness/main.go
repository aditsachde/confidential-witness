package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"cloud.google.com/go/compute/metadata"
	kms "cloud.google.com/go/kms/apiv1"
	"github.com/transparency-dev/witness/monitoring"
	"github.com/transparency-dev/witness/omniwitness"
	"golang.org/x/mod/sumdb/note"
	"google.golang.org/api/option"
)

func main() {
	o_ctx, cancel := context.WithTimeout(context.Background(), 22*time.Hour)
	defer cancel()
	meta := getMetadata(o_ctx)

	// Keygen
	// The key is seeded by a fixed seed. See getSeed for details.
	client, err := getClient(o_ctx, meta)
	if err != nil {
		log.Fatalln("Failed to create KMS client:", err)
	}
	defer client.Close()

	noteKms, err := NewNoteKms(o_ctx, client, meta.key, meta.name)
	if err != nil {
		log.Fatalln("Failed to create NoteKms:", err)
	}

	// Serve the public key on port 8080 so that it is actually accessible somewhere.
	// Confidential spaces disable logs on production workloads.
	revision, modified := getRevision()
	publicKey := noteKms.PublicKey()
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, publicKey+"\n\n"+revision+"\n"+modified)
		})
		http.ListenAndServe(":8080", nil)
	}()

	o_operatorConfig := omniwitness.OperatorConfig{
		WitnessKeys:     []note.Signer{noteKms},
		WitnessVerifier: noteKms,
		FeedInterval:    time.Minute,
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
	region   string
	name     string
	key      string
	audience string
}

// Returns metadata from the environment
func getMetadata(ctx context.Context) Meta {
	var meta Meta

	if metadata.OnGCE() {
		zone, err := metadata.ZoneWithContext(ctx)
		if err != nil {
			log.Fatalln("Failed to get zone:", err)
		}
		meta.region = zone[:len(zone)-2]
	} else {
		meta.region = "dev"
	}

	meta.name = os.Getenv("WITNESS_NAME")
	if meta.name == "" {
		log.Fatalln("WITNESS_NAME not set")
	}

	meta.key = os.Getenv("WITNESS_KEY")
	if meta.key == "" {
		log.Fatalf("Environment variable WITNESS_KEY is not set or is empty")
	}

	meta.audience = os.Getenv("WITNESS_AUDIENCE")
	if meta.audience == "" {
		log.Fatalf("Environment variable WITNESS_AUDIENCE is not set or is empty")
	}

	return meta
}

// Returns a name for the witness that is unique to the zone
// to allow for multiple witnesses with the same configuration
func getName(meta Meta) string {
	return meta.name + "-" + meta.region
}

// Create a new Cloud KMS Client
func getClient(ctx context.Context, meta Meta) (*kms.KeyManagementClient, error) {
	// this token is managed by the confidential space runner
	attestation_token_path := "/run/container_launcher/attestation_verifier_claims_token"

	creds := fmt.Sprintf(`{
	"type": "external_account",
	"audience": "%s",
	"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
	"token_url": "https://sts.googleapis.com/v1/token",
	"credential_source": {
	  "file": "%s"
	}
	}`, meta.audience, attestation_token_path)

	// Create the client.
	client, err := kms.NewKeyManagementClient(ctx, option.WithCredentialsJSON([]byte(creds)))
	if err != nil {
		return nil, fmt.Errorf("failed to create kms client: %w", err)
	}
	return client, nil
}

// Current Git commit hash and if the repository is modified
func getRevision() (revision string, modified string) {
	info, _ := debug.ReadBuildInfo()
	for _, i := range info.Settings {
		if i.Key == "vcs.revision" {
			revision = i.Value
		} else if i.Key == "vcs.modified" {
			modified = i.Value
		}
	}
	return
}
