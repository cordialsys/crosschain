FROM debian:bookworm-slim as bittensor-build

RUN apt update -y
RUN apt install pkg-config libssl-dev curl build-essential protobuf-compiler clang git -y

# prepackaged Rust
ENV RUST_VER=1.82
ENV PATH="/root/.cargo/bin:$PATH"
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain ${RUST_VER}
RUN curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash
RUN cargo version

WORKDIR /root
RUN git clone --depth 1 https://github.com/opentensor/subtensor.git
WORKDIR /root/subtensor
RUN cargo build --workspace --profile=release --features pow-faucet
RUN echo 

RUN mv ./target /target

RUN apt install python3 python3-pip -y
RUN python3 -m venv /btcli
RUN pip3 install bittensor torch flask --break-system-packages

# release image
FROM debian:bookworm-slim
COPY --from=bittensor-build /target/release/node-subtensor /usr/bin/
COPY --from=bittensor-build /root/subtensor /root/subtensor
COPY --from=bittensor-build /btcli /btcli

WORKDIR /root/subtensor
RUN apt update -y
RUN apt install python3 python3-pip pkg-config libssl-dev curl -y
RUN python3 -m venv /btcli

COPY localnet.sh  ./scripts/localnet.sh 
RUN mkdir -p target && ln -s /usr/bin target/release

# RUN ln -s ~/.cargo/bin/ ./target/release
ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
ENV BUILD_BINARY 0

CMD ["./scripts/localnet.sh"]