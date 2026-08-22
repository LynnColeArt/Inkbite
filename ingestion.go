package inkbite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"

	internalingestion "github.com/LynnColeArt/Inkbite/internal/ingestion"
	"github.com/LynnColeArt/Inkbite/internal/normalize"
)

// IngestOptions carries the detailed-ingestion policy and the existing
// converter options. A completely zero value materializes the documented
// default policy. Remote authority is always taken from Policy.
type IngestOptions struct {
	Policy         IngestionPolicy
	ConvertOptions ConvertOptions
}

type ingestionPipelineResult struct {
	envelope IngestionEnvelope
	legacy   Result
}

type selectedConversion struct {
	value                     DetailedConversion
	legacy                    Result
	winner                    string
	attempts                  []ConversionAttempt
	warnings                  []WarningRecord
	rewriteArtifactReferences bool
}

type sealedArtifactDraft struct {
	role       ArtifactRole
	bytes      []byte
	identity   ContentIdentity
	mediaType  string
	safeName   string
	occurrence string
	attributes []MetadataFact
	orderKey   string
	rawIndex   int
}

type artifactReferenceResolver func(string) (string, bool)

type ingestionDispatchIntent uint8

const (
	ingestionDispatchLegacy ingestionDispatchIntent = iota
	ingestionDispatchDetailed
)

// Ingest executes the versioned detailed-ingestion contract through the same
// source, registry, conversion, normalization, and verification pipeline used
// by the legacy conversion methods.
func (e *Engine) Ingest(
	ctx context.Context,
	source any,
	hints *StreamInfo,
	options IngestOptions,
) (IngestionEnvelope, error) {
	policy, err := materializeIngestionPolicy(options.Policy)
	if err != nil {
		return IngestionEnvelope{}, err
	}
	convertOptions := options.ConvertOptions
	convertOptions.EnableHTTP = policy.Remote.Enabled
	if convertOptions.MaxHTTPBytes <= 0 || convertOptions.MaxHTTPBytes > policy.MaxSourceBytes {
		convertOptions.MaxHTTPBytes = policy.MaxSourceBytes
	}
	result, err := e.runIngestionPipeline(ctx, source, hints, convertOptions, policy, ingestionDispatchDetailed)
	if err != nil {
		return IngestionEnvelope{}, err
	}
	return result.envelope, nil
}

func (e *Engine) runIngestionPipeline(
	ctx context.Context,
	source any,
	hints *StreamInfo,
	convertOptions ConvertOptions,
	policy IngestionPolicy,
	intent ingestionDispatchIntent,
) (ingestionPipelineResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	policy, err := materializeIngestionPolicy(policy)
	if err != nil {
		return ingestionPipelineResult{}, err
	}
	pipelineBudget, err := internalingestion.NewRequestBudget(internalLimits(policy))
	if err != nil {
		return ingestionPipelineResult{}, publicPipelineError("policy", err)
	}
	containerBudget := pipelineBudget
	conversionPolicy := policy
	if existing, ok := internalingestion.RequestBudgetFromContext(ctx); ok {
		containerBudget = existing
		conversionPolicy = withContainerLimits(conversionPolicy, existing.Limits())
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return ingestionPipelineResult{}, publicPipelineError("ingest-start", err)
	}

	var resolved resolvedSource
	if policy.MaxSourceBytes == DefaultMaxSourceBytes {
		// Preserve the established default acquisition seam; it delegates to the
		// same bounded authority used for an explicit non-default policy below.
		resolved, err = e.resolveSource(ctx, source, hints, convertOptions)
	} else {
		resolved, err = e.acquireSource(ctx, source, hints, convertOptions, policy.MaxSourceBytes)
	}
	if err != nil {
		return ingestionPipelineResult{}, err
	}
	if err := pipelineBudget.AdmitSource(resolved.owned.ByteLength); err != nil {
		return ingestionPipelineResult{}, publicPipelineError("source-budget", err)
	}
	enriched, sniffed, err := enrichStreamInfoWithFacts(resolved.reader, resolved.info)
	if err != nil {
		return ingestionPipelineResult{}, publicPipelineError("source-sniff", err)
	}
	resolved.facts = append(resolved.facts, safeStreamFacts(StreamInfo{}, StreamInfo{}, sniffed)...)

	conversionCtx, err := internalingestion.WithRequestBudget(ctx, containerBudget)
	if err != nil {
		return ingestionPipelineResult{}, publicPipelineError("request-budget", err)
	}
	selected, err := e.dispatchIngestion(conversionCtx, resolved.reader, enriched, convertOptions, conversionPolicy, intent)
	if err != nil {
		return ingestionPipelineResult{}, err
	}
	sealed, err := sealIngestionEnvelope(ctx, resolved, selected, conversionPolicy, pipelineBudget)
	if err != nil {
		return ingestionPipelineResult{}, err
	}
	return ingestionPipelineResult{envelope: sealed, legacy: selected.legacy}, nil
}

