// Implements the note.Signer interface for Ed25519 keys stored in GCP KMS.

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	kms "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
	"golang.org/x/mod/sumdb/note"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

const (
	algEd25519              = 1
	algECDSAWithSHA256      = 2
	algEd25519CosignatureV1 = 4
	algRFC6962STH           = 5
)

const (
	keyHashSize   = 4
	timestampSize = 8
)

type NoteKms struct {
	gcpclient  *kms.KeyManagementClient
	gcpkeyname string
	gcpctx     context.Context

	name string

	pubkey  ed25519.PublicKey
	keyhash uint32
}

// NewNoteKms constructs a new Signer that produces timestamped
// cosignature/v1 signatures from a Ed25519 key (EC_SIGN_ED25519) stored in GCP KMS.
func NewNoteKms(ctx context.Context, client *kms.KeyManagementClient, kmskeyname string, name string) (*NoteKms, error) {
	if !isValidName(name) {
		return nil, errors.New("invalid name")
	}

	n := &NoteKms{
		gcpclient:  client,
		gcpkeyname: kmskeyname,
		gcpctx:     ctx,
		name:       name,
	}

	pubkey, err := n.getPublicKey()
	if err != nil {
		return nil, err
	}

	n.pubkey = pubkey
	n.keyhash = keyHashEd25519(name, append([]byte{algEd25519CosignatureV1}, pubkey...))

	return n, nil

}

func (n *NoteKms) PublicKey() string {
	return fmt.Sprintf("%s+%08x+%s", n.name, n.keyhash,
		base64.StdEncoding.EncodeToString(
			// The algorithm byte is expected to be algEd25519, not algEd25519CosignatureV1.
			append([]byte{algEd25519}, n.pubkey...)))
			
}

func (n *NoteKms) Sign(msg []byte) ([]byte, error) {
	t := uint64(time.Now().Unix())
	m, err := formatCosignatureV1(t, msg)
	if err != nil {
		return nil, err
	}

	signature, err := n.signMsg(m)
	if err != nil {
		return nil, err
	}

	// The signature itself is encoded as timestamp || signature.
	sig := make([]byte, 0, timestampSize+ed25519.SignatureSize)
	sig = binary.BigEndian.AppendUint64(sig, t)
	sig = append(sig, signature...)
	return sig, nil
}

// https://github.com/transparency-dev/formats/blob/a07008fc07298aaf8d9d46ebd31b7031c4b4841d/note/note_cosigv1.go#L131
func (n *NoteKms) Verify(msg, sig []byte) bool {
	if len(sig) != timestampSize+ed25519.SignatureSize {
		return false
	}
	t := binary.BigEndian.Uint64(sig)
	sig = sig[timestampSize:]
	m, err := formatCosignatureV1(t, msg)
	if err != nil {
		return false
	}
	return ed25519.Verify(n.pubkey, m, sig)

}

// Helper methods

func (n *NoteKms) getPublicKey() (ed25519.PublicKey, error) {
	publicKey, err := n.gcpclient.GetPublicKey(n.gcpctx, &kmspb.GetPublicKeyRequest{
		Name: n.gcpkeyname,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	block, _ := pem.Decode([]byte(publicKey.Pem))
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	pubkey, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("invalid public key, not ed25519")
	}

	return pubkey, nil
}

func (n *NoteKms) signMsg(msg []byte) ([]byte, error) {
	// Build the signing request.
	//
	// Note: Key algorithms will require a varying hash function. For example,
	// EC_SIGN_P384_SHA384 requires SHA-384.
	req := &kmspb.AsymmetricSignRequest{
		Name:       n.gcpkeyname,
		Data:       msg,
		DataCrc32C: wrapperspb.Int64(int64(crc32c(msg))),
	}

	// Call the API.
	result, err := n.gcpclient.AsymmetricSign(n.gcpctx, req)
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

// Interface implementations
func (n *NoteKms) Name() string    { return n.name }
func (n *NoteKms) KeyHash() uint32 { return n.keyhash }

var _ note.Verifier = (*NoteKms)(nil)
var _ note.Signer = (*NoteKms)(nil)

/// Helper functions

// Calculate the CRC32C checksum of the data.
func crc32c(data []byte) uint32 {
	t := crc32.MakeTable(crc32.Castagnoli)
	return crc32.Checksum(data, t)
}

// https://github.com/transparency-dev/formats/blob/a07008fc07298aaf8d9d46ebd31b7031c4b4841d/note/note_cosigv1.go#L146
func formatCosignatureV1(t uint64, msg []byte) ([]byte, error) {
	// The signed message is in the following format
	//
	//      cosignature/v1
	//      time TTTTTTTTTT
	//      origin line
	//      NNNNNNNNN
	//      tree hash
	//      ...
	//
	// where TTTTTTTTTT is the current UNIX timestamp, and the following
	// lines are the lines of the note.
	//
	// While the witness signs all lines of the note, it's important to
	// understand that the witness is asserting observation of correct
	// append-only operation of the log based on the first three lines;
	// no semantic statement is made about any extra "extension" lines.

	if lines := bytes.Split(msg, []byte("\n")); len(lines) < 3 {
		return nil, errors.New("cosigned note format invalid")
	}
	return []byte(fmt.Sprintf("cosignature/v1\ntime %d\n%s", t, msg)), nil
}

// https://github.com/transparency-dev/formats/blob/a07008fc07298aaf8d9d46ebd31b7031c4b4841d/note/note_cosigv1.go#L199
// isValidName reports whether name is valid.
// It must be non-empty and not have any Unicode spaces or pluses.
func isValidName(name string) bool {
	return name != "" && utf8.ValidString(name) && strings.IndexFunc(name, unicode.IsSpace) < 0 && !strings.Contains(name, "+")
}

// https://github.com/transparency-dev/formats/blob/a07008fc07298aaf8d9d46ebd31b7031c4b4841d/note/note_cosigv1.go#L203
func keyHashEd25519(name string, key []byte) uint32 {
	h := sha256.New()
	h.Write([]byte(name))
	h.Write([]byte("\n"))
	h.Write(key)
	sum := h.Sum(nil)
	return binary.BigEndian.Uint32(sum)
}
