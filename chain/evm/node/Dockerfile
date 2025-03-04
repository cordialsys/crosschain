FROM debian:bookworm-slim
WORKDIR /root

RUN apt update
RUN apt install -y python3 python3-pip git npm

WORKDIR /root/
RUN pip3 install --break-system-packages flask requests

COPY hardhat.config.js .
COPY ./scripts ./scripts
COPY ./contracts ./contracts
COPY *.json .
COPY ./start_node.sh .
RUN chmod +x start_node.sh

# pin hardhat version
RUN npm install hardhat@2.22.19
RUN npm install

COPY faucet.py /

# default chain ID
ENV CHAIN_ID 1000

ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
CMD ["bash", "-c", "(python3 /faucet.py &) ; /root/start_node.sh"]