func withContainerLimits(policy IngestionPolicy, limits internalingestion.Limits) IngestionPolicy {
	policy.MaxContainerEntries = limits.MaxContainerEntries
	policy.MaxContainerEntryBytes = limits.MaxContainerEntryBytes
	policy.MaxExpandedBytes = limits.MaxExpandedBytes
	policy.MaxContainerDepth = limits.MaxContainerDepth
	policy.MaxExpansionRatio = limits.MaxExpansionRatio
	return policy
}

func (e *Engine) dispatchIngestion(
	ctx context.Context,
	reader io.ReadSeeker,
	info StreamInfo,
	options ConvertOptions,
	policy IngestionPolicy,
	intent ingestionDispatchIntent,
) (selectedConversion, error) {
	attempts := make([]ConversionAttempt, 0, len(e.converters))
	warnings := make([]WarningRecord, 0)
	failures := make([]ConversionError, 0)

	for _, converter := range e.RegisteredConverters() {
		name := converter.Name()
		if err := checkpointAndReset(ctx, reader); err != nil {
			return selectedConversion{}, publicPipelineError("converter-reset", err)
		}
		if !converter.Accepts(ctx, reader, info, options) {
			if err := internalingestion.Checkpoint(ctx); err != nil {
				return selectedConversion{}, publicPipelineError("converter-accept", err)
			}
			attempts = append(attempts, ConversionAttempt{Converter: name, Outcome: AttemptUnsupported})
			continue
		}
		if err := checkpointAndReset(ctx, reader); err != nil {
			return selectedConversion{}, publicPipelineError("converter-reset", err)
		}

		var detailed DetailedConversion
		var legacy Result
		var err error
		rewriteArtifactReferences := false
		if converterWithDetail, ok := converter.(DetailedConverter); ok && intent == ingestionDispatchDetailed {
			detailed, err = converterWithDetail.ConvertDetailed(ctx, reader, info, options, policy)
			legacy = detailed.Result
			rewriteArtifactReferences = true
		} else {
			legacy, err = converter.Convert(ctx, reader, info, options)
			detailed = DetailedConversion{Result: legacy}
		}
		if checkpointErr := internalingestion.Checkpoint(ctx); checkpointErr != nil {
			return selectedConversion{}, publicPipelineError("converter-run", checkpointErr)
		}
		if err != nil {
			if errors.Is(err, ErrUnsupportedFormat) {
				attempts = append(attempts, ConversionAttempt{Converter: name, Outcome: AttemptUnsupported})
				continue
			}
			if terminalConverterError(err) {
				return selectedConversion{}, publicPipelineError("converter-run", err)
			}
			attempts = append(attempts, ConversionAttempt{Converter: name, Outcome: AttemptFailed, Category: "converter"})
			failures = append(failures, ConversionError{Converter: name, Err: err})
			warnings = append(warnings, WarningRecord{
				Category:  "converter_fallback",
				Converter: name,
				Detail:    "converter failure",
			})
			continue
		}

		legacy.Markdown = normalize.Markdown(legacy.Markdown, options.KeepDataURIs)
		detailed.Result = legacy
		attempts = append(attempts, ConversionAttempt{Converter: name, Outcome: AttemptSelected})
		return selectedConversion{
			value:                     detailed,
			legacy:                    legacy,
			winner:                    name,
			attempts:                  attempts,
			warnings:                  warnings,
			rewriteArtifactReferences: rewriteArtifactReferences,
		}, nil
	}

	if len(failures) > 0 {
		return selectedConversion{}, FailedAttemptsError{Attempts: failures}
	}
	return selectedConversion{}, UnsupportedFormatError{Info: info}
}

