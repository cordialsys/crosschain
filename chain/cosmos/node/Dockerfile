FROM us-docker.pkg.dev/cordialsys/containers/build-base:latest
WORKDIR /root


RUN yum install python3 python3-pip -y
RUN pip3 install flask

# install ignite
RUN curl https://get.ignite.com/cli! | bash

# scaffold a chain, use same prefix as a cosmos chain in xc
RUN ignite scaffold chain example --address-prefix xpla --skip-git

WORKDIR /root/example
COPY config.yml /root/example/config.yml
RUN ignite chain build
RUN ignite chain init

# we should also add wasm + cw20 contract, but ignite is still working on the wasm plugin.

COPY faucet.py /root/faucet.py

# run both the validator + faucet script
EXPOSE 26657 26658

# skip invariant checks as some settings in genesis are broken
CMD ["bash", "-c", "(python3 /root/faucet.py &) ; exampled start"]