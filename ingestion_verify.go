package inkbite

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

var (
	contentIdentityPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	artifactIDPattern      = regexp.MustCompile(`^artifact-[0-9]{6}$`)
)

const v1MaxByteLength int64 = 32 << 20

// VerificationCategory is a stable class of pure-verification finding.
type VerificationCategory string

const (
	VerificationContract     VerificationCategory = "contract"
	VerificationShape        VerificationCategory = "shape"
	VerificationPolicy       VerificationCategory = "policy"
	VerificationIntegrity    VerificationCategory = "integrity"
	VerificationOrdering     VerificationCategory = "ordering"
	VerificationDuplicate    VerificationCategory = "duplicate"
	VerificationReference    VerificationCategory = "reference"
	VerificationRelationship VerificationCategory = "relationship"
	VerificationWarning      VerificationCategory = "warning"
	VerificationProvenance   VerificationCategory = "provenance"
	VerificationOwnership    VerificationCategory = "ownership"
)

// VerificationFinding identifies one invalid envelope field without echoing
// attacker-controlled field contents.
type VerificationFinding struct {
	Category VerificationCategory `json:"category"`
	Path     string               `json:"path"`
	Detail   string               `json:"detail"`
}

// VerificationReport is the deterministic result of pure envelope checking.
type VerificationReport struct {
	Valid                      bool                  `json:"valid"`
	Findings                   []VerificationFinding `json:"findings"`
	VerifiedSourceIdentity     ContentIdentity       `json:"verified_source_identity,omitempty"`
	VerifiedArtifactIdentities []ContentIdentity     `json:"verified_artifact_identities"`
}

type envelopeVerifier struct {
	envelope IngestionEnvelope
	report   VerificationReport
	ids      map[string]string
}

// VerifyEnvelope validates one envelope using only the values passed to it.
// It performs no acquisition, conversion, network, component, clock, process,
// persistence, or cleanup work and does not mutate the envelope.
func VerifyEnvelope(envelope IngestionEnvelope) VerificationReport {
	v := envelopeVerifier{
		envelope: envelope,
		report: VerificationReport{
			Findings:                   make([]VerificationFinding, 0),
			VerifiedArtifactIdentities: make([]ContentIdentity, 0, 1+len(envelope.Artifacts)),
		},
		ids: make(map[string]string, 2+len(envelope.Artifacts)),
	}
	v.verify()
	v.report.Valid = len(v.report.Findings) == 0
	return v.report
}

func (v *envelopeVerifier) verify() {
	v.verifyContract()
	v.verifyPolicy()
	v.verifySource()
	v.verifyPrimary()
	v.verifyArtifacts()
	v.verifyProvenance()
	v.verifyRelationshipCoverage()
	v.verifyWarnings()
	v.verifyOwnership()
}

func (v *envelopeVerifier) add(category VerificationCategory, path, detail string) {
	v.report.Findings = append(v.report.Findings, VerificationFinding{
		Category: category,
		Path:     path,
		Detail:   detail,
	})
}

func (v *envelopeVerifier) verifyContract() {
	if v.envelope.ContractVersion != IngestionContractV1 {
		v.add(VerificationContract, "contract_version", "unsupported contract version")
	}
	if v.envelope.Provenance.ContractVersion != IngestionContractV1 ||
		v.envelope.Provenance.ContractVersion != v.envelope.ContractVersion {
		v.add(VerificationContract, "provenance.contract_version", "contract version does not match envelope")
	}
	if v.envelope.Artifacts == nil {
		v.add(VerificationShape, "artifacts", "required collection is null")
	}
	if v.envelope.Warnings == nil {
		v.add(VerificationShape, "warnings", "required collection is null")
	}
}