func checkpointAndReset(ctx context.Context, reader io.ReadSeeker) error {
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return err
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return internalingestion.Checkpoint(ctx)
}

func sealIngestionEnvelope(
	ctx context.Context,
	resolved resolvedSource,
	selected selectedConversion,
	policy IngestionPolicy,
	budget *internalingestion.RequestBudget,
) (IngestionEnvelope, error) {
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return IngestionEnvelope{}, publicPipelineError("seal-start", err)
	}
	if selected.value.Component != "" && selected.value.Component != policy.Component {
		return IngestionEnvelope{}, publicPipelineError("seal-component", ErrIntegrityFailure)
	}

	sourceBytes := cloneExactBytes(resolved.owned.Bytes)
	sourceIdentity := ContentIdentity(internalingestion.Identity(sourceBytes))

	drafts := make([]sealedArtifactDraft, 0, len(selected.value.Artifacts))
	for rawIndex, raw := range selected.value.Artifacts {
		owned := cloneExactBytes(raw.Bytes)
		if err := budget.AdmitArtifact(int64(len(owned))); err != nil {
			return IngestionEnvelope{}, publicPipelineError("artifact-budget", err)
		}
		attributes, err := canonicalArtifactFacts(raw.Attributes)
		if err != nil {
			return IngestionEnvelope{}, err
		}
		draft := sealedArtifactDraft{
			role:       raw.Role,
			bytes:      owned,
			identity:   ContentIdentity(internalingestion.Identity(owned)),
			mediaType:  raw.MediaType,
			safeName:   raw.SafeName,
			occurrence: raw.Occurrence,
			attributes: attributes,
			rawIndex:   rawIndex,
		}
		orderingArtifact := draft.contentArtifact("", sourceIdentity)
		orderKey, ok := canonicalArtifactOrderKey(orderingArtifact, func(reference string) (string, bool) {
			if reference == string(sourceIdentity) {
				return canonicalTuple("source", reference), true
			}
			return "", false
		})
		if !ok {
			return IngestionEnvelope{}, publicPipelineError("artifact-order", ErrIntegrityFailure)
		}
		draft.orderKey = orderKey
		drafts = append(drafts, draft)
	}
	sort.SliceStable(drafts, func(i, j int) bool {
		return drafts[i].orderKey < drafts[j].orderKey
	})
	for index := 1; index < len(drafts); index++ {
		if drafts[index-1].orderKey == drafts[index].orderKey {
			return IngestionEnvelope{}, publicPipelineError("artifact-order", ErrIntegrityFailure)
		}
	}
	artifacts := make([]ContentArtifact, 0, len(drafts))
	finalArtifactIDs := make([]string, len(drafts))
	for index, draft := range drafts {
		artifactID := fmt.Sprintf("artifact-%06d", index+1)
		finalArtifactIDs[draft.rawIndex] = artifactID
		artifacts = append(artifacts, draft.contentArtifact(artifactID, sourceIdentity))
	}
	primaryMarkdown := selected.legacy.Markdown
	if selected.rewriteArtifactReferences {
		var err error
		primaryMarkdown, err = rewriteDetailedArtifactReferences(primaryMarkdown, finalArtifactIDs)
		if err != nil {
			return IngestionEnvelope{}, err
		}
	}
	primaryBytes := cloneExactBytes([]byte(primaryMarkdown))
	if err := budget.AdmitPrimary(int64(len(primaryBytes))); err != nil {
		return IngestionEnvelope{}, publicPipelineError("primary-budget", err)
	}
	primaryIdentity := ContentIdentity(internalingestion.Identity(primaryBytes))
	primary := ContentArtifact{
		ArtifactID: "artifact-000000",
		Role:       ArtifactRolePrimaryMarkdown,
		Bytes:      primaryBytes,
		Identity:   primaryIdentity,
		ByteLength: int64(len(primaryBytes)),
		MediaType:  "text/markdown",
		Relations: []ArtifactRelation{{
			Kind:   RelationDerivedFrom,
			FromID: string(sourceIdentity),
			ToID:   "artifact-000000",
		}},
		Attributes: make([]MetadataFact, 0),
	}

	facts, err := canonicalStreamFacts(resolved.facts, selected.value.Facts)
	if err != nil {
		return IngestionEnvelope{}, err
	}
	warnings := append(slices.Clone(selected.warnings), slices.Clone(selected.value.Warnings)...)
	sort.SliceStable(warnings, func(i, j int) bool { return compareWarnings(warnings[i], warnings[j]) < 0 })
	outputIdentities := make([]ContentIdentity, 0, 1+len(artifacts))
	outputIdentities = append(outputIdentities, primaryIdentity)
	for _, artifact := range artifacts {
		outputIdentities = append(outputIdentities, artifact.Identity)
	}

	envelope := IngestionEnvelope{
		ContractVersion: IngestionContractV1,
		Source: SourceArtifact{
			Bytes:      sourceBytes,
			Identity:   sourceIdentity,
			ByteLength: int64(len(sourceBytes)),
			MediaType:  resolved.info.MIMEType,
			SourceKind: resolved.kind,
			SafeName:   resolved.info.Filename,
		},
		Primary:   primary,
		Artifacts: artifacts,
		Provenance: ConversionProvenance{
			ContractVersion:  IngestionContractV1,
			SourceIdentity:   sourceIdentity,
			Converter:        selected.winner,
			Backend:          selected.value.Backend,
			Component:        policy.Component,
			StreamFacts:      facts,
			Policy:           policy,
			OutputIdentities: outputIdentities,
			Attempts:         slices.Clone(selected.attempts),
		},
		Warnings: warnings,
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return IngestionEnvelope{}, publicPipelineError("seal-finish", err)
	}
	report := VerifyEnvelope(envelope)
	if !report.Valid {
		return IngestionEnvelope{}, publicPipelineError("envelope-verify", ErrIntegrityFailure)
	}
	if err := internalingestion.Checkpoint(ctx); err != nil {
		return IngestionEnvelope{}, publicPipelineError("ingest-finish", err)
	}
	return envelope, nil
}

