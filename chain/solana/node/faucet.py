import os
from flask import Flask

app = Flask(__name__)

def system(cmd: str):
    r = os.system(cmd)
    if r != 0:
        raise RuntimeError(f"faucet did not succeed ({r}), see logs")

@app.route("/fund/SOL/<address>/<int:amount>")
def fund_sol(address: str, amount: int):
    # convert to human + round
    amount = round(amount / 10**9, 6)
    # need initial funds gas
    system(f"solana airdrop 1")
    system(f"solana airdrop {amount} {address}")
    return {
    }

@app.route("/fund/WSOL/<address>/<int:amount>")
def fund_wsol(address: str, amount: int):
    amount = round(amount / 10**9, 6)
    contract = "So11111111111111111111111111111111111111112"
    system(f"solana airdrop {amount + 2}")
    try:
        # try to unwrap any existing wsol account first.. solana doesn't let you have multiple.
        system(f"spl-token unwrap")
    except:
        pass
    system(f"spl-token wrap {amount + 1}")
    system(f"spl-token transfer {contract} {amount} {address} --allow-unfunded-recipient --fund-recipient")
    return {
        "contract": contract,
    }

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8898)
