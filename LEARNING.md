# Learning TODO

Topics this project touched, ordered from the hardware up. Each item says
why it matters *here*, where to learn it, and a small exercise in this repo.

**Core books**
- **CS:APP**: *Computer Systems: A Programmer's Perspective* (Bryant & O'Hallaron)
- **OSTEP**: *Operating Systems: Three Easy Pieces* (free at ostep.org)
- **TLPI**: *The Linux Programming Interface* (Kerrisk)
- **DDIA**: *Designing Data-Intensive Applications* (Kleppmann)

---

## 1. Data representation

- [ ] **Two's complement and the sign bit**
  - Why: the top bit is 0 so IDs stay positive in signed types; `uint64(-5)` becomes a huge number.
  - Learn: CS:APP ch. 2 (integer representation).
  - Exercise: print `-1`, `-5` and `MaxInt64` as `%064b` and explain every bit.

- [ ] **Floating point: why float64 is exact only up to 2^53**
  - Why: the first Maelstrom run failed because IDs went through `float64` in JSON decoding.
  - Learn: CS:APP ch. 2 (IEEE 754), "What Every Computer Scientist Should Know About Floating-Point Arithmetic".
  - Exercise: find the smallest integer that `float64` cannot represent exactly; convert two adjacent Snowflake IDs to `float64` and compare.

## 2. Time and clocks

- [ ] **Wall clock vs monotonic clock**
  - Why: `Generate` errors when the clock goes backwards; a monotonic clock never does.
  - Learn: `man 7 time`, `man 2 clock_gettime` (`CLOCK_REALTIME` vs `CLOCK_MONOTONIC`), TLPI ch. 10, Go `time` package docs ("Monotonic Clocks").
  - Exercise: build a clock that reads wall time once at startup and adds monotonic elapsed time; swap it in for `generateTimeStamp`.

- [ ] **NTP: slewing vs stepping**
  - Why: decides whether the clock jumps backwards or just slows down.
  - Learn: `man 8 chronyd` / `man 8 systemd-timesyncd`, DDIA ch. 8 ("Unreliable Clocks").
  - Exercise: change `Generate` to wait out small backward steps (< 5 ms) and error only on large ones; test with `scriptedClock`.

## 3. Processes, scheduling and pauses

- [ ] **Process pauses (GC, scheduling, swapping, VM migration)**
  - Why: a paused node can't tell that time passed; this breaks leases.
  - Learn: OSTEP (CPU scheduling, swapping), CS:APP ch. 8 (processes) and §9.10 (garbage collection), DDIA ch. 8 ("Process Pauses").
  - Exercise: run with `GODEBUG=gctrace=1` and read the pause times Go reports.

- [ ] **How Go's garbage collector works**
  - Learn: "A Guide to the Go Garbage Collector" (go.dev/doc/gc-guide).
  - Exercise: write a benchmark that allocates heavily and watch pause times with `gctrace`.

## 4. Concurrency

- [ ] **Mutexes, data races and the memory model**
  - Why: removing the mutex in `Generate` gave duplicate IDs and 13 data races.
  - Learn: OSTEP concurrency chapters (locks, condition variables), CS:APP ch. 12, "The Go Memory Model" (go.dev/ref/mem).
  - Exercise: explain *exactly* which interleaving of two goroutines produces a duplicate ID without the lock.

- [ ] **Lock-free with atomics (compare-and-swap)**
  - Learn: OSTEP (locks chapter, hardware primitives), `sync/atomic` docs.
  - Exercise: pack `lastTimestamp` + `sequence` into one `uint64` and implement `Generate` with a CAS loop; benchmark against the mutex version.

## 5. Storage and durability

- [ ] **`fsync` and what "on disk" really means**
  - Why: required for persisting a reserved high-water mark safely.
  - Learn: OSTEP persistence chapters (files & directories, crash consistency), TLPI ch. 13 (file I/O buffering).
  - Exercise: implement restart safety: persist "may use timestamps up to T", `fsync`, wait past T on startup; test crash points with a fake clock.

## 6. Distributed systems

- [ ] **Leases and fencing tokens**
  - Why: the robust way to assign machine ids; survives GC pauses.
  - Learn: DDIA ch. 8 ("The Truth Is Defined by the Majority", fencing tokens), Kleppmann's "How to do distributed locking".
  - Exercise: write down, step by step, how two nodes end up with the same machine id after a pause, and how a fencing token prevents the damage.

- [ ] **Coordination services (ZooKeeper / etcd) and consensus**
  - Why: where leases come from.
  - Learn: DDIA ch. 9, the Raft paper ("In Search of an Understandable Consensus Algorithm"), etcd docs on leases.
  - Exercise: assign machine ids from an etcd lease instead of the Maelstrom node id.

- [ ] **Machine id assignment in practice**
  - Options: static config, Kubernetes StatefulSet ordinal, leases.
  - Exercise: list the failure mode of each option.

## 7. ID design alternatives (interview prep)

- [ ] **UUID v4 vs UUID v7** (RFC 9562): size, sortability, B-tree locality.
- [ ] **Database auto-increment**: simplicity vs single point of failure.
- [ ] **Ticket servers** (Flickr's odd/even design).
- [ ] **Range / hi-lo allocation**: reserve blocks, persist the upper bound.
- [ ] **Twitter's original Snowflake announcement (2010)**.
- [ ] Be able to explain: throughput math (4096/ms/node), how to re-split the bits, what IDs leak (time, volume, node count), k-sorted vs strictly ordered.