func (v *envelopeVerifier) verifyPolicy() {
	p := v.envelope.Provenance.Policy
	if p.MaxSourceBytes <= 0 || p.MaxPrimaryBytes <= 0 || p.MaxArtifacts < 0 ||
		p.MaxArtifactBytes <= 0 || p.MaxTotalArtifactBytes <= 0 ||
		p.MaxContainerEntries <= 0 || p.MaxContainerEntryBytes <= 0 ||
		p.MaxExpandedBytes <= 0 || p.MaxContainerDepth < 0 ||
		p.MaxExpansionRatio < 1 || math.IsNaN(p.MaxExpansionRatio) || math.IsInf(p.MaxExpansionRatio, 0) {
		v.add(VerificationPolicy, "provenance.policy", "effective policy contains an invalid limit")
	}
	if p.Component != "" && !safePublicText(p.Component) {
		v.add(VerificationPolicy, "provenance.policy.component", "component identity is not safe public metadata")
	}
}

func (v *envelopeVerifier) verifySource() {
	source := v.envelope.Source
	if source.Bytes == nil {
		v.add(VerificationShape, "source.bytes", "required bytes are null")
	}
	recomputed := identityFor(source.Bytes)
	v.report.VerifiedSourceIdentity = recomputed
	v.registerID(string(source.Identity), "source.identity")
	if !validIdentity(source.Identity) || source.Identity != recomputed {
		v.add(VerificationIntegrity, "source.identity", "source identity is noncanonical or does not match bytes")
	}
	if source.ByteLength != int64(len(source.Bytes)) {
		v.add(VerificationIntegrity, "source.byte_length", "source length does not match bytes")
	}
	if source.ByteLength > v.envelope.Provenance.Policy.MaxSourceBytes {
		v.add(VerificationPolicy, "source.byte_length", "source exceeds effective policy")
	}
	if source.ByteLength > v1MaxByteLength {
		v.add(VerificationPolicy, "source.byte_length", "source exceeds the v1 contract ceiling")
	}
	if !validSourceKind(source.SourceKind) {
		v.add(VerificationShape, "source.source_kind", "source kind is not a v1 value")
	}
	if source.MediaType != "" && !canonicalMediaType(source.MediaType) {
		v.add(VerificationShape, "source.media_type", "source media type is not canonical")
	}
	if source.SafeName != "" && !safeName(source.SafeName) {
		v.add(VerificationShape, "source.safe_name", "source name is not safe public metadata")
	}
}

func (v *envelopeVerifier) verifyPrimary() {
	primary := v.envelope.Primary
	if primary.ArtifactID != "artifact-000000" {
		v.add(VerificationShape, "primary.artifact_id", "primary artifact identifier is invalid")
	}
	if primary.Role != ArtifactRolePrimaryMarkdown {
		v.add(VerificationShape, "primary.role", "primary artifact role is invalid")
	}
	if primary.MediaType != "text/markdown" {
		v.add(VerificationShape, "primary.media_type", "primary artifact must be canonical Markdown")
	}
	if !utf8.Valid(primary.Bytes) {
		v.add(VerificationShape, "primary.bytes", "primary artifact is not valid UTF-8")
	}
	v.verifyArtifact(primary, "primary", v.envelope.Provenance.Policy.MaxPrimaryBytes)
}

