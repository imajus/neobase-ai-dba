# How to Setup
To setup NeoBase on your server or local machine, you can follow the implementer friendly instructions listed in this file.

## Pre-requisites
Below tech stack should be available with you in order to run NeoBase on your server:
- **Docker**
- **Go (v1.22+)**
- **Node.js (v18+)**
- **MongoDB & Redis instance(NeoBase requires them)**
- **Open AI or Gemini API Key**
- **Any supported Database to use**

## How to Setup

### Frontend/Client setup

1. Cd into `client/` folder.
2. Setup the `.env` file based on `.env.example`.
3. Run `npm install` to install depedencies.
4. Run `npm run dev` to run in dev mode, for release mode use `npm run build`.
5. Alternatively, you can run the client in docker via `docker-compose.yml` available on the root folder.

### Backend setup
1. Cd into `backend/` folder
2. Setup the `.env` file based on `.env.example`.
3. Run `go mod tidy` to install dependencies.
4. Make sure your MongoDB & Redis are up & running.
5. Run `go run cmd/main.go` to run the backend.
6. Alternatively, you can run the backend in docker via `docker-compose.yml` available on the root folder.

## Thank you for using NeoBase!