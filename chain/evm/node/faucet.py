import os, time
from flask import Flask
from flask import request
import requests

app = Flask(__name__)

FAUCET_PORT = int(os.getenv("FAUCET_PORT", "10001"))
RPC_PORT = int(os.getenv("RPC_PORT", "10000"))
PRIVATE_KEY = os.getenv("PRIVATE_KEY")

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
    rpc_url = f"http://127.0.0.1:{RPC_PORT}"

    if len(contract) > 16:
        system(f"bash -c 'CONTRACT={contract} AMOUNT={amount} TO={address} forge script ./script/MintToken.s.sol --broadcast --private-key {PRIVATE_KEY} --rpc-url {rpc_url}'")
        return {}
    elif chain_id == contract:

        # read the current balance
        r = requests.post(rpc_url, json={
            "jsonrpc": "2.0",
            "method": "eth_getBalance",
            "params": [address, "latest"],
            "id": 1
        })
        current_balance_wei = int(r.json()['result'], 16)

        # add to our current balance
        new_balance = current_balance_wei + int(amount)

        # https://hardhat.org/hardhat-network/docs/reference#hardhat_setbalance
        r = requests.post(rpc_url, json={
            "method":"hardhat_setBalance",
            "params":[address, hex(int(new_balance))],
            "id":1,
            "jsonrpc":"2.0",
        })
        print("response content:\n",r.content, flush=True)
        if 'error' in r.json():
            print("FAILED", flush=True)
            return {}, 500

        return {}, r.status_code
    else:
        print("FAILED", flush=True)
        return {"message": "asset not supported"}, 400




if __name__ == "__main__":
    app.run(host="0.0.0.0", port = FAUCET_PORT)
