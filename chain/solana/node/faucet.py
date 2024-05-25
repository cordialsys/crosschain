import os
from flask import Flask, jsonify, request

app = Flask(__name__)

FAUCET_PORT = int(os.getenv("FAUCET_PORT", "10001"))
RPC_PORT = int(os.getenv("RPC_PORT", "10000"))

def system(cmd: str):
    r = os.system(cmd)
    if r != 0:
        raise RuntimeError(f"faucet did not succeed ({r}), see logs")

def set_rpc_url():
    system(f"solana config set --url http://127.0.0.1:{RPC_PORT}")

@app.route("/chains/<chain_id>/assets/<contract>", methods = ['POST', 'PUT'])
def fund(chain_id:str, contract: str):
    content = request.get_json(force=True)
    amount = int(content.get('amount', '1'))
    address = content['address']


    if chain_id == contract:
        # need initial funds gas
        system(f"solana airdrop 1")
        # convert to human + round
        human_amount = round(amount / 10**9, 6)
        # send funds
        system(f"solana airdrop {human_amount} {address}")
    else:
        # only support airdropping WSOL
        if contract != "So11111111111111111111111111111111111111112":
            return jsonify({"code":3,"status":"InvalidArgument", "message": "unsupported contract address"}), 400

        amount = round(amount / 10**9, 6)
        # need initial funds gas + funds to wrap
        system(f"solana airdrop ${amount + 2}")
        try:
            # try to unwrap any existing wsol account first.. solana doesn't let you have multiple.
            system(f"spl-token unwrap")
        except:
            pass
        system(f"spl-token wrap {amount + 1}")
        system(f"spl-token transfer {contract} {amount} {address} --allow-unfunded-recipient --fund-recipient")
    return {
    }


if __name__ == "__main__":
    set_rpc_url()
    app.run(host="0.0.0.0", port = int(os.getenv("FAUCET_PORT", "10001")))
