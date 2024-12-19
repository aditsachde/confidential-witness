package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"syscall"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

const (
	releaseURL = "https://api.github.com/repos/aditsachde/confidential-witness/releases/latest"
	userAgent  = "github.com/aditsachde/confidential-witness/cmd/bootloader"

	binaryName      = "confidential-witness"
	attestationName = "attestation.json"

	trustedRoot = "trusted_root.json"
	tufRepo     = "https://tuf-repo-cdn.sigstore.dev/"

	sanRegex = "https://github\\.com/aditsachde/confidential-witness/\\.github/workflows/build\\.yml@refs/tags/v.+"
	issuer   = "https://token.actions.githubusercontent.com"

	filePath         = "/confidential-witness"
	verificationPath = "/verification.json"
)

type Assets struct {
	Assets []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}

func main() {
	// Get the latest release json
	client := &http.Client{}

	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("failed to get release: %v", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var assets Assets
	err = json.NewDecoder(resp.Body).Decode(&assets)
	if err != nil {
		log.Fatalf("failed to decode response: %v", err)
	}

	// Find the binary and attestation assets
	var binaryAsset, attestationAsset *Asset
	for i, asset := range assets.Assets {
		if asset.Name == binaryName {
			binaryAsset = &assets.Assets[i]
		}
		if asset.Name == attestationName {
			attestationAsset = &assets.Assets[i]
		}
	}

	if binaryAsset == nil {
		log.Fatalf("Binary asset not found on latest release!")
	}
	if attestationAsset == nil {
		log.Fatalf("Attestation asset not found on latest release!")
	}

	// Download binary and attestation
	binaryResp, err := http.Get(binaryAsset.BrowserDownloadURL)
	if err != nil {
		log.Fatalf("failed to get binary: %v", err)
	}
	defer binaryResp.Body.Close()

	attestationResp, err := http.Get(attestationAsset.BrowserDownloadURL)
	if err != nil {
		log.Fatalf("failed to get attestation: %v", err)
	}
	defer attestationResp.Body.Close()

	// Read the bytes
	binary, err := io.ReadAll(binaryResp.Body)
	if err != nil {
		log.Fatalf("failed to read binary response: %v", err)
	}
	attestation, err := io.ReadAll(attestationResp.Body)
	if err != nil {
		log.Fatalf("failed to read attestation response: %v", err)
	}

	// Parse the attestation bundle json
	var attestationBundle bundle.Bundle
	attestationBundle.Bundle = new(protobundle.Bundle)
	err = attestationBundle.UnmarshalJSON(attestation)
	if err != nil {
		log.Fatalf("failed to unmarshal attestation bundle: %v", err)
	}

	// Create a certificate identity
	certIdent, err := verify.NewShortCertificateIdentity(issuer, "", "", sanRegex)
	if err != nil {
		log.Fatalf("failed to create certificate identity: %v", err)
	}

	// Fetch the sigstore trusted root via TUF
	tufOpts := tuf.DefaultOptions()
	tufOpts.RepositoryBaseURL = tufRepo
	tufFetcher := fetcher.DefaultFetcher{}
	tufFetcher.SetHTTPUserAgent(userAgent)
	tufOpts.Fetcher = &tufFetcher
	tufClient, err := tuf.New(tufOpts)
	if err != nil {
		log.Fatalf("failed to create TUF client: %v", err)
	}

	trustedRootJSON, err := tufClient.GetTarget(trustedRoot)

	var trustedRoot *root.TrustedRoot
	trustedRoot, err = root.NewTrustedRootFromJSON(trustedRootJSON)
	if err != nil {
		log.Fatalf("failed to create trusted root: %v", err)
	}

	// Create a signed entity verifier
	sev, err := verify.NewSignedEntityVerifier(trustedRoot, verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1))
	if err != nil {
		log.Fatalf("failed to create signed entity verifier: %v", err)
	}

	// Verify the signed entity
	artifactPolicy := verify.WithArtifact(bytes.NewReader(binary))
	res, err := sev.Verify(&attestationBundle, verify.NewPolicy(artifactPolicy, verify.WithCertificateIdentity(certIdent)))
	if err != nil {
		log.Fatalf("failed to verify signed entity: %v", err)
	}

	// Write files to disk
	verification, err := json.Marshal(res)
	if err != nil {
		log.Fatalf("failed to marshal verification: %v", err)
	}

	err = os.WriteFile(verificationPath, verification, 0444)
	if err != nil {
		log.Fatalf("failed to write verification to disk: %v", err)
	}
	err = os.WriteFile(filePath, binary, 0555)
	if err != nil {
		log.Fatalf("failed to write binary to disk: %v", err)
	}

	// Run the binary with the exec syscall, completely replacing the bootloader
	// This copies the current environment variables
	err = syscall.Exec(filePath, []string{}, os.Environ())

	// If we reach this point, the exec syscall failed
	log.Fatalf("failed to exec binary: %v", err)
}
