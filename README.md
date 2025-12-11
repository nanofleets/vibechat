# Vibe Chat

Yeah, I vibe coded a chat app..

## Project Layout

- A docker-compose.yaml file to run the app with a single command for development. This also generates the images to be deployed. But it prod it's all setup to be hot reloaded and seeded with test data

## The App

- When the user opens the app a cookie is checked if they already have a nick chosen, if not they are prompted to choose a nick, which is stored in a cookie. A web socket is then opened to the server to join the chat and receive messages.
- When a user chooses a nick a "device fingerprint" is generated. When hovering over the users nick, this fingerprint is shown. This is to help identify users in the chat.
- Redis and redis pub/sub are used to broadcast messages to all connected clients.
- The header of the app layout is like this: "[vibechat] -> {nick} [socket status}"
- The only page is /, which is a chat interface, on the left are users in the chat, and on the right is the chat messages.
- when a chat message is sent, it's posted to /api/chat/messages (a go api), and is saved to redis (with a ttl of 24 hours) and published to the pub/sub channel, which all connected clients are subscribed to.

## Requirements

- Running `docker-compose up` brings up a full working development environment with hot reloading for redis, the react (use vite), and the go api.
- All components are built to an image with a simple command to push them to a gchr, maybe `make build push`, there will be two images. vibechat-backend, vibechat-frontend
