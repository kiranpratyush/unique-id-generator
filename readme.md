# unique-id-generator

A Snowflake-style 64-bit unique ID generator in Go, verified with
[Maelstrom](https://github.com/jepsen-io/maelstrom)'s `unique-ids` workload.

Each node generates IDs independently, with no coordination between nodes,
so it keeps working through network partitions.

## ID layout

```
 63  62                                    22 21          12 11          0
┌───┬────────────────────────────────────────┬──────────────┬─────────────┐
│ 0 │         timestamp (41 bits)            │ machine (10) │ sequence(12)│
└───┴────────────────────────────────────────┴──────────────┴─────────────┘
```

| Field     | Bits | Range        | Meaning                                         |
|-----------|------|--------------|-------------------------------------------------|
| sign      | 1    | always 0     | keeps IDs positive in signed 64-bit types       |
| timestamp | 41   | ~69.7 years  | milliseconds since a custom epoch (2000-11-23)  |
| machine   | 10   | 0–1023       | unique per node                                 |
| sequence  | 12   | 0–4095       | counter within the same millisecond             |

```
id = (timestamp << 22) | (machineID << 12) | sequence
```

Because the timestamp occupies the high bits, IDs are roughly sorted by
creation time, and the creation time can be recovered with `id >> 22`.

## How the generator works

- **Same millisecond:** increment the sequence.
- **New millisecond:** reset the sequence to 0.
- **Sequence exhausted (4096 IDs in one ms):** wait for the next millisecond.
- **Clock moved backwards:** return an error rather than risk duplicates.
- **Concurrency:** a mutex guards the generator state for the whole
  read-decide-write step, including the wait.
- **Validation:** every field is bounds-checked before packing; a negative
  timestamp is checked while still signed, before conversion to `uint64`.

Bit widths are defined once as constants; max values and masks are derived
from them, so changing the split means editing two numbers.

## Project layout

| File            | Contents                                              |
|-----------------|-------------------------------------------------------|
| `snowflake.go`  | bit layout, pack/decode, the generator                |
| `main.go`       | Maelstrom node: handles `generate`, maps `nN` → machine id `N` |
| `main_test.go`  | round-trip, bit-layout, sign-bit, validation and ordering tests |
| `snowflake_test.go` | generator tests with fake clocks: same-ms sequencing, reset on new ms, overflow wait, backwards clock, concurrent uniqueness |

## Running

Tests:

```bash
go test -race ./...
```

Maelstrom (assumes Maelstrom is checked out at `../maelstrom`):

```bash
go build -o unique-id-generator .
../maelstrom/maelstrom test -w unique-ids --bin ./unique-id-generator \
  --time-limit 30 --rate 1000 --node-count 3 \
  --availability total --nemesis partition
```

A passing run ends with `Everything looks good! ヽ('ー`)ノ`.
Browse results with `../maelstrom/maelstrom serve` (http://localhost:8080).

## Lessons learned

- **The 2^53 trap.** IDs are sent to Maelstrom as strings. The Maelstrom Go
  library's `Reply` round-trips the body through `map[string]any`, which
  decodes JSON numbers as `float64` — exact only up to 2^53. Snowflake IDs are
  ~2^61, so neighbouring IDs rounded to the same value and the first run
  failed with duplicates. Same reason Twitter's API exposes `id_str`.
- **One meaning per state variable.** The sequence field must consistently
  mean "last value used"; mixing that with "next value to use" produced
  duplicate IDs within a millisecond.
- **Check, don't mask, the top field.** Masking an oversized timestamp to
  41 bits would silently wrap it; returning an error fails loudly.

## Not production-ready yet

- Machine id assignment outside Maelstrom (config, StatefulSet ordinal, or
  leases via etcd/ZooKeeper).
- Tolerating small backward clock steps; protecting against restarts with a
  clock behind the last issued timestamp (e.g. persisting a reserved
  high-water mark and waiting past it on startup).
- A more recent epoch to use the full ~69-year range.

Note: Snowflake IDs reveal creation time and approximate volume, so they
should not be used where IDs must be unguessable.
