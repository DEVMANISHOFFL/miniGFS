# Mini-GFS (Google File System Inspired)

A lightweight, educational distributed file system inspired by the Google File System design. Mini-GFS consists of a **Master** node that manages metadata and multiple **ChunkServers** that store replicated chunks on disk. The goal is to learn how large-scale distributed storage works underneath while keeping the implementation simple and readable.

## Table of Contents
- Overview
- Features
- Architecture
- Repo Layout
- Requirements
- Build & Run
- HTTP APIs
- Example Flow
- Configuration
- Troubleshooting
- Contributors
- License

## Overview
Mini-GFS implements the core ideas of the Google File System (GFS) in a simplified manner:

- A **Master** service that:
  - Tracks all ChunkServers using heartbeats.
  - Maintains metadata mapping filenames → chunk handles → replica locations.
  - Allocates new chunks and selects replicas.
  - Detects dead servers via a sweeper goroutine.
  - Manages write leases for consistency.

- **ChunkServers** that:
  - Register automatically with the master.
  - Send periodic heartbeats.
  - Store chunk data in `data/<chunkHandle>.bin`.
  - Serve read/write operations.
  - Participate in chained replication.

## Features

### Master
- `/register` and `/heartbeat` endpoints.
- Detects alive/dead chunkservers.
- File + chunk metadata management.
- Chunk allocation with replica selection.
- Primary lease assignment for writes.
- Re-replication (depending on progress stage).

### ChunkServer
- Auto-registration with master.
- Disk-backed chunk store.
- Read/write APIs.
- Write replication chain via `/forward-write`.
- Graceful shutdown logging.

## Architecture
Mini-GFS follows the classic GFS model:

- **Master = control + metadata plane**  
- **ChunkServers = data plane**  
- **Clients = request reads/writes**

### Write Flow
1. Client asks Master for the primary + replicas of a chunk.  
2. Client pushes data to replicas.  
3. Primary applies write ordering.  
4. Primary instructs replicas to commit.

### Read Flow
Client reads directly from any replica provided by the Master.

## Repo Layout
```

/master
main.go
state.go
handlers.go
sweeper.go
metadata/
files.json

/chunkserver
main.go
storage.go
replication.go
client.go

/shared
types.go

README.md

````

## Requirements
- Go ≥ 1.20

## Build & Run

### Start Master
```bash
cd master
go build -o master
./master
````

### Start ChunkServer

```bash
cd chunkserver
go build -o chunkserver
./chunkserver -port=9001
```

Run more chunkservers:

```bash
./chunkserver -port=9002
./chunkserver -port=9003
```

## HTTP APIs

### Master

* POST `/register`
* POST `/heartbeat`
* POST `/create-file`
* POST `/allocate-chunk`
* GET `/file-metadata/<filename>`
* POST `/get-primary`

### ChunkServer

* POST `/write-chunk`
* POST `/forward-write`
* GET `/read-chunk?chunk=<id>`

## Example Flow

### Create a file

```bash
curl -X POST localhost:8080/create-file -d '{"filename":"hello.txt"}'
```

### Allocate a new chunk

```bash
curl -X POST localhost:8080/allocate-chunk -d '{"filename":"hello.txt"}'
```

### Write data

Client gets primary using `/get-primary`, then writes to primary, which replicates to secondaries.

### Read a chunk

```bash
curl "localhost:9001/read-chunk?chunk=chunk_0001"
```

## Configuration

Tunables such as:

* Heartbeat interval
* Timeout threshold
* Replication factor
* Storage directory

Are found in:

* `master/state.go`
* `chunkserver/client.go`

## Troubleshooting

* **Master marks server dead too fast** → Increase timeout.
* **Reads fail** → Check if replica is alive.
* **Writes not replicating** → Inspect `/forward-write` logs.
* **Allocation fails** → Not enough alive replicas.

## Contributors

Inspired by the Google File System paper. Created as a learning project to understand distributed storage systems.

## License
MIT License
