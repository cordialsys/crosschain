FROM debian:bookworm-slim as bittensor-build

RUN apt update -y
RUN apt install -y pkg-config libssl-dev curl build-essential \
    protobuf-compiler clang git libsoup2.4-dev \
    libjavascriptcoregtk-4.0-dev

# prepackaged Rust
ENV RUST_VER=1.85.0
ENV PATH="/root/.cargo/bin:$PATH"
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain ${RUST_VER}
RUN curl -L --proto '=https' --tlsv1.2 -sSf https://raw.githubusercontent.com/cargo-bins/cargo-binstall/main/install-from-binstall-release.sh | bash
RUN cargo version
RUN rustup toolchain install stable-$(uname -m | sed -e 's/amd64/x86_64/' -e 's/arm64/aarch64/')-unknown-linux-gnu
RUN rustup toolchain install $RUST_VER-$(uname -m | sed -e 's/amd64/x86_64/' -e 's/arm64/aarch64/')-unknown-linux-gnu
RUN rustup target add wasm32-unknown-unknown --toolchain $RUST_VER-$(uname -m | sed -e 's/amd64/x86_64/' -e 's/arm64/aarch64/')-unknown-linux-gnu
RUN rustup component add rust-src  --toolchain $RUST_VER-$(uname -m | sed -e 's/amd64/x86_64/' -e 's/arm64/aarch64/')-unknown-linux-gnu

WORKDIR /root
RUN git clone --depth 1 --branch v2.0.4 https://github.com/opentensor/subtensor.git
WORKDIR /root/subtensor
RUN cargo build --workspace --profile=release --features pow-faucet
RUN echo 

RUN mv ./target /target

RUN apt install -y python3 python3-pip python3-venv
RUN apt install -y libcairo2-dev libgtk-3-dev libwebkit2gtk-4.0-dev python3-dev

RUN python3 -m venv /btcli
ENV PATH="/btcli/bin:$PATH"
RUN pip3 install bittensor torch bittensor-cli==9.1.0 --break-system-packages

# release image
FROM debian:bookworm-slim
COPY --from=bittensor-build /target/release/node-subtensor /usr/bin/
COPY --from=bittensor-build /root/subtensor /root/subtensor
COPY --from=bittensor-build /btcli /btcli

WORKDIR /root/subtensor
RUN apt update -y
RUN apt install python3 python3-pip python3-venv pkg-config libssl-dev curl -y
RUN python3 -m venv /btcli
ENV PATH="/btcli/bin:$PATH"
RUN pip3 install flask --break-system-packages

COPY localnet.sh  ./scripts/localnet.sh 
RUN mkdir -p target && ln -s /usr/bin target/release

# generate a wallet for the faucet
RUN btcli wallet new_coldkey --wallet.name xc --no-use-password   --wallet-path /root/.bittensor/wallets/ --n-words 12

# RUN ln -s ~/.cargo/bin/ ./target/release
COPY faucet.py /
ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
ENV BUILD_BINARY 0

# CMD ["./scripts/localnet.sh"]
CMD [ "/bin/bash", "-c", "\
    (python3 /faucet.py &) ; \
    ./scripts/localnet.sh \
    "]