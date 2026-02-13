# Test Environment

Docker Compose environment that runs one of each supported Ethereum beacon node client alongside a shared Geth execution client.

## Prerequisites

- Docker with Compose v2 (`docker compose`)
- OpenSSL (for JWT secret generation)

## Setup

Generate the JWT secret shared between execution and consensus clients:

```bash
./tests/setup.sh
```

This creates `tests/jwt.hex`. The file is gitignored and only needs to be generated once.

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
| Nimbus | `statusim/nimbus-eth2:multiarch-latest` | `http://localhost:5053` | |
| Teku | `consensys/teku:latest` | `http://localhost:5051` | |
| Erigon | `erigontech/erigon:latest` | `http://localhost:5555` | Self-contained EL+CL via Caplin |

All beacon nodes connect to Geth via JWT-authenticated Engine API except Erigon, which runs its own built-in execution layer.

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
