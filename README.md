## Transaction checker
This is third part of the shadowing app

# !!! IMPORTANT !!!

Before runing this app, the hedera local node and smart hedera shadowing smart contract comparison must be running.
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

