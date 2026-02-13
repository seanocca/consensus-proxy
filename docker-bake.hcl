// Docker Buildx Bake file for consensus-proxy
//
// Usage:
//   Local build:            docker buildx bake
//   Multi-arch + push:      docker buildx bake --set "*.platform=linux/amd64,linux/arm64" --push
//   Specific target:        docker buildx bake consensus-proxy
//   With custom tag:        TAG=v1.2.3 docker buildx bake
//   With custom registry:   REGISTRY=ghcr.io/seanocca docker buildx bake

variable "REGISTRY" {
  default = "ghcr.io/seanocca"
}

variable "IMAGE" {
  default = "consensus-proxy"
}

variable "TAG" {
  default = "latest"
}

variable "RELEASE" {
  default = ""
}

group "default" {
  targets = ["consensus-proxy"]
}

target "consensus-proxy" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
  tags = [
    "${REGISTRY}/${IMAGE}:${TAG}",
    "${REGISTRY}/${IMAGE}:latest",
  ]
  args = {
    release = "${RELEASE}"
  }
  cache-from = ["type=gha"]
  cache-to   = ["type=gha,mode=max"]
}
