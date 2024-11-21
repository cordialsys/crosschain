FROM debian:bookworm-slim as cosmos-build
WORKDIR /root

RUN apt update
RUN apt install -y git npm curl golang

# prepackaged Go
ENV GO_VER=1.23.1
ENV PATH="/root/go/bin:/usr/local/go/bin:$PATH"
RUN echo curl -fsSLO https://go.dev/dl/go$GO_VER.linux-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/').tar.gz
RUN curl -fsSLO https://go.dev/dl/go$GO_VER.linux-$(uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/').tar.gz
RUN cat go$GO_VER.linux-*.tar.gz  | tar -C /usr/local -xz
RUN go version

# install ignite
RUN curl https://get.ignite.com/cli! | bash

# scaffold a chain, use same prefix as a cosmos chain in xc
RUN ignite scaffold chain example --address-prefix xpla --skip-git

WORKDIR /root/example
COPY config.yml /root/example/config.yml
RUN ignite chain build
RUN ignite chain init

FROM debian:bookworm-slim
COPY --from=cosmos-build /root/go/bin/exampled /usr/bin/
COPY --from=cosmos-build  /root/example /root/example
COPY --from=cosmos-build  /root/.example /root/.example
WORKDIR /root/example

RUN apt update
RUN apt install -y python3 python3-pip
RUN pip3 install --break-system-packages flask

# we should also add wasm + cw20 contract, but ignite is still working on the wasm plugin.

COPY faucet.py /faucet.py

ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
# skip invariant checks as some settings in genesis are broken
CMD ["bash", "-c", "(python3 /faucet.py &) ; exampled start --rpc.laddr tcp://0.0.0.0:${RPC_PORT}"]