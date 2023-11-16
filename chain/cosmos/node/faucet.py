import os, time
from flask import Flask

app = Flask(__name__)

def system(cmd: str):
    r = os.system(cmd)
    if r != 0:
        raise RuntimeError(f"faucet did not succeed ({r}), see logs")

@app.route("/fund/<denom>/<address>/<int:amount>")
def fund(denom:str, address: str, amount: int):
    faucet_alice_addr = "xpla1mmu56rh6syyruc5xeea2f82askyk5tvts8xnqf"
    system(f"exampled tx bank send {faucet_alice_addr} {address} {amount}{denom} --from alice -y")
    # wait for block
    time.sleep(3)
    return {
    }


if __name__ == "__main__":
    app.run(host="0.0.0.0", port = 26658)
