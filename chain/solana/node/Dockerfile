FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest as solana-build
WORKDIR /root

# install solana CLI build dependencies
ENV PACKAGES perl-FindBin perl-File-Compare systemd-devel gcc-c++ clang-libs clang-devel bzip2
RUN dnf -y install $PACKAGES
RUN curl -fsSLO https://github.com/protocolbuffers/protobuf/releases/download/v27.0/protoc-27.0-linux-$(uname -m | sed 's/x86_64/x86_64/' | sed 's/aarch64/aarch_64/').zip
RUN unzip protoc-*.zip
RUN mv ./bin/protoc /usr/bin/

# download + compile solana release
ENV SOLANA_VERSION 1.17.34
RUN wget https://github.com/solana-labs/solana/archive/refs/tags/v$SOLANA_VERSION.tar.gz
RUN tar xvf v$SOLANA_VERSION.tar.gz
RUN mkdir -p /solana

ENV CARGO_TARGET_DIR=/tmp/target
RUN --mount=type=cache,target=/tmp/target \
    --mount=type=cache,target=/root/.cargo/registry \
    --mount=type=cache,target=/root/.cache \
    cd solana-$SOLANA_VERSION && ln -s ${CARGO_TARGET_DIR} target && ./scripts/cargo-install-all.sh . && mv bin/* /solana

# setup faucet with binaries
FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
COPY --from=solana-build /solana/* /usr/bin/
WORKDIR /root

RUN yum install bzip2 python3 python3-pip -y
RUN pip3 install flask

# generate a wallet to request funds to
RUN solana-keygen new -o /root/.config/solana/id.json --no-bip39-passphrase


WORKDIR /root/solana-local-validator
RUN mkdir -p test-ledger

COPY faucet.py /
ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
# decreasing ticks-per-slot has effect of increasing finality time in tests (default 64)
CMD ["bash", "-c", "(python3 /faucet.py &) ; solana-test-validator --ticks-per-slot 32 --rpc-port ${RPC_PORT}"]