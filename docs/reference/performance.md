# Performance

Stele's compiled Go verifier reduces the latency of a representative verification check. The npm package exposes that executable directly at `dist/stele`, so an installed `stele` command has the same runtime path as the direct Go measurement below.

## Current benchmark

The benchmark ran `stele verify --json` against the standalone Todo example. The fixture contains four requirements, five scenarios, four implementation anchors, and five test anchors. All three implementations returned a passing verification result with zero linkage errors.

| Entry path | Median | Mean | p95 | Median improvement |
|---|---:|---:|---:|---:|
| JavaScript verifier from commit `6e3a337` | 60.41 ms | 61.04 ms | 63.79 ms | baseline |
| Go executable directly | 17.18 ms | 17.11 ms | 18.22 ms | **3.52x faster** |
| Current npm-installed CLI (`dist/stele`) | 17.18 ms | 17.11 ms | 18.22 ms | **3.52x faster** |
| Removed Node launcher and Go executable | 47.19 ms | 47.26 ms | 49.88 ms | **1.28x faster** |

The direct Go path lowered median end-to-end latency by 71.6%. Because npm now links the command straight to the executable, installed use keeps that improvement while preserving normal npm installation and command discovery. An earlier generated JavaScript launcher improved the old JavaScript implementation by only 21.9%; its 33 ms startup cost is why it was removed.

## Startup cost

The lightweight `stele version` command approximates process and launcher startup without parsing project files.

| Entry path | Median | Mean | p95 |
|---|---:|---:|---:|
| JavaScript verifier | 33.98 ms | 34.14 ms | 36.08 ms |
| Go executable directly | 4.06 ms | 4.06 ms | 4.57 ms |
| Current npm-installed CLI (`dist/stele`) | 4.06 ms | 4.06 ms | 4.57 ms |
| Removed Node launcher and Go executable | 33.40 ms | 33.73 ms | 35.16 ms |

Subtracting each startup median from its verification median gives an illustrative estimate of the actual check work: 26.42 ms for JavaScript and 13.12 ms for Go, about a 2.01x improvement. This subtraction is useful for orientation, but it is not a standalone microbenchmark of internal functions.

## Method

- Hardware and OS: Apple Silicon (`arm64`), macOS 26.3.2
- Runtime: Node.js 24.16.0 and Go 1.27.1
- Sample: 12 warmups followed by 160 measured runs per command
- Ordering: randomized between implementations within each operation
- Clock: monotonic nanosecond clock from Python `time.perf_counter_ns`
- Output: generated JSON was discarded to avoid terminal-rendering cost
- Go build: current source compiled with `-trimpath`
- npm path: direct executable at `dist/stele`, produced by `npm run build`

Each measurement launches a new process, so the numbers reflect interactive CLI latency with warm filesystem caches. Results will vary by machine, repository size, filesystem state, and Git working-tree size. The Todo fixture is deliberately small. The direct binary result also represents the current command developers run after installing the package. The removed launcher row is retained to document the measured reason for exposing the binary directly.

The JavaScript and Go reports use different verifier versions and input-digest implementations. Both found the same requirements and anchors and returned a passing linkage verdict; existing execution evidence was current under the JavaScript digest and stale under the Go digest. The timed command verifies structure and linkage rather than executing scenario tests, so this status difference does not affect the verification verdict.
