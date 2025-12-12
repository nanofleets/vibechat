# Vibe Chat Example App

Example app for NOA/Nanofleets.

## Deploying to a NOA cluster

Apps are deployed to NOA as containers. There are images for vibechat already built at [text](https://github.com/orgs/nanofleets/packages?tab=packages&q=vibechat) that we'll use.

There are two containers for vibechat, backend and frontend. Redis is required as well.

If you want to try this out, create your own free NOA cluster at [nanofleets.com](https://nanofleets.com), setup the CLI, and run these commands:

```sh
# I suggest having the console open while deploying to see the progress
noa console open

# Add redis (a "named instance" can be used for persistant storage, we'll just use the overlay fs for now)
noa app add redis

# Add the vibechat backend
noa app add ghcr.io/nanofleets/vibechat-backend:latest -e REDIS_URL=redis:6379 --path=/api

# Add the frontend
noa app add ghcr.io/nanofleets/vibechat-frontend:latest --path=/

# See if it works! Go to your cluster, you can get your cluster url with
noa status

# Finally, open the console and go to "Gateway", leave this open and run some bots
docker run --rm ghcr.io/nanofleets/vibechat-utils:latest bots --count=5 --interval=10 \
  --url=https://sypq.nanofleets.com
```