const detailedArtifactReferencePrefix = "inkbite-artifact:"

// rewriteDetailedArtifactReferences resolves converter-local artifact ordinals
// only after the engine has assigned IDs in its canonical final order.
func rewriteDetailedArtifactReferences(markdown string, finalArtifactIDs []string) (string, error) {
	var rewritten strings.Builder
	for cursor := 0; cursor < len(markdown); {
		relativePrefixIndex := strings.Index(markdown[cursor:], detailedArtifactReferencePrefix)
		if relativePrefixIndex < 0 {
			rewritten.WriteString(markdown[cursor:])
			return rewritten.String(), nil
		}
		prefixIndex := cursor + relativePrefixIndex
		prefixEnd := prefixIndex + len(detailedArtifactReferencePrefix)
		if prefixIndex > 0 && !artifactReferenceOpener(markdown[prefixIndex-1]) {
			rewritten.WriteString(markdown[cursor:prefixEnd])
			cursor = prefixEnd
			continue
		}
		rewritten.WriteString(markdown[cursor:prefixEnd])
		remaining := markdown[prefixEnd:]
		const artifactIDLength = len("artifact-000000")
		if len(remaining) < artifactIDLength {
			return "", publicPipelineError("artifact-reference", ErrIntegrityFailure)
		}
		provisionalID := remaining[:artifactIDLength]
		if !artifactIDPattern.MatchString(provisionalID) ||
			(len(remaining) > artifactIDLength && !artifactReferenceDelimiter(remaining[artifactIDLength])) {
			return "", publicPipelineError("artifact-reference", ErrIntegrityFailure)
		}
		ordinal, err := strconv.Atoi(provisionalID[len("artifact-"):])
		if err != nil || ordinal < 1 || ordinal > len(finalArtifactIDs) || finalArtifactIDs[ordinal-1] == "" {
			return "", publicPipelineError("artifact-reference", ErrIntegrityFailure)
		}
		rewritten.WriteString(finalArtifactIDs[ordinal-1])
		cursor = prefixEnd + artifactIDLength
	}
	return rewritten.String(), nil
}

