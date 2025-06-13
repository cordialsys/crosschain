FROM debian:bookworm-slim
WORKDIR /root

RUN apt update
RUN apt install -y python3 python3-pip git npm netcat-openbsd curl

# install foundryup
ENV PATH="/root/.foundry/bin:${PATH}"
RUN echo 1
RUN curl -L https://foundry.paradigm.xyz | bash
RUN foundryup

WORKDIR /root/
RUN pip3 install --break-system-packages flask requests

COPY foundry.toml .
COPY ./script ./script
COPY ./src ./src
COPY ./start_node.sh .
RUN chmod +x start_node.sh

RUN forge install foundry-rs/forge-std --no-git
RUN forge build

COPY faucet.py /

# default anvil dev private key
ENV PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

# default chain ID
ENV CHAIN_ID 1000

ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
CMD ["bash", "-c", "(python3 /faucet.py &) ; /root/start_node.sh"]