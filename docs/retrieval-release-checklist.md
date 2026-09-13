# Retrieval release checklist

- [ ] Owned PostgreSQL 18 + pgvector DSN is explicit and isolated from the
      runtime database.
- [ ] Fixture, representation, fusion, ranking, embedding, reranker, analysis,
      and release-policy identities are compatible with the immutable baseline.
- [ ] Real-stack replay is passed; skipped/synthetic evidence is not treated as
      a release pass.
- [ ] Protected simple-fact recall, temporal coverage, multi-hop coverage,
      duplicate rate, candidate budget, latency, and isolation gates are green.
- [ ] Progressive context levels have fresh watermarks, bounded costs,
      citations, and deterministic rebuild evidence.
- [ ] Parent-first remains shadow/off unless the same gates and rollback proof
      pass for the exact scope.
- [ ] Retrieval trajectories and integrity reports are redacted, bounded, and
      accessible only on authorized evaluation/admin paths.
- [ ] Retention cleanup is tested and does not delete canonical source records.
- [ ] Rebuild/re-index and rollback runbooks were exercised and append-only
      history is preserved.
- [ ] Threshold owner reviewed the versioned release policy and recorded the
      decision category.
