FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
WORKDIR /root

ENV PATH="${PATH}:/root/.local/share/solana/install/active_release/bin" RUST_BACKTRACE=1

RUN yum install bzip2 python3 python3-pip -y
RUN pip3 install flask

# install solana CLI tools
RUN sh -c "$(curl -sSfL https://release.solana.com/v1.17.5/install)"

# generate a wallet to request funds to
RUN solana-keygen new -o /root/.config/solana/id.json --no-bip39-passphrase
# use local node
RUN solana config set --url localhost

COPY faucet.py .

WORKDIR /root/solana-local-validator
RUN mkdir -p test-ledger

# run both the validator + faucet script
EXPOSE 8898 8899

# decreasing ticks-per-slot has effect of increasing finality time in tests (default 64)
CMD ["bash", "-c", "(python3 /root/faucet.py &) ; solana-test-validator --ticks-per-slot 32"]