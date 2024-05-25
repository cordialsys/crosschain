FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
WORKDIR /root
RUN git clone https://github.com/NomicFoundation/hardhat-boilerplate.git
WORKDIR /root/hardhat-boilerplate

RUN yum install python3 python3-pip -y
RUN pip3 install flask requests

# pin hardhat version
RUN npm install hardhat@2.19.5
RUN npm install
COPY hardhat.config.js .
COPY ./start_node.sh .
RUN chmod +x start_node.sh

COPY faucet.py /

ENV RPC_PORT 10000
ENV FAUCET_PORT 10001
CMD ["bash", "-c", "(python3 /faucet.py &) ; /root/hardhat-boilerplate/start_node.sh"]