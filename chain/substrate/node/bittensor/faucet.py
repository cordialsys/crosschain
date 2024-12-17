import os, time
from flask import Flask
from flask import request
from threading import Thread
import requests


app = Flask(__name__)

FAUCET_PORT = int(os.getenv("FAUCET_PORT", "10001"))
RPC_PORT = int(os.getenv("RPC_PORT", "10000"))

DID_PoW = False

# The TAO devnet node requires you to submit PoW
# to get funds from the faucet, ~1k TAO at a time.
# We just do this in a loop 100 times.
def run_tao_faucet():
    global DID_PoW
    # wait for node to start RPC
    while True:
        try:
            requests.get(f"http://localhost:{RPC_PORT}/")
        except requests.exceptions.ConnectionError:
            print("not yet started..")
            time.sleep(0.5)
            continue
        break
    for i in range(0,100):
        try:
            print("requesting from faucet")
            system(f"btcli wallet faucet --wallet.name xc --subtensor.chain_endpoint ws://localhost:{RPC_PORT} --max-successes 1 -y")
            DID_PoW = True
        except Exception as e:
            print("could not mine from faucet", e)
        time.sleep(1)

def system(cmd: str):
    print("running:\n"+cmd)
    r = os.system(cmd)
    if r != 0:
        raise RuntimeError(f"faucet did not succeed ({r}), see logs")

@app.route("/chains/<chain_id>/assets/<contract>", methods = ['POST', 'PUT'])
def fund(chain_id:str, contract: str):
    content = request.get_json(force=True)
    sats = content.get('amount', '1')
    address = content['address']
    
    # convert RAO to TAO
    tao = f"{(int(sats) / 10**9):.8f}"
    while DID_PoW is False:
        print("waiting for initial TAO mining...")
        time.sleep(1)


    print(f"REQUEST address={address} contract={contract} amount={tao}", flush=True)

    username = os.getenv("USERNAME", "")
    password = os.getenv("PASSWORD", "")

    system(f"btcli wallet transfer --amount {tao} --destination {address} -y --wallet-name xc --subtensor.chain_endpoint ws://localhost:{RPC_PORT}")
    # wait for block (12s block time)
    time.sleep(12.5)

    return {
    }


if __name__ == "__main__":
    thread = Thread(target = run_tao_faucet)
    thread.start()
    app.run(host="0.0.0.0", port = FAUCET_PORT)
    thread.join()
