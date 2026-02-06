# komodo-stats

Grab server stats from Komodo and expose them for Prometheus.
## Usage
1. Generate an API key in Komodo (Settings -> Profile -> API Keys) 
2. Create an `.env` file:
```env
  # Required
  KOMODO_HOST = YOUR_KOMODO_HOST_URL
  KOMODO_API_KEY = YOUR_KOMODO_API_KEY
  KOMODO_API_SECRET = YOUR_KOMODO_API_SECRET
  # Optional 
  # KOMODO_INSECURE_SKIP_VERIFY = false
  # KOMODO_LISTEN_ADDR = :9109
```
3. Create a `compose.yaml`:
```yaml
services:
  komodo-stats:
    image: ghcr.io/zuhayrali/komodo-stats:latest
    container_name: komodo-stats
    environment:
      KOMODO_HOST: "${KOMODO_HOST}"
      KOMODO_API_KEY: "${KOMODO_API_KEY}"
      KOMODO_API_SECRET: "${KOMODO_API_SECRET}"
      KOMODO_INSECURE_SKIP_VERIFY: "${KOMODO_INSECURE_SKIP_VERIFY}"
      KOMODO_LISTEN_ADDR: "${KOMODO_LISTEN_ADDR}"
    ports:
      # Change if you modified KOMODO_LISTEN_ADDR
      - "9109:9109"
```
4. Deploy

    a. Using Docker
   - ```shell
      docker compose up -d
     ```
    b. Using podman
   - ```shell
      podman compose up -d
     ```
