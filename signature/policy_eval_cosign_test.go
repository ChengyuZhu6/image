// Policy evaluation for prCosignSigned.

package signature

import (
	"context"
	"encoding/base64"
	"os"
	"testing"

	"github.com/containers/image/v5/internal/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRrCosignSignedIsSignatureAuthorAccepted(t *testing.T) {
	// Currently, this fails even with a correctly signed image.
	prm := NewPRMMatchRepository() // We prefer to test with a Cosign-created signature for interoperability, and that doesn’t work with matchExact.
	testImage := dirImageMock(t, "fixtures/dir-img-cosign-valid", "192.168.64.2:5000/cosign-signed-single-sample")
	testImageSigBlob, err := os.ReadFile("fixtures/dir-img-cosign-valid/signature-1")
	require.NoError(t, err)

	// Successful validation, with KeyData and KeyPath
	pr, err := newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	sar, parsedSig, err := pr.isSignatureAuthorAccepted(context.Background(), testImage, testImageSigBlob)
	assertSARRejected(t, sar, parsedSig, err)
}

// cosignSignatureFromFile returns a signature.Cosign loaded from path.
func cosignSignatureFromFile(t *testing.T, path string) signature.Cosign {
	blob, err := os.ReadFile(path)
	require.NoError(t, err)
	genericSig, err := signature.FromBlob(blob)
	require.NoError(t, err)
	sig, ok := genericSig.(signature.Cosign)
	require.True(t, ok)
	return sig
}

func TestPRrCosignSignedIsSignatureAccepted(t *testing.T) {
	assertAccepted := func(sar signatureAcceptanceResult, err error) {
		assert.Equal(t, sarAccepted, sar)
		assert.NoError(t, err)
	}
	assertRejected := func(sar signatureAcceptanceResult, err error) {
		assert.Equal(t, sarRejected, sar)
		assert.Error(t, err)
	}

	prm := NewPRMMatchRepository() // We prefer to test with a Cosign-created signature to ensure interoperability, and that doesn’t work with matchExact. matchExact is tested later.
	testImage := dirImageMock(t, "fixtures/dir-img-cosign-valid", "192.168.64.2:5000/cosign-signed-single-sample")
	testImageSig := cosignSignatureFromFile(t, "fixtures/dir-img-cosign-valid/signature-1")

	// Successful validation, with KeyData and KeyPath
	pr, err := newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	sar, err := pr.isSignatureAccepted(context.Background(), testImage, testImageSig)
	assertAccepted(sar, err)

	keyData, err := os.ReadFile("fixtures/cosign.pub")
	require.NoError(t, err)
	pr, err = newPRCosignSignedKeyData(keyData, prm)
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), testImage, testImageSig)
	assertAccepted(sar, err)

	// Both KeyPath and KeyData set. Do not use newPRCosignSigned*, because it would reject this.
	pr = &prCosignSigned{
		KeyPath:        "/foo/bar",
		KeyData:        []byte("abc"),
		SignedIdentity: prm,
	}
	// Pass nil and empty data to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil, testImageSig)
	assertRejected(sar, err)

	// Invalid KeyPath
	pr, err = newPRCosignSignedKeyPath("/this/does/not/exist", prm)
	require.NoError(t, err)
	// Pass nil and empty data to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil, testImageSig)
	assertRejected(sar, err)

	// KeyData doesn’t contain a public key.
	pr, err = newPRCosignSignedKeyData([]byte{}, prm)
	require.NoError(t, err)
	// Pass nil and empty data to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil, testImageSig)
	assertRejected(sar, err)

	// Signature has no cryptographic signature
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil,
		signature.CosignFromComponents(testImageSig.UntrustedMIMEType(), testImageSig.UntrustedPayload(), nil))
	assertRejected(sar, err)

	// A signature which does not verify
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil,
		signature.CosignFromComponents(testImageSig.UntrustedMIMEType(), testImageSig.UntrustedPayload(), map[string]string{
			signature.CosignSignatureAnnotationKey: base64.StdEncoding.EncodeToString([]byte("invalid signature")),
		}))
	assertRejected(sar, err)

	// A valid signature using an unknown key.
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	// Pass a nil pointer to, kind of, test that the return value does not depend on the image.
	sar, err = pr.isSignatureAccepted(context.Background(), nil, cosignSignatureFromFile(t, "fixtures/unknown-cosign-key.signature"))
	assertRejected(sar, err)

	// A valid signature with a rejected identity.
	nonmatchingPRM, err := NewPRMExactReference("this/doesnt:match")
	require.NoError(t, err)
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", nonmatchingPRM)
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), testImage, testImageSig)
	assertRejected(sar, err)

	// Error reading image manifest
	image := dirImageMock(t, "fixtures/dir-img-cosign-no-manifest", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), image, cosignSignatureFromFile(t, "fixtures/dir-img-cosign-no-manifest/signature-1"))
	assertRejected(sar, err)

	// Error computing manifest digest
	image = dirImageMock(t, "fixtures/dir-img-cosign-manifest-digest-error", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), image, cosignSignatureFromFile(t, "fixtures/dir-img-cosign-manifest-digest-error/signature-1"))
	assertRejected(sar, err)

	// A valid signature with a non-matching manifest
	image = dirImageMock(t, "fixtures/dir-img-cosign-modified-manifest", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), image, cosignSignatureFromFile(t, "fixtures/dir-img-cosign-modified-manifest/signature-1"))
	assertRejected(sar, err)

	// Minimally check that the prmMatchExact also works as expected:
	// - Signatures with a matching tag work
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-with-tag", "192.168.64.2:5000/skopeo-signed:tag")
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), image, cosignSignatureFromFile(t, "fixtures/dir-img-cosign-valid-with-tag/signature-1"))
	assertAccepted(sar, err)
	// - Signatures with a non-matching tag are rejected
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-with-tag", "192.168.64.2:5000/skopeo-signed:othertag")
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), image, cosignSignatureFromFile(t, "fixtures/dir-img-cosign-valid-with-tag/signature-1"))
	assertRejected(sar, err)
	// - Cosign-created signatures are rejected
	pr, err = newPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	sar, err = pr.isSignatureAccepted(context.Background(), testImage, testImageSig)
	assertRejected(sar, err)
}

