import os, time
from flask import Flask
from flask import request

app = Flask(__name__)

FAUCET_PORT = int(os.getenv("FAUCET_PORT", "10001"))
RPC_PORT = int(os.getenv("RPC_PORT", "10000"))

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
    
    # convert sats to BTC
    btc = f"{(int(sats) / 10**8):.8f}"

    print(f"REQUEST address={address} contract={contract} amount={btc}", flush=True)

    username = os.getenv("USERNAME", "")
    password = os.getenv("PASSWORD", "")

    system(f"bitcoin-cli -regtest -rpcuser={username} -rpcpassword={password} sendtoaddress {address} {btc}")
    # wait for block (depends on resync period of blockbook)
    time.sleep(4)

    return {
    }


if __name__ == "__main__":
    app.run(host="0.0.0.0", port = FAUCET_PORT)
