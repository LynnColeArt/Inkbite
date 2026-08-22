---
mission_slug: inkbite-ingestion-contract-01M0M3HW
wp_id: WP03
ruling: correction_required_with_contract_clarification
arbiter_profile: architect-alphonso
ruled_at: '2026-08-22T11:20:54Z'
reviewed_product_commit: f5c8a49faab488b49b49a25fe6cc1a18dae33f52
review_artifact: review-cycle-1.md
primary_state_commit: 68d9c08e8a08b566bc5b736d70250c637bbb5509
affected_files:
  - kitty-specs/inkbite-ingestion-contract-01M0M3HW/tasks/WP03-source-acquisition-remote-authority/arbiter-cancellation-ruling.md
---

# WP03 Binding Cancellation and Remote-Admission Ruling

This ruling resolves the architectural question raised by WP03 review cycle 1.
It does not approve WP03, alter runtime status, modify product code, merge or
push a branch, or activate dependent work.

## Binding disposition

| Review finding | Ruling | Consequence |
|---|---|---|
| The IANA deny set admits `100:0:0:1::1` and `2001:5::1` | **Valid blocking product defect** | Correct the registry policy and prove it red-to-green before cycle-2 review. |
| An arbitrary non-closing `io.Reader` or `io.ReadSeeker` cannot acknowledge cancellation while its synchronous `Read` is blocked | **Valid observation; overreach if interpreted as a prompt-interruption requirement** | Clarify the contract, preserve the synchronous join boundary, and add the tests specified below. |
| NFR-007/T010 require Inkbite to forcibly interrupt every arbitrary reader within one second | **Not an implementable or approved v1 guarantee** | No detached read worker, legacy rejection, or public API expansion is authorized. |

The cancellation finding therefore identified a real ambiguity and correctly
escalated it instead of prescribing a timing-only patch. It does **not** prove
that `f5c8a49` violates the intended cooperative cancellation guarantee merely
because an arbitrary non-cooperative `Read` remains blocked. It does prove that
the broad wording in `spec.md` and T010 must be reconciled with the already
approved charter, research, and plan before WP03 can be approved.

WP03 remains rejected independently because the IANA finding is valid.

## Meaning of the cancellation terms

For this mission, the following meanings are authoritative:

- **Cancellation acknowledgement** means that Inkbite observes a canceled or
  expired context at a cooperative boundary, returns the stable cancellation
  category (wrapping `context.Canceled` or `context.DeadlineExceeded`), and
  returns no successful or partial source/envelope.
- **Cooperative boundary** means an operation that either accepts a context,
  returns control so Inkbite can checkpoint the context, or has an explicitly
  exercised interruption operation whose completion unblocks the in-flight
  call. A type assertion to `io.Closer` alone is not a universal promise that
  `Close` interrupts `Read`; the concrete implementation must cooperate.
- **Mission-owned worker** means a goroutine or process started by Inkbite for
  the request. Inkbite must join it before reporting terminal completion.
- **Non-cooperative reader** means a caller-owned `io.Reader` or
  `io.ReadSeeker` whose in-flight `Read` or `Seek` neither accepts the request
  context nor returns in response to an available interruption operation.

The one-second target applies to cooperative work. It is not a preemption
facility for arbitrary Go method calls.

## Evidence and constraint analysis

### Approved mission evidence

1. The project charter states twice that cancellation must terminate
   **cooperative work** within one second.
2. `plan.md` line 31 carries the same cooperative-boundary qualifier.
3. `research.md` R-009 states that cancellation is advisory and that
   mission-owned workers must be joined before terminal cleanup.
4. `research.md` risk 5 explicitly states that context cancellation cannot
   forcibly stop pure library calls and that process isolation is out of scope.
5. The plan preserves the existing `Converter`, `Result`, source entry points,
   and one shared pipeline. The mission base accepts arbitrary `io.Reader` and
   `io.ReadSeeker` values.
6. `context.CancelFunc` signals abandonment but does not wait for work to stop.
   The standard `io.Reader`, `io.ReadSeeker`, and `io.Closer` interfaces expose
   no context parameter and no general interrupt/join guarantee.

The broad NFR-007 phrase “Blocking reader ... tests must return within one
second” is therefore underspecified: it omits the cooperative qualification
that the charter, plan performance target, and research explicitly retain.
Under specification-fidelity and living-documentation doctrine, this must be
amended rather than silently interpreted by implementation.