func TestPRCosignSignedIsRunningImageAllowed(t *testing.T) {
	prm := NewPRMMatchRepository() // We prefer to test with a Cosign-created signature to ensure interoperability, and that doesn’t work with matchExact. matchExact is tested later.

	// A simple success case: single valid signature.
	image := dirImageMock(t, "fixtures/dir-img-cosign-valid", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err := NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err := pr.isRunningImageAllowed(context.Background(), image)
	assertRunningAllowed(t, allowed, err)

	// Error reading signatures
	invalidSigDir := createInvalidSigDir(t)
	image = dirImageMock(t, invalidSigDir, "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejected(t, allowed, err)

	// No signatures
	image = dirImageMock(t, "fixtures/dir-img-unsigned", "testing/manifest:latest")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejected(t, allowed, err)

	// Only non-Cosign signatures
	image = dirImageMock(t, "fixtures/dir-img-valid", "testing/manifest:latest")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejected(t, allowed, err)

	// Only non-signature Cosign attachments
	image = dirImageMock(t, "fixtures/dir-img-cosign-other-attachment", "testing/manifest:latest")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejected(t, allowed, err)

	// 1 invalid signature: use dir-img-valid, but a non-matching Docker reference
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid", "testing/manifest:notlatest")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)

	// 2 valid signatures
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-2", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningAllowed(t, allowed, err)

	// One invalid, one valid signature (in this order)
	image = dirImageMock(t, "fixtures/dir-img-cosign-mixed", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningAllowed(t, allowed, err)

	// 2 invalid signajtures: use dir-img-cosign-valid-2, but a non-matching Docker reference
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-2", "this/doesnt:match")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", prm)
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)

	// Minimally check that the prmMatchExact also works as expected:
	// - Signatures with a matching tag work
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-with-tag", "192.168.64.2:5000/skopeo-signed:tag")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningAllowed(t, allowed, err)
	// - Signatures with a non-matching tag are rejected
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid-with-tag", "192.168.64.2:5000/skopeo-signed:othertag")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)
	// - Cosign-created signatures are rejected
	image = dirImageMock(t, "fixtures/dir-img-cosign-valid", "192.168.64.2:5000/cosign-signed-single-sample")
	pr, err = NewPRCosignSignedKeyPath("fixtures/cosign.pub", NewPRMMatchExact())
	require.NoError(t, err)
	allowed, err = pr.isRunningImageAllowed(context.Background(), image)
	assertRunningRejectedPolicyRequirement(t, allowed, err)
}