import os
import time
import sys

# Ensure we can import the SDK if not installed
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '../../../sdks/python')))

from fintech import FintechClient

def main():
    print("🚀 Starting Sapliy Fintech Quickstart (Python)...")

    api_key = os.getenv("API_KEY", "sk_test_123")
    base_url = os.getenv("BASE_URL", "http://localhost:8080")

    client = FintechClient(api_key=api_key, base_url=base_url)

    try:
        # 1. Create a Ledger Account
        account_id = f"acc_{int(time.time())}"
        print(f"\n1. Creating Ledger Account: {account_id}")

        # Note: In a real app, you would use client.payments.create_intent()
        # For now, we simulate using the ledger directly as per the current SDK state.
        
        print("   (Simulating payment confirmation via Ledger Transaction)")
        transaction = client.ledger.record_transaction(
            account_id=account_id,
            amount=1000, # $10.00
            currency="USD",
            description="Quickstart Payment",
            reference_id=f"ref_{int(time.time())}"
        )

        print("✅ Transaction Recorded:", transaction)

        # 2. Check Balance
        print(f"\n2. Checking Balance for Account: {account_id}")
        # Small delay to ensure eventual consistency if needed (though local should be fast)
        time.sleep(1)
        
        account = client.ledger.get_account(account_id)
        
        # The response structure might vary slightly based on the Go backend, 
        # but typically it returns the account object directly.
        balance = account.get('balance', 0)
        currency = account.get('currency', 'USD')
        
        print("💰 Account Details:", account)
        print(f"   Balance: {balance/100:.2f} {currency}")

    except Exception as e:
        print(f"❌ Error: {e}")

if __name__ == "__main__":
    main()
