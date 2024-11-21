variable "TAG" {
  default = "latest"
}
variable "platforms" {
  default = ["linux/amd64", "linux/arm64"]
}

target "evm" {
  dockerfile = "Dockerfile"
  platforms = platforms
  tags = [
    "us-docker.pkg.dev/cordialsys/crosschain/evm:latest",
  ]
  context = "./chain/evm/node/"
}

target "solana" {
  dockerfile = "Dockerfile"
  platforms = platforms
  tags = [
    "us-docker.pkg.dev/cordialsys/crosschain/solana:latest",
  ]
  context = "./chain/solana/node/"
}

target "cosmos" {
  dockerfile = "Dockerfile"
  platforms = platforms
  tags = [
    "us-docker.pkg.dev/cordialsys/crosschain/cosmos:latest",
  ]
  context = "./chain/cosmos/node/"
}

group "default" {
  targets = ["evm", "solana", "cosmos"]
}