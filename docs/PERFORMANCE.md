# Performance measurements and component ownership

Run `npm run test:performance` from the project root. The Go benchmark creates
and removes its own temporary SQLite databases. It adds 5,000 contacts, 5,000
applications and 5,000 required documents to the demo data; 20% of those records
belong to another workspace. Application actions have deadlines and outstanding
document requirements. Setup and authentication token creation are excluded from
timing. Each HTTP handler must return success or the benchmark fails.

These measurements cover backend request processing and allocations, including
permissions, SQL reads and validated writes. They exclude network latency,
browser rendering and external services. Use the actual deployment's record mix
and a browser trace before setting user-facing loading targets. Three iterations
provide a short local baseline, not percentiles or production capacity.

## Baseline — 2026-10-11

Linux/amd64, Intel Core 2 Quad Q9400 at 2.66 GHz, benchmark parallelism 4,
three measured iterations per operation. Results are means for this local run.
The different hardware makes direct comparison with older baselines unreliable.

| Operation | Mean request time | Allocated bytes per operation |
| --- | ---: | ---: |
| Contact list, 25 records after offset 500 | 22.5 ms | 366,989 |
| Global search, `client-123` | 1,504.4 ms | 327,090,832 |
| Application record detail | 1.10 ms | 57,869 |
| Application requirements | 17.9 ms | 92,101 |
| Dashboard report aggregates | 608.0 ms | 114,078,648 |
| Daily work totals and first page | 533.7 ms | 117,649,205 |
| Validated contact save | 1.37 ms | 90,493 |

Application detail, requirements and dashboard report requests are measured
separately; these numbers cannot be summed into complete page-load times because
requests may run concurrently and pages fetch additional tools. The search
query matches a subset of records; repeat with names, emails and reference
numbers, both common and rare, on deployment data before prioritizing changes.

Global search was the first candidate for profiling because its latency and
allocations are highest here. Dashboard and daily-work aggregation are the next
candidates. Inspect query plans and allocation profiles, then compare the same
workload before and after any proposed fix. Preserve workspace boundaries,
visibility, counts, ordering and revision checks. This cycle records the evidence;
it does not assume that a faster individual query improves the whole page.

## Search improvement — 2026-10-11

Global search now reads its results directly from SQL. Previously, request
middleware loaded and cloned the complete workspace, and the handler serialized
every entity again for each candidate result to check visibility. Search now
shares the record-list SQL visibility predicate, including document, payment and
activity parent checks. Those checks run before sorting and the 31-candidate
limit, so hidden parents cannot crowd accessible matches out of the 30 results.
Literal matching, searched fields and title/entity/ID ordering are preserved.

Fresh before/after runs used the same hardware and 5,000-record workload above,
with three measured iterations per run:

| Search, `client-123` | Before | After |
| --- | ---: | ---: |
| Mean request time | 2,754.7 ms | 134.6 ms |
| Allocated bytes per request | 327,090,848 | 123,898 |
| Allocations per request | 691,122 | 915 |

The local result is about 20 times faster with 99.96% fewer allocated bytes.
The fresh before measurement differs from the earlier baseline; these short
samples remain sensitive to machine load and do not establish production
percentiles or complete browser loading times. Dashboard and daily-work
aggregation remain the next profiling candidates.

Reproduce the isolated comparison with:

```sh
cd golang-backend
go test -run '^$' -bench '^BenchmarkRepositoryWorkload/search$' -benchtime=3x -benchmem
```

Verification: the full backend suite passes. Search regression tests cover all
13 searchable entities, committed SQL records absent from the legacy cache,
workspace/team/private visibility, hidden/foreign/missing parents, optional
activity parents, document case precedence, and exact 30/31-result boundaries.
Existing authentication and literal wildcard tests also pass.

## JavaScript ownership

`public/bootstrap.js` initializes Elm and bridges ports. Workspace keyboard
access, modal focus, read-only affordances and handoff deep links belong to
`public/workspace-interactions.js`. Its controller owns one observer, delegated
document listeners and explicit `start()` / `stop()` methods; repeated starts do
not duplicate listeners. Changes to these behaviors should stay in that module.
The PWA shell includes it so offline work has the same interactions.

Daily work and student journey rendering remain owned by their custom elements
in `public/components.js`. Application requirements belong to `public/progress.js`;
modal drafts belong to `public/draft-recovery.js`; staff observations and review
priorities belong to `public/usability.js`. Continue consolidating by behavior
when editing those screens. Keep lifecycle cleanup and meaningful interaction
checks with each move, and avoid changing backend permission enforcement during
frontend reorganization.