func (v *envelopeVerifier) verifyArtifacts() {
	policy := v.envelope.Provenance.Policy
	if len(v.envelope.Artifacts) > DefaultMaxArtifacts {
		v.add(VerificationPolicy, "artifacts", "artifact count exceeds the v1 contract ceiling")
	}
	if len(v.envelope.Artifacts) > policy.MaxArtifacts {
		v.add(VerificationPolicy, "artifacts", "artifact count exceeds effective policy")
	}
	resolve := newEnvelopeArtifactReferenceResolver(v.envelope)
	previousKey := ""
	previousValid := false

	var total int64
	for i, artifact := range v.envelope.Artifacts {
		path := fmt.Sprintf("artifacts[%d]", i)
		key, keyValid := canonicalArtifactOrderKey(artifact, resolve)
		if !keyValid {
			v.add(VerificationOrdering, "artifacts", "artifact relationships cannot be resolved without positional identity")
		} else if previousValid {
			switch strings.Compare(previousKey, key) {
			case 1:
				v.add(VerificationOrdering, "artifacts", "derived artifacts are not in canonical order")
			case 0:
				v.add(VerificationDuplicate, "artifacts", "derived artifacts have ambiguous canonical identity")
			}
		}
		previousKey, previousValid = key, keyValid
		wantID := fmt.Sprintf("artifact-%06d", i+1)
		if artifact.ArtifactID != wantID {
			v.add(VerificationOrdering, path+".artifact_id", "artifact identifier does not match canonical position")
		}
		if artifact.Role == "" || artifact.Role == ArtifactRolePrimaryMarkdown || !safePublicText(string(artifact.Role)) {
			v.add(VerificationShape, path+".role", "derived artifact role is invalid")
		}
		v.verifyArtifact(artifact, path, policy.MaxArtifactBytes)
		total += artifact.ByteLength
	}
	if total < 0 || total > policy.MaxTotalArtifactBytes {
		v.add(VerificationPolicy, "artifacts", "aggregate artifact bytes exceed effective policy")
	}
}

func (v *envelopeVerifier) verifyArtifact(artifact ContentArtifact, path string, maxBytes int64) {
	v.registerID(artifact.ArtifactID, path+".artifact_id")
	if artifact.Bytes == nil {
		v.add(VerificationShape, path+".bytes", "required bytes are null")
	}
	recomputed := identityFor(artifact.Bytes)
	v.report.VerifiedArtifactIdentities = append(v.report.VerifiedArtifactIdentities, recomputed)
	if !validIdentity(artifact.Identity) || artifact.Identity != recomputed {
		v.add(VerificationIntegrity, path+".identity", "artifact identity is noncanonical or does not match bytes")
	}
	if artifact.ByteLength != int64(len(artifact.Bytes)) {
		v.add(VerificationIntegrity, path+".byte_length", "artifact length does not match bytes")
	}
	if artifact.ByteLength > maxBytes {
		v.add(VerificationPolicy, path+".byte_length", "artifact exceeds effective policy")
	}
	if artifact.ByteLength > v1MaxByteLength {
		v.add(VerificationPolicy, path+".byte_length", "artifact exceeds the v1 contract ceiling")
	}
	if !artifactIDPattern.MatchString(artifact.ArtifactID) {
		v.add(VerificationShape, path+".artifact_id", "artifact identifier is malformed")
	}
	if !canonicalMediaType(artifact.MediaType) {
		v.add(VerificationShape, path+".media_type", "artifact media type is not canonical")
	}
	if artifact.SafeName != "" && !safeName(artifact.SafeName) {
		v.add(VerificationShape, path+".safe_name", "artifact name is not safe public metadata")
	}
	if artifact.Relations == nil {
		v.add(VerificationShape, path+".relations", "required collection is null")
	} else {
		v.verifyRelations(artifact, path)
	}
	if artifact.Attributes == nil {
		v.add(VerificationShape, path+".attributes", "required collection is null")
	} else {
		v.verifyFacts(artifact.Attributes, path+".attributes")
	}
}

