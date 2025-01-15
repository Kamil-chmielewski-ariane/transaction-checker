### Transaction checker
This is third part of the shadowing app

This app is an API implemented by our team that we use to send transaction information for asynchronous check for the transaction status that we are pushing to 
Hedera consensus node via transfer transaction or EthereumTransaction method. Also after the check is completed in there a response is send to our other application (hedera-shadowing-smart-contract-comparison) to check smart contract states between Hedera and Sepolia.

# !!! IMPORTANT !!!

Before runing this app, the hedera local node and hedera shadowing smart contract comparison must be running.
[Hedera Shadowing Smart Contract Comparison](https://github.com/Kamil-chmielewski-ariane/hedera-shadowing-smart-contract-comparison)

# USAGE

Create a ```.env``` file in the root of project and add all variables as in ```.env.example```. Api key for ```OPERATOR_PRIVATE``` should be added from the shadowing

- ``PORT``- port which app will be running on - default is 8081
- ``NETWORK_WORKERS``- max network workers - default 128
- ``MIRROR_WORKERS``- max mirror workers - default is 64
- ``NETWORK_QUEUE_CAPACITY``- max network que capacity - default is 65536
- ``MIRROR_QUEUE_CAPACITY``- max mirror queue capacity - default is 65536
- ``LOG_FILE_PATH``- directory to store logs
- ``NETWORK_URL``- url for the network node api
- ``NETWORK_ACCOUNT``- id for network account
- ``OPERATOR_ACCOUNT``- id for operator account
- ``OPERATOR_ACCOUNT_KEY``- operator key - get this from the shadowing app
- ``MIRROR_NODE_URL``- url for the mirror node api
- ``SHADOWING_API_URL`` - url for the shadowing api - get it from the hedera-shadowing-smart-contract-comparison app

## Docker
``docker compose up -d``

