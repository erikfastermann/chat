# Chat

# Getting started

Install Go 1.13 or later.

Then run:

```
mkdir db_1

CHAT_ADDRESS=:5000 \
CHAT_HTTPS_ADDRESS=:5001 \
CHAT_KEY=server.key \
CHAT_CERT=server.crt \
CHAT_DOMAIN=https://localhost:5001 \
CHAT_DB_DIR=db_1 \
CHAT_TEMPLATE_GLOB='template/*' \
go run .
```
