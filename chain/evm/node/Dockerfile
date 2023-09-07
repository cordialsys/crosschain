FROM node:lts-buster-slim
RUN apt update
RUN apt install git -y
WORKDIR /root
RUN git clone https://github.com/NomicFoundation/hardhat-boilerplate.git
WORKDIR /root/hardhat-boilerplate
RUN npm install
COPY hardhat.config.js .
COPY ./start_node.sh .
RUN chmod +x start_node.sh
# ["./start_node.sh"]
ENTRYPOINT ["/root/hardhat-boilerplate/start_node.sh"]