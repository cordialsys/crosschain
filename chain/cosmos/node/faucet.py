import os, time
from flask import Flask
from flask import request

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
    print(f"REQUEST address={address} contract={contract} amount={amount}", flush=True)

    faucet_alice_addr = "xpla1mmu56rh6syyruc5xeea2f82askyk5tvts8xnqf"
    system(f"exampled tx bank send {faucet_alice_addr} {address} {amount}{contract} --from alice -y --chain-id example --node tcp://127.0.0.1:{RPC_PORT}")
    # wait for block
    time.sleep(3)
    return {
    }


if __name__ == "__main__":
    app.run(host="0.0.0.0", port = FAUCET_PORT)