func (v *envelopeVerifier) verifyRelations(artifact ContentArtifact, path string) {
	if !slices.IsSortedFunc(artifact.Relations, compareRelations) {
		v.add(VerificationOrdering, path+".relations", "relations are not in canonical order")
	}
	seen := make(map[string]struct{}, len(artifact.Relations))
	for i, relation := range artifact.Relations {
		relationPath := fmt.Sprintf("%s.relations[%d]", path, i)
		key := relationKey(relation)
		if _, exists := seen[key]; exists {
			v.add(VerificationDuplicate, relationPath, "duplicate relationship")
		}
		seen[key] = struct{}{}
		if !validRelationKind(relation.Kind) {
			v.add(VerificationRelationship, relationPath+".kind", "relationship kind is invalid")
		}
		if _, exists := v.ids[relation.FromID]; !exists {
			// Artifact identifiers are registered before endpoint resolution in a
			// second pass below; defer non-source references until then.
			if relation.FromID != string(v.envelope.Source.Identity) && !artifactIDPattern.MatchString(relation.FromID) {
				v.add(VerificationReference, relationPath+".from_id", "relationship source does not resolve")
			}
		}
		if relation.ToID != artifact.ArtifactID {
			v.add(VerificationRelationship, relationPath+".to_id", "relationship target does not match owning artifact")
		}
		if relation.FromID == relation.ToID {
			v.add(VerificationRelationship, relationPath, "self relationship is not permitted")
		}
		if relation.Occurrence != "" && !safePublicText(relation.Occurrence) {
			v.add(VerificationRelationship, relationPath+".occurrence", "relationship occurrence is unsafe")
		}
	}
}

func (v *envelopeVerifier) verifyRelationshipCoverage() {
	outputs := make([]ContentArtifact, 0, 1+len(v.envelope.Artifacts))
	outputs = append(outputs, v.envelope.Primary)
	outputs = append(outputs, v.envelope.Artifacts...)
	for i, artifact := range outputs {
		hasValidNonSelf := false
		for _, relation := range artifact.Relations {
			_, fromExists := v.ids[relation.FromID]
			_, toExists := v.ids[relation.ToID]
			if validRelationKind(relation.Kind) && fromExists && toExists &&
				relation.FromID != artifact.ArtifactID && relation.ToID == artifact.ArtifactID {
				hasValidNonSelf = true
				break
			}
		}
		if !hasValidNonSelf {
			path := "primary.relations"
			if i > 0 {
				path = fmt.Sprintf("artifacts[%d].relations", i-1)
			}
			v.add(VerificationRelationship, path, "at least one valid non-self relationship is required")
		}
	}
}

func (v *envelopeVerifier) verifyProvenance() {
	p := v.envelope.Provenance
	if p.SourceIdentity != v.envelope.Source.Identity {
		v.add(VerificationIntegrity, "provenance.source_identity", "provenance source identity does not match source")
	}
	if !isSafeLabel(p.Converter) {
		v.add(VerificationProvenance, "provenance.converter", "converter identity is invalid")
	}
	if p.Backend != "" && !isSafeLabel(p.Backend) {
		v.add(VerificationProvenance, "provenance.backend", "backend identity is invalid")
	}
	if p.Component != "" && !safePublicText(p.Component) {
		v.add(VerificationProvenance, "provenance.component", "component identity is unsafe")
	}
	if p.Component != p.Policy.Component {
		v.add(VerificationProvenance, "provenance.component", "component does not match effective policy")
	}
	if p.StreamFacts == nil {
		v.add(VerificationShape, "provenance.stream_facts", "required collection is null")
	} else {
		v.verifyFacts(p.StreamFacts, "provenance.stream_facts")
	}
	want := make([]ContentIdentity, 0, 1+len(v.envelope.Artifacts))
	want = append(want, v.envelope.Primary.Identity)
	for _, artifact := range v.envelope.Artifacts {
		want = append(want, artifact.Identity)
	}
	if !slices.Equal(p.OutputIdentities, want) {
		v.add(VerificationIntegrity, "provenance.output_identities", "provenance outputs do not match envelope artifacts")
	}
	if len(p.Attempts) == 0 {
		v.add(VerificationShape, "provenance.attempts", "at least one conversion attempt is required")
	} else {
		selected := 0
		for i, attempt := range p.Attempts {
			path := fmt.Sprintf("provenance.attempts[%d]", i)
			if !isSafeLabel(attempt.Converter) ||
				(attempt.Category != "" && !safePublicText(attempt.Category)) {
				v.add(VerificationProvenance, path, "conversion attempt contains unsafe metadata")
			}
			switch attempt.Outcome {
			case AttemptUnsupported, AttemptFailed:
			case AttemptSelected:
				selected++
				if i != len(p.Attempts)-1 || attempt.Converter != p.Converter {
					v.add(VerificationProvenance, path, "selected attempt is not the winning converter")
				}
			default:
				v.add(VerificationProvenance, path+".outcome", "attempt outcome is invalid")
			}
		}
		if selected != 1 {
			v.add(VerificationProvenance, "provenance.attempts", "exactly one selected attempt is required")
		}
	}

	// Resolve artifact-shaped endpoints after all artifact IDs have been seen.
	for i, artifact := range append([]ContentArtifact{v.envelope.Primary}, v.envelope.Artifacts...) {
		for j, relation := range artifact.Relations {
			if _, exists := v.ids[relation.FromID]; !exists {
				v.add(VerificationReference, fmt.Sprintf("outputs[%d].relations[%d].from_id", i, j), "relationship source does not resolve")
			}
			if _, exists := v.ids[relation.ToID]; !exists {
				v.add(VerificationReference, fmt.Sprintf("outputs[%d].relations[%d].to_id", i, j), "relationship target does not resolve")
			}
		}
	}
}

