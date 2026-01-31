import { FintechClient } from 'fintech-node-sdk';
import * as dotenv from 'dotenv';

dotenv.config();

const API_KEY = process.env.API_KEY || 'sk_test_123';
const BASE_URL = process.env.BASE_URL || 'http://localhost:8080';

async function main() {
    console.log('🚀 Starting Sapliy Fintech Quickstart...');

    const client = new FintechClient(API_KEY, BASE_URL);

    try {
        // 1. Create a Ledger Account (if not exists)
        // For simplicity, we'll try to get one or create a new one.
        // In a real app, you'd store this ID.
        const accountId = `acc_${Date.now()}`;
        console.log(`\n1. Creating Ledger Account: ${accountId}`);

        // Note: The SDK currently only has RecordTransaction and GetAccount.
        // We'll simulate a payment flow by recording a transaction directly for now
        // until the Payment Intent API is added to the SDK.

        // In the future v3.x SDK, this would be:
        // const intent = await client.payments.createIntent({ ... });

        console.log('   (Simulating payment confirmation via Ledger Transaction)');
        const transaction = await client.ledger.recordTransaction({
            accountId: accountId,
            amount: 1000, // $10.00
            currency: 'USD',
            description: 'Quickstart Payment',
            referenceId: `ref_${Date.now()}`,
        });

        console.log('✅ Transaction Recorded:', transaction);

        // 2. Check Balance
        console.log(`\n2. Checking Balance for Account: ${accountId}`);
        const account = await client.ledger.getAccount(accountId);

        console.log('💰 Account Details:', account);
        console.log(`   Balance: ${(account.balance / 100).toFixed(2)} ${account.currency}`);

    } catch (error) {
        console.error('❌ Error:', error);
    }
}

main();
