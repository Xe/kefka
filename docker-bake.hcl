variable "ALPINE_VERSION" { default = "3.23" }
variable "GO_VERSION" { default = "1.26" }

group "default" {
  targets = [
    "sophia",
  ]
}

target "sophia" {
  args = {
    ALPINE_VERSION = null
    GO_VERSION = null
  }
  attest = [
    "type=provenance,mode=max",
    "type=sbom",
  ]
  context = "."
  dockerfile = "./docker/sophia.Dockerfile"
  platforms = [ "linux/amd64", "linux/arm64" ]
  pull = true
  tags = [
    "atcr.io/xeiaso.net/kefka/sophia:latest"
  ]
}