func (v *envelopeVerifier) verifyWarnings() {
	if !slices.IsSortedFunc(v.envelope.Warnings, compareWarnings) {
		v.add(VerificationOrdering, "warnings", "warnings are not in canonical order")
	}
	for i, warning := range v.envelope.Warnings {
		path := fmt.Sprintf("warnings[%d]", i)
		if warning.Category == "" || !safePublicText(warning.Category) ||
			(warning.Converter != "" && !isSafeLabel(warning.Converter)) ||
			(warning.Location != "" && !safePublicText(warning.Location)) ||
			(warning.Detail != "" && !safePublicText(warning.Detail)) {
			v.add(VerificationWarning, path, "warning contains unsafe public metadata")
		}
	}
}

func (v *envelopeVerifier) verifyOwnership() {
	byteSlices := make([][]byte, 0, 2+len(v.envelope.Artifacts))
	byteSlices = append(byteSlices, v.envelope.Source.Bytes, v.envelope.Primary.Bytes)
	for _, artifact := range v.envelope.Artifacts {
		byteSlices = append(byteSlices, artifact.Bytes)
	}
	for i := range byteSlices {
		for j := i + 1; j < len(byteSlices); j++ {
			if storageRangesOverlap(byteSlices[i], byteSlices[j]) {
				v.add(VerificationOwnership, "bytes", "byte-bearing objects share storage")
			}
		}
	}
}

func storageRangesOverlap(a, b []byte) bool {
	if cap(a) == 0 || cap(b) == 0 {
		return false
	}
	aStart := uintptr(unsafe.Pointer(unsafe.SliceData(a)))
	bStart := uintptr(unsafe.Pointer(unsafe.SliceData(b)))
	aSize := uintptr(cap(a))
	bSize := uintptr(cap(b))
	aEnd := aStart + aSize
	bEnd := bStart + bSize
	if aStart == 0 || bStart == 0 || aEnd < aStart || bEnd < bStart ||
		int(aSize) != cap(a) || int(bSize) != cap(b) {
		return true
	}
	return aStart < bEnd && bStart < aEnd
}

func (v *envelopeVerifier) verifyFacts(facts []MetadataFact, path string) {
	if !slices.IsSortedFunc(facts, compareFacts) {
		v.add(VerificationOrdering, path, "metadata facts are not in canonical order")
	}
	seen := make(map[string]struct{}, len(facts))
	for i, fact := range facts {
		factPath := fmt.Sprintf("%s[%d]", path, i)
		key := fact.Kind + "\x00" + string(fact.Origin)
		if _, exists := seen[key]; exists {
			v.add(VerificationDuplicate, factPath, "duplicate metadata fact")
		}
		seen[key] = struct{}{}
		if !isSafeLabel(fact.Kind) || !validMetadataOrigin(fact.Origin) || !safePublicText(fact.Value) {
			v.add(VerificationShape, factPath, "metadata fact is invalid or unsafe")
		}
	}
}