### Exact reproduction at `f5c8a49`

A disposable external-package probe drove the stable public `Engine.Convert`
entry point with a channel-synchronized non-closing reader and read-seeker.
For each source it:

1. waited until `Read` was definitely entered;
2. canceled the context;
3. observed no return for 250 ms while `Read` remained blocked;
4. released `Read`; and
5. observed a joined return with both `ErrCancellation` and
   `context.Canceled`, and no result.

The probe passed for both source forms in 0.50 seconds total. This proves the
reviewer's empirical statement and also proves that `f5c8a49` does not abandon
a read worker: the public call remains synchronously joined to caller-owned
`Read` and classifies cancellation at the next checkpoint.

The same probe directly called the `f5c8a49` remote-address policy and confirmed
that both rejected IPv6 sentinels returned allowed.

## Impossibility boundary

The following four properties cannot all be supplied for an arbitrary
non-cooperative `io.Reader`/`io.ReadSeeker`:

1. preserve the accepted legacy reader surface;
2. return within one second after mid-call cancellation;
3. never abandon an unjoined mission-owned worker; and
4. execute in-process using synchronous Go `Read`/`Seek` semantics.

Calling `Read` in a goroutine and selecting on `ctx.Done()` satisfies property
2 only by violating property 3 when the reader never returns. Waiting for that
goroutine preserves property 3 but cannot satisfy property 2. Rejecting all
non-closing readers under a cancellable context violates property 1 and would
also reject ordinary readers such as `bytes.Reader` and `strings.Reader`.
Process isolation cannot marshal an arbitrary in-process Go interface and is
explicitly outside this mission.

## Binding v1 cancellation semantics

1. Check the context before source-specific work, before every owned read
   iteration, after every read, after initial seek, and before returning
   success.
2. A pre-canceled request performs no caller `Seek` or `Read` and returns typed
   cancellation with a zero result.
3. For a cooperative blocking source, cancellation must unblock and join the
   operation, return within one second, return typed cancellation, and expose
   no partial source or success result.
4. When a caller source implements `io.Closer`, Inkbite may call `Close` on
   cancellation as the interruption attempt. The one-second guarantee applies
   only when that concrete `Close` cooperatively unblocks the in-flight call.
5. For an arbitrary non-cooperative `Read` or `Seek`, Inkbite remains
   synchronously joined until the method returns. It then observes cancellation
   at the next checkpoint and returns typed cancellation with no partial source.
6. Inkbite must not introduce a read goroutine solely to race an arbitrary
   reader against `ctx.Done()` and return without joining that goroutine.
7. These rules apply equally to legacy and detailed ingestion because both use
   the single acquisition pipeline.

No public API change is authorized or required. A future strict-preemption
capability would need a new, explicitly cancelable source contract or an
isolated execution boundary and is a separate mission.

## Required narrow contract amendment

No charter amendment or exception is needed: the charter already says
“cooperative work.” Before cycle-2 approval, the primary planning owner must
make one independently reviewed specification amendment, limited to these
behavior descriptions:

1. **`spec.md` NFR-007** — replace the unqualified blocking-reader promise with:

   > Cancellation returns a typed failure and no successful envelope. Work at
   > a cooperative interruption boundary, including deterministic blocking
   > reader, remote, and converter fixtures, must terminate and join all
   > Inkbite-owned workers within one second. An arbitrary caller-owned
   > non-cooperative `io.Reader` or `io.ReadSeeker` remains synchronously joined
   > until its in-flight method returns; cancellation is then observed at the
   > next checkpoint and cannot yield partial success.

2. **WP03 T010** — say “cooperative blocking readers cancel promptly”; require
   arbitrary non-cooperative readers/seekers to remain joined and fail without
   a partial source after they return.
3. **`contracts/public-api.md`** — document that reader cancellation depends on
   a cooperative boundary, that cancellation may close a caller source which
   implements `io.Closer`, and that no read worker is abandoned.
4. **`plan.md` cancellation/redaction red** — replace the phrase suggesting
   that Inkbite can preempt non-cooperative calls with the joined semantics in
   this ruling. Preserve “cancellation without worker join” as a prohibited
   design risk.