func artifactReferenceOpener(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', '(', '[', '<', '\'', '"':
		return true
	default:
		return false
	}
}

func artifactReferenceDelimiter(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r', ')', ']', '>', '\'', '"':
		return true
	default:
		return false
	}
}

func (draft sealedArtifactDraft) contentArtifact(artifactID string, sourceIdentity ContentIdentity) ContentArtifact {
	return ContentArtifact{
		ArtifactID: artifactID,
		Role:       draft.role,
		Bytes:      draft.bytes,
		Identity:   draft.identity,
		ByteLength: int64(len(draft.bytes)),
		MediaType:  draft.mediaType,
		SafeName:   draft.safeName,
		Relations: []ArtifactRelation{{
			Kind:       RelationDerivedFrom,
			FromID:     string(sourceIdentity),
			ToID:       artifactID,
			Occurrence: draft.occurrence,
		}},
		Attributes: draft.attributes,
	}
}

// cloneExactBytes returns independently owned, present bytes with no writable
// capacity beyond the accounted value. A present empty input remains non-nil.
func cloneExactBytes(data []byte) []byte {
	if data == nil {
		return nil
	}
	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned[:len(cloned):len(cloned)]
}

// canonicalArtifactOrderKey covers every retained semantic field except the
// positional ArtifactID. Relationship endpoints are translated through the
// supplied semantic resolver; an artifact's own target resolves directly to
// its complete non-relational reference value.
func canonicalArtifactOrderKey(artifact ContentArtifact, resolve artifactReferenceResolver) (string, bool) {
	self := canonicalArtifactReferenceKey(artifact)
	relationKeys := make([]string, 0, len(artifact.Relations))
	for _, relation := range artifact.Relations {
		from, ok := canonicalRelationEndpoint(relation.FromID, artifact.ArtifactID, self, resolve)
		if !ok {
			return "", false
		}
		to, ok := canonicalRelationEndpoint(relation.ToID, artifact.ArtifactID, self, resolve)
		if !ok {
			return "", false
		}
		relationKeys = append(relationKeys, canonicalTuple(from, to, string(relation.Kind), relation.Occurrence))
	}
	sort.Strings(relationKeys)
	return canonicalTuple(
		canonicalTuple(relationKeys...),
		string(artifact.Role),
		artifact.MediaType,
		artifact.SafeName,
		string(artifact.Identity),
		strconv.FormatInt(artifact.ByteLength, 10),
		canonicalArtifactAttributesKey(artifact.Attributes),
	), true
}

func canonicalRelationEndpoint(reference, selfID, self string, resolve artifactReferenceResolver) (string, bool) {
	if reference == selfID {
		return self, true
	}
	if resolve == nil {
		return "", false
	}
	return resolve(reference)
}

func canonicalArtifactReferenceKey(artifact ContentArtifact) string {
	return canonicalTuple(
		"artifact",
		string(artifact.Role),
		artifact.MediaType,
		artifact.SafeName,
		string(artifact.Identity),
		strconv.FormatInt(artifact.ByteLength, 10),
		canonicalArtifactAttributesKey(artifact.Attributes),
	)
}

