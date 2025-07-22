# Lazy-NCL

Lazy-NCL is my attempt at a distributed log storage system inspired by the principles of LazyLog (SOSP '24) and SplitFT (EuroSys '24) from the DASSL at the University of Illinois Urbana-Champaign. This project implements a simplified version of these systems to demonstrate the core concepts of decoupling the data and metadata paths for high-performance, fault-tolerant writes.

## Key Features

*   **Decoupled Data and Metadata:** The system separates the write path into a data path for storing the actual payload and a metadata path for ordering and replication. This allows for low-latency writes and flexible data placement.
*   **Fault Tolerance:** The metadata log is replicated across a configurable number of NCL (Near Compute Log) peers, using a quorum-based approach to tolerate `f` failures.
*   **Horizontal Scalability:** The data storage layer is sharded, allowing for horizontal scaling of storage capacity.
*   **Global Ordering:** A dedicated orderer component polls the metadata log to establish a global, linearizable order of all records in the system.
*   **gRPC Communication:** All communication between system components is done using gRPC, with data structures defined using Protocol Buffers.

## Architecture

The system is composed of the following microservices:

*   **Controller:** A central service that manages the cluster configuration, including the list of active NCL peers.
*   **NCL Peers:** A cluster of nodes that replicate the metadata log. They are responsible for appending and serving metadata entries.
*   **Data Shards:** A set of storage nodes that store the actual data payloads. The data is sharded across these nodes based on a hash of the record ID. RocksDB is used as the underlying storage engine.
*   **Orderer:** A service that polls the NCL leader to establish a global order of records. It constructs a global index that maps a global sequence number to a record ID and shard ID.
*   **Client:** A library that provides a simple API for interacting with the system, allowing users to append data and read it back by its global log position.

## How it Works

1.  **Write Path:**
    *   The client generates a unique record ID for the data.
    *   It determines the target data shard by hashing the record ID.
    *   It sends the data payload to the target data shard and, in parallel, sends the metadata (record ID and shard ID) to all NCL peers.
    *   The write is considered successful once the client receives acknowledgments from the data shard and a quorum of NCL peers.

2.  **Read Path:**
    *   The client requests to read data at a specific global log position from the Orderer.
    *   The Orderer resolves the global position to a record ID and shard ID.
    *   The client then fetches the data directly from the specified data shard using the record ID.

## Getting Started

### Prerequisites

*   [Docker](https://www.docker.com/) and [Docker Compose](https://docs.docker.com/compose/)
*   [Go](https://golang.org/) (for building the binaries)

### Building and Running

1.  **Build the Docker images and run the services:**

    ```bash
    docker-compose up --build
    ```

    This will start all the services defined in `docker-compose.yml`.

2.  **Running the client:**

    A client is not provided in the `docker-compose.yml`. You can write your own client using the `internal/client` package or run the provided client example (when available (SOON!)).

## Future Work

*   Create a producer and consumer with the client package.
*   Implement a more robust leader election mechanism for the NCL peers.
*   Add support for client-side sharding and caching.
*   Implement garbage collection for the data shards.
*   Add more comprehensive testing and benchmarking.
*   Explore different data sharding strategies, implement proper fault tolerance for data.