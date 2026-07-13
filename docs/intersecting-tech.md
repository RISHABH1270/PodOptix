# PodOptix — Intersecting Technologies

Interesting technologies used in PodOptix — explained clearly.

---

## UUID — Universally Unique Identifier

A 128-bit randomly generated ID. Always 36 characters: `a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d`

**Why not 1, 2, 3...?**
Sequential IDs are guessable — attacker tries `/clusters/1`, `/clusters/2` and accesses other users' data. UUID has 340 trillion trillion trillion possible values. You cannot guess the next one.

**How random is it?**
If you generated 1 billion UUIDs per second for 100 years — probability of collision: `0.0000000000000000006%`

**In Go:**
```go
import "github.com/google/uuid"
id := uuid.New().String()
// returns: "a3f8c2d1-9b4e-4f1a-8c3d-2e5f7a9b1c4d"
```

**In the database:**
```sql
cluster_id VARCHAR(36) PRIMARY KEY  -- always exactly 36 characters
```

---
