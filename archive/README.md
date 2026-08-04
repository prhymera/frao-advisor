# Frao Advisor — Database Archive

Snapshot of the frao-advisor usage database, taken **2026-08-04** when the
dashboard was separated into a standalone Docker service.

- **File:** `advisor-history-2026-08-04.db`
- **Contents (at snapshot):** 138 consultations · 279 expert reviews · 24 deliberations
- **Why archived:** the dashboard now reads `./data/advisor.db` (bind-mounted into the
  `frao-advisor-dashboard` container). This file preserves the pre-separation history.

Restore it if ever needed:

```bash
cp archive/advisor-history-2026-08-04.db data/advisor.db
```
