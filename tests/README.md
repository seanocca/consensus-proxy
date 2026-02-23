# Test Environment

Docker Compose environment that runs one of each supported Ethereum beacon node client alongside a shared Geth execution client.

## Prerequisites

- Docker with Compose v2 (`docker compose`)
- OpenSSL (for JWT secret generation)
- curl (for downloading genesis state files)

## Setup

Run the setup script to generate the JWT secret and detect your CPU architecture:

```bash
./tests/setup.sh
```

This creates:
- `tests/jwt.hex` — JWT secret shared between execution and consensus clients
- `tests/.env` — architecture variable for images without proper multi-arch support
- `tests/genesis/` — genesis state files needed by Prysm for testnet networks

All generated files are gitignored and only need to be created once.

## Starting

Start all services on **Holesky** (default):

```bash
docker compose -f tests/docker-compose.yaml up -d
```

Or on **Sepolia**:

```bash
NETWORK=sepolia docker compose -f tests/docker-compose.yaml up -d
```

## Services

| Service | Image | Beacon API | Notes |
|---|---|---|---|
| Geth | `ethereum/client-go:stable` | Engine API `localhost:8551`, RPC `localhost:8545` | Shared execution client |
| Lighthouse | `sigp/lighthouse:latest-modern` | `http://localhost:5052` | |
| Prysm | `gcr.io/prysmaticlabs/prysm/beacon-chain:latest` | `http://localhost:3500` | |
| Nimbus | `statusim/nimbus-eth2:{arch}-v25.9.2` | `http://localhost:5053` | Arch-specific tag; pinned for Holesky support |
| Teku | `consensys/teku:latest` | `http://localhost:5051` | |
| Erigon | `erigontech/erigon:v3.2.3` | `http://localhost:5555` | Self-contained EL+CL via Caplin; pinned for Holesky support |

All beacon nodes connect to Geth via JWT-authenticated Engine API except Erigon, which runs its own built-in execution layer.

### Version pinning

Nimbus and Erigon are pinned to specific versions because their latest releases have dropped Holesky support following the [Holesky shutdown announcement](https://blog.ethereum.org/2025/09/01/holesky-shutdown-announcement). Update these versions when migrating to a different testnet.

### Architecture support

Most images publish proper multi-arch manifests and work on both `amd64` and `arm64` without any changes. Nimbus is the exception — it uses arch-specific tags (`amd64-v25.9.2` / `arm64-v25.9.2`). The `setup.sh` script detects your architecture and writes the correct value to `tests/.env` automatically.

## Example consensus-proxy config

```toml
[beacons]
nodes = ["lighthouse", "prysm", "nimbus", "teku", "erigon"]

[beacons.lighthouse]
url = "http://localhost:5052"
type = "lighthouse"

[beacons.prysm]
url = "http://localhost:3500"
type = "prysm"

[beacons.nimbus]
url = "http://localhost:5053"
type = "nimbus"

[beacons.teku]
url = "http://localhost:5051"
type = "teku"

[beacons.erigon]
url = "http://localhost:5555"
type = "erigon"
```

## Useful commands

```bash
# View logs for a specific service
docker compose -f tests/docker-compose.yaml logs -f lighthouse

# Restart a single service
docker compose -f tests/docker-compose.yaml restart prysm

# Check sync status of a beacon node
curl -s http://localhost:5052/eth/v1/node/syncing | jq

# Stop and remove all containers and volumes
docker compose -f tests/docker-compose.yaml down -v
```
