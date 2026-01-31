# Node.js SDK Examples

This directory contains examples of how to use the Sapliy Fintech SDK for Node.js.

## Prerequisites

- Node.js 16+
- A running instance of the Fintech Ecosystem (or access to one)
- An API Key (sk_test_...)

## Examples

| Example | Description |
|---------|-------------|
| [01-quickstart](./01-quickstart) | Basic flow: Create a payment intent and confirm it, then check the ledger. |

## Setup

1. Install dependencies in the example folder:
   ```bash
   cd 01-quickstart
   npm install
   ```

2. Run the example:
   ```bash
   API_KEY=sk_test_... npm start
   ```