`research.md`, the charter, and the data model's no-success-on-cancellation
invariant are already consistent and need no semantic change. User-facing
reader-cancellation guidance belongs in WP08 after the implementation is final.

## Exact cancellation test matrix for cycle 2

All tests must drive an existing public conversion entry point where possible
and synchronize on lifecycle channels rather than `time.Sleep`.

1. **Pre-canceled reader**: a counting/panic reader receives zero `Read` calls;
   the public result is zero and the error matches `ErrCancellation` plus the
   context cause.
2. **Pre-canceled read-seeker**: receives zero `Seek` and zero `Read` calls with
   the same typed zero-result outcome.
3. **Cooperative blocking reader**: wait for `Read` entry, cancel, have the
   fixture's `Close` unblock `Read`, require return within one second, assert
   `Read` and `Close` have both exited before API return, and assert no partial
   result or secret text.
4. **Cooperative blocking read-seeker**: the same matrix after a successful
   initial seek.
5. **Context-aware non-closing reader**: a fixture whose `Read` cooperates with
   the same context returns within one second with typed cancellation and no
   partial result.
6. **Non-cooperative non-closing reader**: wait for `Read` entry, cancel, prove
   the API has not returned while `Read` remains blocked, release it, then
   require a joined typed cancellation and zero result within one second of
   release.
7. **Non-cooperative non-closing read-seeker**: the same proof for an in-flight
   `Read`; separately prove an in-flight non-cooperative `Seek` remains joined
   and is classified at the first checkpoint after it returns.
8. **Partial-before-block**: allow an initial short read, block on the next
   read, cancel, release, and prove none of the accepted prefix escapes.
9. **No-abandon mutation**: a mutation that moves arbitrary `Read` to a worker
   and returns on `ctx.Done()` must fail cases 6–8 because the API would return
   before the fixture's in-flight method exits.
10. **Checkpoint/close mutations**: removing pre-seek, pre-read, post-read, or
    cooperative close/join guards must make the corresponding case fail for the
    intended observable reason.

The existing `blockingReadCloser` test uses a sleep before cancellation. It may
be retained only after its setup is changed to prove `Read` entry explicitly;
otherwise it is not reliable evidence of mid-read cancellation.

## IANA finding remains binding

The authoritative IANA IPv4 and IPv6 special-purpose registries still report a
`2025-10-09` revision as of this ruling. The IPv6 registry identifies:

- `100:0:0:1::/64` as the Dummy IPv6 Prefix with destination, forwarding, and
  global-reachability properties false; and
- `2001::/23` as default false for those properties unless a more-specific
  allocation grants an exception.

Therefore `100:0:0:1::1` and unexcepted `2001:5::1` must be denied. The
correction must use an auditable, revision-pinned, most-specific policy table:
an umbrella prefix denies by default, while only registry entries explicitly
globally reachable are deliberate exceptions. The retained matrix must cover
IPv4, IPv6, mapped IPv4, the two sentinels, umbrella exceptions, mixed DNS
answers, and guard-deletion mutations.

Primary sources:

- <https://www.iana.org/assignments/iana-ipv6-special-registry/>
- <https://www.iana.org/assignments/iana-ipv4-special-registry/>

## Bounded correction and re-review path

1. Preserve cycle-1 review and this ruling unchanged as historical evidence.
2. Land the narrow planning-contract amendment on primary and synchronize it
   into lane C without touching unrelated mission scope.
3. Keep the IANA red proof separate from its production correction.
4. Add the cancellation contract tests above red-first where a product guard is
   absent; do not create a red test for impossible arbitrary-reader preemption.
5. Limit product correction to WP03-owned source/remote files. Do not modify
   `Engine`, converter code, frozen surfaces, dependencies, or public types.
6. Rerun WP03 focused count/race gates, full normal/race suites, static,
   vulnerability, module, coverage, scope, redaction, remote-isolation, and
   compatibility gates.
7. Assign a reviewer independent of both the implementation session and this
   arbitration. The reviewer must verify exact final bytes, both IANA sentinels,
   the full cancellation matrix, and that no worker-outlive workaround or
   unapproved API change was introduced.
8. Only that independent cycle-2 review may approve or reject WP03. Dependent
   work remains inactive until approval.

This ruling is binding for WP03 cycle 2 and for later mission acceptance unless
superseded by an explicit, independently reviewed specification amendment.
