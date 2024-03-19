# Locking behavior

When you're using [advisory locks](https://www.postgresql.org/docs/9.1/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS) in Postgres, lock calls stack when executed on the same connection (A.K.A. session):

- Connection 1 calls `pg_advisory_lock(42)`, acquires the lock and continues
- Connection 1 calls `pg_advisory_lock(42)`, this lock "stacks" with the previous call and continues
- Connection 2 calls `pg_advisory_lock(42)`, this blocks
- Connection 1 calls `pg_advisory_unlock(42)`, this pops one lock call off the stack and continues
- Connection 1 calls `pg_advisory_unlock(42)`, this pops the last lock call off the stack and continues
- Connection 2 finally acquires the lock and continues

If you get connections from a pool (e.g. the standard `sql` library in Go maintains an internal pool of connections), you need to be aware of the locking behavior otherwise you might get unpredictable behavior or deadlock. You can get deterministic behavior by explicitly taking a connection from the pool (e.g. with `db.Conn()`).

Here's an example of bad code that can deadlock if the connection happens to be different across lock calls: ❌

```go
// Grab a write lock
db.Exec("SELECT pg_advisory_lock(1)")
// Grab a read lock
db.Exec("SELECT pg_advisory_lock_shared(1)") // 💥 Can deadlock
```

Good code explicitly takes a connection out of the pool first ✅

```go
conn := db.Conn()
// Grab a write lock
conn.Exec("SELECT pg_advisory_lock(1)")
// Grab a read lock
conn.Exec("SELECT pg_advisory_lock_shared(1)") // OK, will not block
```
