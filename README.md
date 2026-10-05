# avalanche-snippets
Two small Go programs for Avalanche, in `snippets/`:

- `c-to-x/`: moves AVAX from your C-Chain to your X-Chain balance (the X-Chain pays the token creation fee).
- `ant/`: creates an Avalanche Native Token (ANT) on the X-Chain, or mints more of one.

## Run them in GitHub Codespaces (nothing to install)

1. On this repo's GitHub page, click **Code → Codespaces → Create codespace on main**. Wait for the setup to finish; it installs Go and the dependencies.
2. Open `snippets/.env` (created for you from `.env.example`) and paste a **testnet** private key after `PRIVATE_KEY=`. Never use a key that holds real funds, and never commit `.env`.
3. Get test AVAX for that key's C-Chain address from the Fuji faucet: https://core.app/tools/testnet-faucet/
4. In the terminal:

   ```sh
   cd snippets/c-to-x && go run .   # move 0.05 AVAX (C_TO_X_AMOUNT) to the X-Chain
   cd ../ant && go run .            # create your token, prints its asset ID
   ```

Change `TOKEN_NAME`, `TOKEN_SYMBOL`, `TOKEN_DECIMALS` and `TOKEN_SUPPLY` in `.env` to define your own token. Everything runs on Fuji testnet unless you set `NETWORK=mainnet`.

## Run them locally

Install Go 1.27+, copy `snippets/.env.example` to `snippets/.env`, fill in the key, then run the same commands.