func (v *envelopeVerifier) registerID(id, path string) {
	if previous, exists := v.ids[id]; exists {
		v.add(VerificationDuplicate, path, "identifier duplicates "+previous)
		return
	}
	v.ids[id] = path
}

func identityFor(data []byte) ContentIdentity {
	sum := sha256.Sum256(data)
	return ContentIdentity("sha256:" + hex.EncodeToString(sum[:]))
}

func validIdentity(identity ContentIdentity) bool {
	return contentIdentityPattern.MatchString(string(identity))
}

func validSourceKind(kind SourceKind) bool {
	switch kind {
	case SourceKindBytes, SourceKindReader, SourceKindFile, SourceKindDataURI, SourceKindRemote:
		return true
	default:
		return false
	}
}

func validRelationKind(kind RelationKind) bool {
	switch kind {
	case RelationDerivedFrom, RelationEmbeddedIn, RelationReferencedBy:
		return true
	default:
		return false
	}
}

func validMetadataOrigin(origin MetadataOrigin) bool {
	switch origin {
	case MetadataOriginCaller, MetadataOriginSource, MetadataOriginSniff, MetadataOriginConverter:
		return true
	default:
		return false
	}
}

func canonicalMediaType(value string) bool {
	mediaType, params, err := mime.ParseMediaType(value)
	return err == nil && len(params) == 0 && mediaType == value && strings.ToLower(value) == value
}

func safeName(value string) bool {
	if len(value) > 4096 {
		return false
	}
	return inspectPercentDecoded(value, func(current string) bool {
		return safePublicTextForm(current) && safeLogicalName(current)
	}, func(current, decoded string) bool {
		return strings.Count(decoded, "/") <= strings.Count(current, "/") &&
			strings.Count(decoded, "\\") <= strings.Count(current, "\\")
	})
}

func safeLogicalName(value string) bool {
	if value == "" || containsAbsolutePath(value) || strings.ContainsAny(value, "\\?#:") {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"data:", "authorization:", "proxy-authorization:", "bearer ", "basic "} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	return true
}

func isHex(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func safePublicText(value string) bool {
	if len(value) > 4096 {
		return false
	}
	return inspectPercentDecoded(value, safePublicTextForm, nil)
}

const maxPublicPercentDecodeRounds = 16

func inspectPercentDecoded(value string, inspect func(string) bool, transition func(string, string) bool) bool {
	current := value
	for round := 0; round <= maxPublicPercentDecodeRounds; round++ {
		if !inspect(current) {
			return false
		}
		decoded, changed, valid := decodePublicPercentEncoding(current)
		if !valid {
			return false
		}
		if !changed {
			return true
		}
		if round == maxPublicPercentDecodeRounds {
			return false
		}
		if transition != nil && !transition(current, decoded) {
			return false
		}
		current = decoded
	}
	return false
}

func decodePublicPercentEncoding(value string) (string, bool, bool) {
	var decoded strings.Builder
	decoded.Grow(len(value))
	changed := false
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			decoded.WriteByte(value[i])
			continue
		}
		if i+1 < len(value) && isHex(value[i+1]) && (i+2 >= len(value) || !isHex(value[i+2])) {
			return "", false, false
		}
		if i+2 < len(value) && isHex(value[i+1]) && isHex(value[i+2]) {
			decoded.WriteByte(fromHex(value[i+1])<<4 | fromHex(value[i+2]))
			i += 2
			changed = true
			continue
		}
		decoded.WriteByte(value[i])
	}
	return decoded.String(), changed, true
}

