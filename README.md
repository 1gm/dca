# dca

A simple DCA tool written to buy Bitcoin at market rates on Kraken.com. Written to cut down on transaction fees caused by 
Kraken's recurring fee implementation. 

**This is still a WIP - the intent is to run this process on a scheduled interval (daily)**.

### Development

This isn't made available for non-developer use. It's probably going to serve more as an example on how to interact
with the Kraken API.

To run the application you should define these environment variables (you can create a .env file with this content):

```bash
ENABLE_LOGGING=true
KRAKEN_API_KEY=zxcvzxcv
KRAKEN_PRIVATE_KEY=zzxcvzxcvz==
ORDER_AMOUNT_CENTS=100
```

When the following environment variables are defined they will be used when access AWS resources

```bash
AWS_ACCESS_KEY_ID=youracceskeyid
AWS_SECRET_ACCESS_KEY=yoursecretaccesskey
AWS_REGION=us-east-1
```

AWS resources are accessed when environment variables are prefixed with either: `awssm:` or `awsssme:` the former indicating
that the resource to be read is from AWS Systems Manager and the latter that it's an encrypted value in AWS Systems Manager. 

Example values include:

```aiignore
KRAKEN_API_KEY=awsssm:/path/to/my/plaintext/value
KRAKEN_PRIVATE_KEY=awsssme:/path/to/my/encrypted/value
```

#### API Key permissions

In order to work with the *[Add Order](https://docs.kraken.com/api/docs/rest-api/add-order/)* API you need a key with permissions
to Create & Modify orders (located under the Orders and Trades permissions).

### Differences vs Recurring Orders

There's a difference in fees accrued and volume. 

#### volume difference

The aim is to buy at least X amount (in cents) of Bitcoin so the system places a market order at asking price. 
This means sometimes the amount purchased is higher or lower than intended but will always exceed the outcomes provided
by the recurring fee feature (you'll get more BTC for your $$).

#### fee difference

The fees incurred will be those caused the taker fees associated with Kraken's [Spot Crypto](https://www.kraken.com/features/fee-schedule)
instead of the 1.5% fee incurred by the recurring buy feature.


Example of the recurring buy feature

<img src="docs/imgs/recurring-buy-example.png" />

vs the spot API

<img src="docs/imgs/market-order-example.png" />

