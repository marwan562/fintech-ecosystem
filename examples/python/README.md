# Python SDK Examples

This directory contains examples of how to use the Sapliy Fintech SDK for Python.

## Prerequisites

- Python 3.7+
- A running instance of the Fintech Ecosystem (or access to one)
- An API Key (sk_test_...)

## Examples

| Example | Description |
|---------|-------------|
| [01-quickstart](./01-quickstart) | Basic flow: Create a payment intent and confirm it, then check the ledger. |

## Setup

1. Install dependencies:
   ```bash
   # Install the SDK from the local path
   pip install -e ../../sdks/python
   ```

2. Run the example:
   ```bash
   export API_KEY=sk_test_...
   python 01-quickstart/main.py
   ```
