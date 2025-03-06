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

### Backend setup
1. Cd into `backend/` folder
2. Setup the `.env` file based on `.env.example`.
3. Run `go mod tidy` to install dependencies.
4. Make sure your MongoDB & Redis are up & running.
5. Run `go run cmd/main.go` to run the backend.


### Run via Docker Compose
You can also run the whole application directly via `docker` available in the `docker-compose` folder.
The folder contains various docker-compose for different purpose.
1. `docker-compose-dependencies.yml` contains the dependencies which are required by neobase applications(mongodb, redis). These can be run differently on the server.
2. `docker-compose-exampledbs.yml` contains the example databases which can be used to test these dbs.
3. `docker-compose-local.yml` contains the local setup containing both neobase applications and it's dependencies. You can run this directly on your local machine.
4. `docker-compose-server.yml` contains the server setup with only neobase applications to be run on the server. You may use your own MongoDB & Redis instances for Neobase or use the `docker-compose-dependencies.yml` to setup them.
*** Setup the .env file based on `.env.example` for the docker-compose files to work. ***


## Thank you for using NeoBase!