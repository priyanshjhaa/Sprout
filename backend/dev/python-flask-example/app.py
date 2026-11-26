"""A small Flask app for trying Sprout's Python builds."""

import platform

from flask import Flask, jsonify
from markupsafe import escape

app = Flask(__name__)


@app.get("/")
def index():
    return f"<h1>Hello from Flask on Sprout</h1><p>Python {escape(platform.python_version())}</p>"


@app.get("/health")
def health():
    return jsonify(status="ok")