func fromHex(value byte) byte {
	switch {
	case value >= '0' && value <= '9':
		return value - '0'
	case value >= 'a' && value <= 'f':
		return value - 'a' + 10
	default:
		return value - 'A' + 10
	}
}

func safePublicTextForm(value string) bool {
	if !utf8.ValidString(value) || containsPathTraversal(value) {
		return false
	}
	lower := strings.ToLower(value)
	for _, forbidden := range []string{"data:", "authorization:", "proxy-authorization:", "bearer ", "basic "} {
		if strings.Contains(lower, forbidden) {
			return false
		}
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	if strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") {
		parsed, err := url.Parse(value)
		return err == nil && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" &&
			!parsed.ForceQuery && parsed.Fragment == "" && parsed.Opaque == "" && !strings.Contains(value, "#")
	}
	if strings.HasPrefix(lower, "file:") || strings.ContainsAny(value, "?#") {
		return false
	}
	return !containsAbsolutePath(value)
}

func containsPathTraversal(value string) bool {
	normalized := strings.ReplaceAll(value, "\\", "/")
	for _, segment := range strings.FieldsFunc(normalized, func(r rune) bool {
		return r == '/' || unicode.IsSpace(r) || strings.ContainsRune(`"'()[]{}<>,;=`, r)
	}) {
		if segment == ".." {
			return true
		}
	}
	return false
}

func containsAbsolutePath(value string) bool {
	if filepath.IsAbs(value) || portableAbsolutePathToken(value) {
		return true
	}
	for _, token := range strings.FieldsFunc(value, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(`:"'()[]{}<>,;=`, r)
	}) {
		if portableAbsolutePathToken(token) {
			return true
		}
	}
	return false
}

func portableAbsolutePathToken(value string) bool {
	value = strings.TrimLeft(value, "`~")
	if value == "" {
		return false
	}
	if filepath.IsAbs(value) || value[0] == '/' || value[0] == '\\' {
		return true
	}
	return len(value) >= 3 &&
		((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) &&
		value[1] == ':' && (value[2] == '/' || value[2] == '\\')
}

func compareFacts(a, b MetadataFact) int {
	return strings.Compare(a.Kind+"\x00"+string(a.Origin)+"\x00"+a.Value, b.Kind+"\x00"+string(b.Origin)+"\x00"+b.Value)
}

func relationKey(relation ArtifactRelation) string {
	return relation.FromID + "\x00" + relation.ToID + "\x00" + string(relation.Kind) + "\x00" + relation.Occurrence
}

func compareRelations(a, b ArtifactRelation) int {
	return strings.Compare(relationKey(a), relationKey(b))
}

func newEnvelopeArtifactReferenceResolver(envelope IngestionEnvelope) artifactReferenceResolver {
	type referenceValue struct {
		semantic string
		count    int
	}
	references := make(map[string]referenceValue, 2+len(envelope.Artifacts))
	semanticCounts := make(map[string]int, 1+len(envelope.Artifacts))
	add := func(id, semantic string) {
		value := references[id]
		value.semantic = semantic
		value.count++
		references[id] = value
		semanticCounts[semantic]++
	}
	add(string(envelope.Source.Identity), canonicalTuple("source", string(envelope.Source.Identity)))
	add(envelope.Primary.ArtifactID, canonicalArtifactReferenceKey(envelope.Primary))
	for _, artifact := range envelope.Artifacts {
		add(artifact.ArtifactID, canonicalArtifactReferenceKey(artifact))
	}
	return func(reference string) (string, bool) {
		value, exists := references[reference]
		return value.semantic, exists && value.count == 1 && semanticCounts[value.semantic] == 1
	}
}

func warningKey(warning WarningRecord) string {
	return warning.Category + "\x00" + warning.Converter + "\x00" + warning.Location + "\x00" + warning.Detail
}

func compareWarnings(a, b WarningRecord) int {
	return strings.Compare(warningKey(a), warningKey(b))
}
