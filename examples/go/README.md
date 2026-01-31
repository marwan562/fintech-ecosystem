# Go SDK Examples

This directory contains examples of how to use the Sapliy Fintech SDK for Go.

## Prerequisites

- Go 1.24+
- A running instance of the Fintech Ecosystem (or access to one)
- An API Key (sk_test_...)

## Examples

| Example | Description |
|---------|-------------|
| [01-quickstart](./01-quickstart) | Basic flow: Create a payment intent (simulated) and check the ledger. |

## Setup

1. Run the example:
   ```bash
   cd 01-quickstart
   export API_KEY=sk_test_...
   go run main.go
   ```