func canonicalArtifactAttributesKey(attributes []MetadataFact) string {
	canonical := slices.Clone(attributes)
	sort.Slice(canonical, func(i, j int) bool { return compareFacts(canonical[i], canonical[j]) < 0 })
	parts := make([]string, 0, len(canonical))
	for _, attribute := range canonical {
		parts = append(parts, canonicalTuple(attribute.Kind, string(attribute.Origin), attribute.Value))
	}
	return canonicalTuple(parts...)
}

func canonicalTuple(parts ...string) string {
	var builder strings.Builder
	for _, part := range parts {
		builder.WriteString(strconv.Itoa(len(part)))
		builder.WriteByte(':')
		builder.WriteString(part)
	}
	return builder.String()
}

func canonicalStreamFacts(source []internalingestion.Fact, converter []MetadataFact) ([]MetadataFact, error) {
	facts := make([]MetadataFact, 0, len(source)+len(converter))
	for _, fact := range source {
		facts = append(facts, MetadataFact{Kind: fact.Kind, Value: fact.Value, Origin: publicFactOrigin(fact.Origin)})
	}
	converted, err := canonicalConverterFacts(converter)
	if err != nil {
		return nil, err
	}
	facts = append(facts, converted...)
	sort.SliceStable(facts, func(i, j int) bool { return compareFacts(facts[i], facts[j]) < 0 })
	for i := 1; i < len(facts); i++ {
		if facts[i-1].Kind == facts[i].Kind && facts[i-1].Origin == facts[i].Origin {
			return nil, publicPipelineError("seal-facts", ErrIntegrityFailure)
		}
	}
	return facts, nil
}

func canonicalConverterFacts(raw []MetadataFact) ([]MetadataFact, error) {
	facts := make([]MetadataFact, 0, len(raw))
	for _, fact := range raw {
		if fact.Origin != MetadataOriginConverter {
			return nil, publicPipelineError("seal-facts", ErrIntegrityFailure)
		}
		canonical, err := internalingestion.NewFact(fact.Kind, fact.Value, internalingestion.OriginConverter)
		if err != nil {
			return nil, publicPipelineError("seal-facts", err)
		}
		facts = append(facts, MetadataFact{Kind: canonical.Kind, Value: canonical.Value, Origin: MetadataOriginConverter})
	}
	sort.SliceStable(facts, func(i, j int) bool { return compareFacts(facts[i], facts[j]) < 0 })
	for i := 1; i < len(facts); i++ {
		if facts[i-1].Kind == facts[i].Kind {
			return nil, publicPipelineError("seal-facts", ErrIntegrityFailure)
		}
	}
	return facts, nil
}

func canonicalArtifactFacts(raw []MetadataFact) ([]MetadataFact, error) {
	facts := make([]MetadataFact, 0, len(raw))
	for _, fact := range raw {
		if fact.Origin != MetadataOriginConverter {
			return nil, publicPipelineError("seal-artifact-facts", ErrIntegrityFailure)
		}
		canonical := fact.Value
		switch fact.Kind {
		case "bits_per_component", "height", "object", "page", "width":
			if !canonicalNonnegativeInteger(canonical) {
				return nil, publicPipelineError("seal-artifact-facts", ErrIntegrityFailure)
			}
		case "image_mask":
			if canonical != "true" && canonical != "false" {
				return nil, publicPipelineError("seal-artifact-facts", ErrIntegrityFailure)
			}
		default:
			return nil, publicPipelineError("seal-artifact-facts", ErrIntegrityFailure)
		}
		facts = append(facts, MetadataFact{Kind: fact.Kind, Value: canonical, Origin: MetadataOriginConverter})
	}
	sort.SliceStable(facts, func(i, j int) bool { return compareFacts(facts[i], facts[j]) < 0 })
	for index := 1; index < len(facts); index++ {
		if facts[index-1].Kind == facts[index].Kind {
			return nil, publicPipelineError("seal-artifact-facts", ErrIntegrityFailure)
		}
	}
	return facts, nil
}

