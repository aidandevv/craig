"""Presentation-only Flask UI. It never opens the SQLite database."""

from __future__ import annotations

import os
from urllib.parse import urljoin

import requests
from flask import Flask, Response, abort, redirect, render_template, request

app = Flask(__name__)
API_BASE_URL = os.environ.get("API_BASE_URL", "http://localhost:8080").rstrip("/") + "/"
GRAFANA_URL = os.environ.get("GRAFANA_URL", "http://localhost:3000")


def api_request(method: str, path: str, **kwargs) -> requests.Response:
    try:
        response = requests.request(method, urljoin(API_BASE_URL, path.lstrip("/")), timeout=5, **kwargs)
    except requests.RequestException:
        abort(503, "The core service is unavailable.")
    if response.status_code == 404:
        abort(404)
    if response.status_code >= 400:
        abort(response.status_code, response.text)
    return response


@app.get("/")
def dashboard():
    listings = api_request("GET", "/api/listings", params={"per_page": 100}).json()
    stats = api_request("GET", "/api/stats").json()
    return render_template("dashboard.html", listings=listings["data"], stats=stats)


@app.get("/listings/<int:listing_id>")
def detail(listing_id: int):
    return render_template("detail.html", listing=api_request("GET", f"/api/listings/{listing_id}").json())


@app.get("/api/listings")
def listings_api():
    return proxy_json("GET", "/api/listings", params=request.args)


@app.get("/api/listings/<int:listing_id>")
def listing_api(listing_id: int):
    return proxy_json("GET", f"/api/listings/{listing_id}")


@app.get("/api/stats")
def stats_api():
    return proxy_json("GET", "/api/stats")


@app.post("/api/listings/<int:listing_id>/mark")
def mark_listing(listing_id: int):
    payload = request.get_json(silent=True) or request.form.to_dict()
    return proxy_json("POST", f"/api/listings/{listing_id}/mark", json=payload)


@app.get("/api/metrics")
def metrics():
    return redirect(GRAFANA_URL, code=302)


def proxy_json(method: str, path: str, **kwargs) -> Response:
    response = api_request(method, path, **kwargs)
    return Response(response.content, status=response.status_code, content_type="application/json")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=int(os.environ.get("PORT", "5000")))
