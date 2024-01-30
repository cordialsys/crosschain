FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
WORKDIR /root
RUN git clone https://github.com/NomicFoundation/hardhat-boilerplate.git
WORKDIR /root/hardhat-boilerplate
# pin hardhat version
RUN npm install hardhat@2.19.5
RUN npm install
COPY hardhat.config.js .
COPY ./start_node.sh .
RUN chmod +x start_node.sh
# ["./start_node.sh"]
ENTRYPOINT ["/root/hardhat-boilerplate/start_node.sh"]