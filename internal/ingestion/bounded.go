package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

const boundedReadChunkSize = 32 << 10

var (
	// ErrLimitExceeded classifies an input or output outside its effective policy.
	ErrLimitExceeded = errors.New("ingestion limit exceeded")
	// ErrIntegrityFailure classifies malformed sizes, partial reads, and other
	// conditions where exact accepted bytes cannot be established.
	ErrIntegrityFailure = errors.New("ingestion integrity failure")
	// ErrPolicyViolation classifies an invalid effective limit or policy value.
	ErrPolicyViolation = errors.New("ingestion policy violation")
)

// OwnedBytes is an exact, independently owned byte value and its canonical
// content identity.
type OwnedBytes struct {
	Bytes      []byte
	ByteLength int64
	Identity   string
}

// Clone returns an independently owned copy.
func (b OwnedBytes) Clone() OwnedBytes {
	clone := make([]byte, len(b.Bytes))
	copy(clone, b.Bytes)
	return OwnedBytes{Bytes: clone, ByteLength: b.ByteLength, Identity: b.Identity}
}

// Identity returns the canonical SHA-256 identity over exact bytes.
func Identity(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// OwnBounded validates the exact byte length and returns an independent copy.
func OwnBounded(ctx context.Context, source []byte, limit int64) (OwnedBytes, error) {
	if err := validateByteLimit(limit); err != nil {
		return OwnedBytes{}, err
	}
	if err := Checkpoint(ctx); err != nil {
		return OwnedBytes{}, err
	}
	if int64(len(source)) > limit {
		return OwnedBytes{}, newClassifiedError(ErrLimitExceeded, nil)
	}
	owned := make([]byte, len(source))
	copy(owned, source)
	if err := Checkpoint(ctx); err != nil {
		return OwnedBytes{}, err
	}
	return sealOwned(owned), nil
}

// ReadBounded reads through EOF using at most a one-byte overflow probe. It
// never returns partial bytes after a limit, integrity, or cancellation error.
func ReadBounded(ctx context.Context, reader io.Reader, limit int64) (OwnedBytes, error) {
	if err := validateByteLimit(limit); err != nil {
		return OwnedBytes{}, err
	}
	if reader == nil {
		return OwnedBytes{}, newClassifiedError(ErrIntegrityFailure, nil)
	}

	initialCapacity := limit
	if initialCapacity > boundedReadChunkSize {
		initialCapacity = boundedReadChunkSize
	}
	accepted := make([]byte, 0, int(initialCapacity))
	buffer := make([]byte, boundedScratchCapacity(limit))

	for {
		if err := Checkpoint(ctx); err != nil {
			return OwnedBytes{}, err
		}

		remaining := limit - int64(len(accepted))
		readSize := int64(boundedReadChunkSize)
		if remaining < readSize-1 {
			readSize = remaining + 1
		}
		n, readErr := reader.Read(buffer[:int(readSize):int(readSize)])
		if err := Checkpoint(ctx); err != nil {
			return OwnedBytes{}, err
		}
		if n < 0 || n > int(readSize) {
			return OwnedBytes{}, newClassifiedError(ErrIntegrityFailure, nil)
		}
		if n > 0 {
			if int64(len(accepted)) > limit-int64(n) {
				return OwnedBytes{}, newClassifiedError(ErrLimitExceeded, nil)
			}
			accepted = appendAccepted(accepted, buffer[:n], limit)
		}

		switch {
		case readErr == io.EOF:
			return sealOwned(accepted), nil
		case errors.Is(readErr, context.Canceled) || errors.Is(readErr, context.DeadlineExceeded):
			return OwnedBytes{}, newClassifiedError(ErrCancellation, readErr)
		case readErr != nil:
			return OwnedBytes{}, newClassifiedError(ErrIntegrityFailure, readErr)
		case n == 0:
			return OwnedBytes{}, newClassifiedError(ErrIntegrityFailure, io.ErrNoProgress)
		}
	}
}

func boundedScratchCapacity(limit int64) int {
	if limit < boundedReadChunkSize {
		return int(limit) + 1
	}
	return boundedReadChunkSize
}

func validateByteLimit(limit int64) error {
	if limit < 0 {
		return newClassifiedError(ErrPolicyViolation, nil)
	}
	return nil
}

func sealOwned(data []byte) OwnedBytes {
	data = data[:len(data):len(data)]
	return OwnedBytes{
		Bytes:      data,
		ByteLength: int64(len(data)),
		Identity:   Identity(data),
	}
}

func appendAccepted(dst, source []byte, limit int64) []byte {
	newLength := len(dst) + len(source)
	if newLength <= cap(dst) {
		oldLength := len(dst)
		dst = dst[:newLength]
		copy(dst[oldLength:], source)
		return dst
	}

	newCapacity := int64(cap(dst)) * 2
	if newCapacity < int64(newLength) {
		newCapacity = int64(newLength)
	}
	if newCapacity > limit {
		newCapacity = limit
	}
	grown := make([]byte, newLength, int(newCapacity))
	copy(grown, dst)
	copy(grown[len(dst):], source)
	return grown
}
