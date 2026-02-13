# Consensus Proxy Helm Chart

A Helm chart for deploying [Consensus Proxy](https://github.com/seanocca/consensus-proxy) — an Ethereum beacon node load balancer and proxy with intelligent failover, health monitoring, and request routing.

## Prerequisites

- Kubernetes 1.23+
- Helm 3.x

## Installation

```bash
helm install consensus-proxy ./helm
```

With custom values:

```bash
helm install consensus-proxy ./helm -f my-values.yaml
```

## Uninstalling

```bash
helm uninstall consensus-proxy
```

## Configuration

### Beacon Nodes

The primary configuration lives in `configMap.config` as a TOML string. To configure your beacon nodes, override this value:

```yaml
configMap:
  config: |
    [beacons]
    nodes = ["lighthouse-primary", "prysm-backup"]

    [beacons.lighthouse-primary]
    url = "http://lighthouse:5052"
    type = "lighthouse"

    [beacons.prysm-backup]
    url = "http://prysm:3500"
    type = "prysm"

    [failover]
    error_threshold = 5

    [health]
    interval = "30s"
    timeout = "5s"
    successful_checks_for_failback = 3
```

The first node in the `nodes` list is treated as the primary. All subsequent nodes are backups in priority order.

Supported beacon types: `lighthouse`, `prysm`, `nimbus`, `teku`, `erigon`, `infura`, `alchemy`.

### Toggleable Resources

Every Kubernetes resource can be independently enabled or disabled:

| Resource | Value | Default |
|---|---|---|
| Deployment | `deployment.enabled` | `true` |
| Service | `service.enabled` | `true` |
| ConfigMap | `configMap.enabled` | `true` |
| ServiceAccount | `serviceAccount.enabled` | `true` |
| PodDisruptionBudget | `podDisruptionBudget.enabled` | `false` |
| Liveness Probe | `livenessProbe.enabled` | `true` |
| Readiness Probe | `readinessProbe.enabled` | `true` |
| ServiceMonitor | `serviceMonitor.enabled` | `false` |

### Values Reference

#### General

| Key | Description | Default |
|---|---|---|
| `namespace.name` | Deploy all resources into this namespace (defaults to release namespace) | `""` |
| `namespace.create` | Create the Namespace resource | `false` |
| `namespace.annotations` | Namespace annotations | `{}` |
| `namespace.labels` | Namespace labels | `{}` |
| `nameOverride` | Override the chart name | `""` |
| `fullnameOverride` | Override the full release name | `""` |
| `commonLabels` | Labels added to all resources | `{}` |
| `commonAnnotations` | Annotations added to all resources | `{}` |

#### Image

| Key | Description | Default |
|---|---|---|
| `image.repository` | Container image repository | `ghcr.io/zircuit-labs/consensus-proxy` |
| `image.tag` | Image tag (defaults to `appVersion`) | `""` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `imagePullSecrets` | Image pull secrets | `[]` |

#### Deployment

| Key | Description | Default |
|---|---|---|
| `deployment.enabled` | Create the Deployment | `true` |
| `deployment.replicas` | Number of replicas | `1` |
| `deployment.revisionHistoryLimit` | Revision history limit | `3` |
| `deployment.strategy` | Update strategy | `RollingUpdate` |
| `deployment.annotations` | Deployment annotations | `{}` |
| `deployment.labels` | Deployment labels | `{}` |

#### Pod

| Key | Description | Default |
|---|---|---|
| `podAnnotations` | Pod annotations | `{}` |
| `podLabels` | Pod labels | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| `resources` | CPU/memory resource requests and limits | requests: `100m/128Mi`, limits: `1/1Gi` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity rules | `{}` |
| `topologySpreadConstraints` | Topology spread constraints | `[]` |
| `extraEnv` | Additional environment variables | `[]` |
| `extraVolumeMounts` | Additional volume mounts | `[]` |
| `extraVolumes` | Additional volumes | `[]` |
| `extraContainers` | Additional containers (e.g. sidecars) | `[]` |
| `initContainers` | Init containers | `[]` |

#### Service

| Key | Description | Default |
|---|---|---|
| `service.enabled` | Create the Service | `true` |
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8080` |
| `service.annotations` | Service annotations | `{}` |
| `service.labels` | Service labels | `{}` |

#### ServiceAccount

| Key | Description | Default |
|---|---|---|
| `serviceAccount.enabled` | Use a ServiceAccount | `true` |
| `serviceAccount.create` | Create the ServiceAccount | `true` |
| `serviceAccount.name` | ServiceAccount name (generated if empty) | `""` |
| `serviceAccount.annotations` | ServiceAccount annotations | `{}` |
| `serviceAccount.labels` | ServiceAccount labels | `{}` |
| `serviceAccount.automountServiceAccountToken` | Automount API token | `false` |

#### ConfigMap

| Key | Description | Default |
|---|---|---|
| `configMap.enabled` | Create the ConfigMap | `true` |
| `configMap.existingConfigMap` | Use a pre-existing ConfigMap by name (must contain a `config.toml` key) | `""` |
| `configMap.annotations` | ConfigMap annotations | `{}` |
| `configMap.labels` | ConfigMap labels | `{}` |
| `configMap.config` | TOML configuration string | See `values.yaml` |

The ConfigMap is mounted at `/etc/consensus-proxy/config.toml` inside the container. When using the chart-managed ConfigMap, a config checksum annotation on the pod ensures rolling restarts on config changes.

To use a pre-existing ConfigMap, set `configMap.existingConfigMap` to its name. The chart will skip creating its own ConfigMap and reference yours instead. The existing ConfigMap must contain a `config.toml` key.

#### PodDisruptionBudget

| Key | Description | Default |
|---|---|---|
| `podDisruptionBudget.enabled` | Create the PDB | `false` |
| `podDisruptionBudget.minAvailable` | Minimum available pods | `1` |
| `podDisruptionBudget.maxUnavailable` | Maximum unavailable pods | _unset_ |
| `podDisruptionBudget.annotations` | PDB annotations | `{}` |
| `podDisruptionBudget.labels` | PDB labels | `{}` |

#### Probes

| Key | Description | Default |
|---|---|---|
| `livenessProbe.enabled` | Enable liveness probe | `true` |
| `livenessProbe.httpGet.path` | Probe path | `/healthz` |
| `livenessProbe.initialDelaySeconds` | Initial delay | `5` |
| `livenessProbe.periodSeconds` | Check interval | `10` |
| `livenessProbe.timeoutSeconds` | Timeout | `3` |
| `livenessProbe.failureThreshold` | Failures before unhealthy | `3` |
| `readinessProbe.enabled` | Enable readiness probe | `true` |

The readiness probe uses the same defaults as the liveness probe.

#### ServiceMonitor

| Key | Description | Default |
|---|---|---|
| `serviceMonitor.enabled` | Create a Prometheus ServiceMonitor | `false` |
| `serviceMonitor.interval` | Scrape interval | `30s` |
| `serviceMonitor.scrapeTimeout` | Scrape timeout | `10s` |
| `serviceMonitor.path` | Metrics path | `/metrics` |
| `serviceMonitor.annotations` | ServiceMonitor annotations | `{}` |
| `serviceMonitor.labels` | ServiceMonitor labels | `{}` |

## Examples

### Production deployment with multiple beacon nodes

```yaml
deployment:
  replicas: 3

podDisruptionBudget:
  enabled: true
  minAvailable: 2

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

configMap:
  config: |
    [beacons]
    nodes = ["lighthouse-1", "lighthouse-2", "prysm-1"]

    [beacons.lighthouse-1]
    url = "http://lighthouse-1:5052"
    type = "lighthouse"

    [beacons.lighthouse-2]
    url = "http://lighthouse-2:5052"
    type = "lighthouse"

    [beacons.prysm-1]
    url = "http://prysm-1:3500"
    type = "prysm"

    [server]
    port = 8080
    max_retries = 3
    request_timeout = "30s"

    [failover]
    error_threshold = 3

    [metrics]
    enabled = true
    namespace = "consensus_proxy"

    [logger]
    level = "info"
    format = "json"
    output = "stdout"

    [health]
    interval = "15s"
    timeout = "5s"
    successful_checks_for_failback = 5
```

### Using a pre-existing ConfigMap

```yaml
configMap:
  existingConfigMap: my-consensus-proxy-config
```

The existing ConfigMap must contain a `config.toml` key with valid TOML configuration.

### Using a Secret for sensitive config (disable built-in ConfigMap)

```yaml
configMap:
  enabled: false

extraVolumes:
  - name: config
    secret:
      secretName: consensus-proxy-config

extraVolumeMounts:
  - name: config
    mountPath: /etc/consensus-proxy
    readOnly: true
```