func canonicalNonnegativeInteger(value string) bool {
	if value == "" || len(value) > 1 && value[0] == '0' {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}

func publicFactOrigin(origin internalingestion.FactOrigin) MetadataOrigin {
	switch origin {
	case internalingestion.OriginCaller:
		return MetadataOriginCaller
	case internalingestion.OriginSource:
		return MetadataOriginSource
	case internalingestion.OriginSniff:
		return MetadataOriginSniff
	case internalingestion.OriginConverter:
		return MetadataOriginConverter
	default:
		return MetadataOrigin("")
	}
}

func materializeIngestionPolicy(policy IngestionPolicy) (IngestionPolicy, error) {
	if policy == (IngestionPolicy{}) {
		policy = DefaultIngestionPolicy()
	}
	if policy.Component != "" && !safePublicText(policy.Component) {
		return IngestionPolicy{}, publicPipelineError("policy", ErrPolicyViolation)
	}
	if _, err := internalingestion.NewRequestBudget(internalLimits(policy)); err != nil {
		return IngestionPolicy{}, publicPipelineError("policy", err)
	}
	return policy, nil
}

func internalLimits(policy IngestionPolicy) internalingestion.Limits {
	return internalingestion.Limits{
		MaxSourceBytes:         policy.MaxSourceBytes,
		MaxPrimaryBytes:        policy.MaxPrimaryBytes,
		MaxArtifacts:           policy.MaxArtifacts,
		MaxArtifactBytes:       policy.MaxArtifactBytes,
		MaxTotalArtifactBytes:  policy.MaxTotalArtifactBytes,
		MaxContainerEntries:    policy.MaxContainerEntries,
		MaxContainerEntryBytes: policy.MaxContainerEntryBytes,
		MaxExpandedBytes:       policy.MaxExpandedBytes,
		MaxContainerDepth:      policy.MaxContainerDepth,
		MaxExpansionRatio:      policy.MaxExpansionRatio,
	}
}

func terminalConverterError(err error) bool {
	return errors.Is(err, ErrMalformedInput) ||
		errors.Is(err, ErrLimitExceeded) ||
		errors.Is(err, ErrPolicyViolation) ||
		errors.Is(err, ErrIntegrityFailure) ||
		errors.Is(err, ErrCancellation) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, internalingestion.ErrLimitExceeded) ||
		errors.Is(err, internalingestion.ErrPolicyViolation) ||
		errors.Is(err, internalingestion.ErrIntegrityFailure) ||
		errors.Is(err, internalingestion.ErrCancellation)
}

func publicPipelineError(operation string, err error) error {
	if failure := (*FailureError)(nil); errors.As(err, &failure) {
		return err
	}
	category := FailureIntegrity
	switch {
	case errors.Is(err, ErrCancellation), errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded), errors.Is(err, internalingestion.ErrCancellation):
		category = FailureCancellation
	case errors.Is(err, ErrLimitExceeded), errors.Is(err, internalingestion.ErrLimitExceeded):
		category = FailureLimit
	case errors.Is(err, ErrPolicyViolation), errors.Is(err, internalingestion.ErrPolicyViolation):
		category = FailurePolicy
	case errors.Is(err, ErrMalformedInput):
		category = FailureMalformed
	case errors.Is(err, ErrConverterFailure):
		category = FailureConverter
	case errors.Is(err, ErrIntegrityFailure), errors.Is(err, internalingestion.ErrIntegrityFailure):
		category = FailureIntegrity
	}
	return &FailureError{Category: category, Operation: operation, Cause: err}
}

func enrichStreamInfoWithFacts(reader io.ReadSeeker, base StreamInfo) (StreamInfo, StreamInfo, error) {
	base = base.normalize()
	enriched, err := enrichStreamInfo(reader, base)
	if err != nil {
		return StreamInfo{}, StreamInfo{}, err
	}
	var sniff StreamInfo
	if base.MIMEType == "" {
		sniff.MIMEType = enriched.MIMEType
	}
	if base.Extension == "" {
		sniff.Extension = enriched.Extension
	}
	if base.Charset == "" {
		sniff.Charset = enriched.Charset
	}
	if base.Filename == "" {
		sniff.Filename = enriched.Filename
	}
	return enriched, sniff, nil
}
