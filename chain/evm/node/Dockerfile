FROM debian:bookworm-slim
WORKDIR /root

RUN apt update
RUN apt install -y python3 python3-pip git npm

RUN git clone https://github.com/NomicFoundation/hardhat-boilerplate.git
WORKDIR /root/hardhat-boilerplate

RUN pip3 install --break-system-packages flask requests

# pin hardhat version
RUN npm install hardhat@2.22.16
RUN npm install
COPY hardhat.config.js .
COPY ./start_node.sh .
RUN chmod +x start_node.sh

COPY faucet.py /

# default chain ID
ENV CHAIN_ID 1000

ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
CMD ["bash", "-c", "(python3 /faucet.py &) ; /root/hardhat-boilerplate/start_node.sh"]