import os
import base64
import requests
from flask import Flask, request, Response
import logging

# This proxy is to support combined use of blockbook and native bitcoin endpoints.

app = Flask(__name__)

# these are configured in the Dockerfile
PORT_INTERNAL_RPC = 18443
PORT_INTERNAL_INDEXER = 9999

# Request will be proxied to the indexer if the path contains any of these.
PATHS_INDEXER = ["/api/status", "/api/v3/", "/api/v2/", "/api/v1/"]

PUBLIC_RPC_PORT = int(os.getenv("PUBLIC_RPC_PORT", "10000"))

# BTC RPC needs some boilerplate default auth
USERNAME = os.getenv("USERNAME", "bitcoin")
PASSWORD = os.getenv("PASSWORD", "1234")

def proxy_request(target_port, path, method, headers, data, params, auth: str = None):
    """Proxy request to the target port"""
    url = f"http://localhost:{target_port}{path}"
    # Remove host header to avoid conflicts
    proxy_headers = {k: v for k, v in headers if k.lower() != 'host'}
    if auth is not None:
        proxy_headers["Authorization"] = f"Basic " + base64.b64encode(auth.encode()).decode()
    
    try:
        response = requests.request(
            method=method,
            url=url,
            headers=proxy_headers,
            data=data,
            params=params,
            stream=True
        )
        
        # Create Flask response with the same status code and headers
        flask_response = Response(
            response.content,
            status=response.status_code,
            headers=dict(response.headers)
        )
        return flask_response
        
    except requests.exceptions.RequestException as e:
        return Response(f"Proxy error: {str(e)}", status=500)


@app.route('/', defaults={'path': ''}, methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'])
@app.route('/<path:path>', methods=['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'])
def proxy(path):
    """Proxy all requests based on path"""
    full_path = f"/{path}" if path else "/"
    auth = None
    
    # Determine target port based on path
    if any(path in full_path for path in PATHS_INDEXER):
        target_port = PORT_INTERNAL_INDEXER
        print(f"Proxying to indexer on port {target_port}")
    else:
        target_port = PORT_INTERNAL_RPC
        auth = f"{USERNAME}:{PASSWORD}"
        print(f"Proxying to RPC on port {target_port}")
    
    # Proxy the request
    return proxy_request(
        target_port=target_port,
        path=full_path,
        method=request.method,
        headers=request.headers,
        auth=auth,
        data=request.get_data(),
        params=request.args
    )


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=PUBLIC_RPC_PORT)
