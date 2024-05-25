import os, time
from flask import Flask
from flask import request
import requests

app = Flask(__name__)

FAUCET_PORT = int(os.getenv("FAUCET_PORT", "10001"))
RPC_PORT = int(os.getenv("RPC_PORT", "10000"))

def system(cmd: str):
    r = os.system(cmd)
    if r != 0:
        raise RuntimeError(f"faucet did not succeed ({r}), see logs")

# chains/INJ/assets/denom
# {address: "address-value", amount: "1234"}
@app.route("/chains/<chain_id>/assets/<contract>", methods = ['POST', 'PUT'])
def fund(chain_id:str, contract: str):
    content = request.get_json(force=True)
    amount = content.get('amount', '1')
    address = content['address']
    # https://hardhat.org/hardhat-network/docs/reference#hardhat_setbalance
    r = requests.post(f"http://127.0.0.1:{RPC_PORT}", json={
        "method":"hardhat_setBalance",
        "params":[address, hex(int(amount))],
        "id":1,
        "jsonrpc":"2.0",
    })
    print("response content:\n",r.content, flush=True)
    if 'error' in r.json():
        print("FAILED", flush=True)
        return {}, 500

    return {
    }, r.status_code


if __name__ == "__main__":
    app.run(host="0.0.0.0", port = FAUCET_PORT)
