FROM bitcoin-base as btc-builder
# use same WORKDIR as builder

RUN apt-get update
RUN apt-get upgrade -y
RUN apt-get install -y devscripts debhelper make dh-exec zstd
RUN apt-get clean

RUN git clone --depth 1 --branch v0.4.0 https://github.com/trezor/blockbook.git
RUN mv blockbook /src
RUN mkdir /out
RUN find /src/build/docker

# builder image expects these files in this dir
RUN cp /src/build/docker/deb/build-deb.sh /build/
RUN cp /src/build/docker/bin/Makefile /build/

RUN PACKAGER=$(id -u):$(id -g) /build/build-deb.sh all bitcoin_regtest

RUN find /out/

# now copy artifiacts to release image
FROM ruimarinho/bitcoin-core:23

COPY --from=btc-builder /out/backend-bitcoin-regtest* /
COPY --from=btc-builder  /out/blockbook-bitcoin-regtest* /

RUN apt update -y
# blockbook deps
RUN apt install -y logrotate libsnappy1v5 libzmq5 psmisc libgflags2.2 git

# our flask faucet app deps
RUN apt install -y python3 python3-pip
RUN pip3 install flask requests

WORKDIR /app
COPY blockchaincfg.json blockchaincfg.json
COPY miner.sh /
COPY faucet.py /
COPY proxy.py /
RUN chmod +x /miner.sh

RUN dpkg -i /backend-bitcoin-regtest*
RUN dpkg -i /blockbook-bitcoin-regtest*

ENV INTERNAL_RPC_PORT=18443
ENV INTERNAL_INDEXER_PORT=9999

# These need to match what's in blockchaincfg.json
ENV USERNAME=bitcoin
ENV PASSWORD=1234

EXPOSE 9030 9130

# 18443 is default rpc port for regtest, which is kept internal as we use blockbook as our "RPC".
CMD [ "/bin/bash", "-c", "\
    bitcoind -printtoconsole -txindex -regtest=1 -fallbackfee=0.001 -rpcallowip=0.0.0.0/0 -rpcbind -rpcport=$INTERNAL_RPC_PORT -rpcuser=$USERNAME -rpcpassword=$PASSWORD & \
    (python3 /faucet.py &) ; \
    (python3 /proxy.py &) ; \
    (/miner.sh &) ; \
    cd /opt/coins/blockbook/bitcoin_regtest/ && ./bin/blockbook -sync -enablesubnewtx -blockchaincfg=/app/blockchaincfg.json -internal=0.0.0.0:9030 -public=0.0.0.0:$INTERNAL_INDEXER_PORT -resyncindexperiod=2500 -resyncmempoolperiod=2500 -workers=1 -logtostderr\
    "]
