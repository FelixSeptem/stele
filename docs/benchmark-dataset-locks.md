# Generic Retrieval Dataset Locks

This file records the first small external inputs selected for the
`generic_retrieval` benchmark family. The complete source data is not part of
the repository. Operators must fetch it into a local cache, verify every
digest in [`generic-retrieval-dataset-locks.json`](../internal/benchmark/testdata/generic-retrieval-dataset-locks.json), and keep the run offline.

| Dataset | Upstream revision | Language | Locked subset | Records | Raw size | Local budget | Status |
| --- | --- | --- | --- | ---: | ---: | ---: | --- |
| `MTEB/scifact` | `817e29a1e23c4a82a92bd97c2f78e0fae52c3d54` | English (`en`) | BEIR/MTEB `test` | 5,183 corpus / 1,109 queries / 339 qrels | 7.79 MiB | 30 MB | metadata-only |
| `C-MTEB/BQ` | `ba129a2170701a233c15533e61a010c3ae0d1b1c` | Chinese (`zh`) | C-MTEB `test` pairwise reranking | 10,000 pairs (10,000 query/text/score rows) | 5.33 MiB for all three splits | 12 MB | metadata-only |

## License and redistribution

ModelScope metadata reports Apache License 2.0 for both entries. The SciFact
dataset card currently declares `license: unknown`, and the BQ card does not
include a license field. Therefore both locks remain `license_status:
needs_review` and `redistribution: restricted`: the files may be used from a
user-owned local cache after review, but are not redistributed by Stele or
committed to Git. A future license review may promote a lock without changing
the upstream digest.

## Input identity

SciFact is a corpus/query/qrels retrieval input. Its test qrels contain 339
records against the 5,183-document corpus and 1,109-query set. BQ is a Chinese
sentence-pair scoring task; it is locked as a pairwise reranking input and must
be converted to an explicit generic query/document/qrels representation before
running a retrieval comparison. It must not be silently treated as a
conversation-memory benchmark.

The lock records exact per-file sizes and SHA-256 values, an estimated
normalized-cache size, and a local storage budget that includes normalized
artifacts and reports. Semantic runs still require an operator-selected,
dimension-locked embedding profile; lexical-only runs are the deterministic
baseline.

## Acquisition and acceptance

Fetch is explicit and offline runs do not contact ModelScope. Before enabling a
dataset, verify the upstream license, compare all file digests, normalize into
the `generic_retrieval` family, and retain the manifest and report. Until that
review and a same-corpus smoke run are complete, the registry intentionally
keeps `mteb` and `c-mteb` at `planned` and these concrete locks at
`metadata-only`.
