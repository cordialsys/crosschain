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

target "bitcoin" {
  dockerfile = "Dockerfile"
  platforms = platforms
  tags = [
    "us-docker.pkg.dev/cordialsys/crosschain/bitcoin:latest",
  ]
  context = "./chain/bitcoin/node/"
  contexts = {
    bitcoin-base = "target:bitcoin-base"
  }
}

// dependency for "bitcoin"
target "bitcoin-base" {
  dockerfile = "base.Dockerfile"
  platforms = platforms
  context = "./chain/bitcoin/node/"
}

target "bittensor" {
  dockerfile = "Dockerfile"
  platforms = platforms
  tags = [
    "us-docker.pkg.dev/cordialsys/crosschain/bittensor:latest",
  ]
  context = "./chain/substrate/node/bittensor"
}

group "default" {
  targets = ["evm", "solana", "cosmos", "bitcoin", "bittensor"]
}