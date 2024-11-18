# QRL Proof-Of-Stake Explorer

Quantum Resistant Ledger Proof-of-Stake explorer. It is blazing fast with a modern frontend using NextJS and Golang for the backend, stable and compatible with the Ethereum network. Very easy to setup. Synchronising the blockchain node to MongoDB takes only 3 to 5 seconds, depending on your hardware, network speed, and size of the blockchain. Which makes it incredibly easy debug the system, as you can easily delete the blockchain data from MongoDB and simply restart to sync it again. Saves a lot of time.

There are four components you will need:

quanta-explorer-go containing two components, frontend and server (handler), 
QRLtoMongoDB-PoS - which is a blockchain synchroniser to MongoDB and
Zond node (It can be either external or local). 

Note: These instructions are only for the explorer related components. Are you trying to get your Zond up and running? Visit https://test-zond.theqrl.org/linux.html

Make sure to clone quanta-explorer-go and QRLtoMongoDB-PoS first:

```
git clone https://github.com/moscowchill/qrl-explorer-pos.git
```

#### Requirements
- Install golang, mongodb/mongosh, pm2 packages - check their documentations
- Ubuntu 20.04 LTS system

### Frontend
Now cd into quanta-explorer-go

```
cd quanta-explorer-go/frontend
```

Create the .env files if you don't have them
```
touch .env && touch .env.local
```

#### .env fields

| VARIABLE | VALUE |
| ------ | ------ |
| DATABASE_URL | mongodb://localhost:27017/qrldata?readPreference=primary |
| NEXTAUTH_URL | 127.0.0.1 |
| NEXT_PUBLIC_DOMAIN_NAME | http://localhost:3000 (dev) OR http://your_domain_name.io (prod) |
| NEXT_PUBLIC_HANDLER_URL | http://localhost:8080 (dev) OR http://your_domain_name.io:8443 (prod) |

#### .env.local fields 

| VARIABLE | VALUE |
| ------ | ------ |
| DATABASE_URL | mongodb://localhost:27017/qrldata?readPreference=primary |
| NEXTAUTH_SECRET | YOUR_SECRET |
| ADMIN_PUBLIC_ADDRESS | YOUR_SECRET |
| DOMAIN_NAME | http://localhost:3000 (dev) OR http://your_domain_name.io (prod) |
| HANDLER_URL | http://localhost:8080 (dev) OR http://your_domain_name.io:8443 (prod) |

Next, run the following command
```
npm run build
```

```
pm2 start npm --name "frontend" -- start
```
End of instructions for Frontend

### Server (Handler)

Cd into the server folder and create two .env files if you don't have them: 
```
touch .env.development && touch .env.production
```

#### .env.development fields 

| VARIABLE | VALUE |
| ------ | ------ |
| GIN_MODE | release |
| MONGOURI | mongodb://localhost:27017/qrldata?readPreference=primary |
| HTTP_PORT | :8080 |
| NODE_URL | http://localhost:8545 |

#### .env.production fields

| VARIABLE | VALUE |
| ------ | ------ |
| GIN_MODE | release |
| MONGOURI | mongodb://localhost:27017/qrldata?readPreference=primary |
| CERT_PATH | PATH_TO_CERT |
| KEY_PATH | PATH_TO_KEY |
| HTTPS_PORT | :8443 |
| NODE_URL | http://localhost:8545 |

Now we need to build the main.go file
```
go build main.go
```
Now we have a main executable. We can add it to pm2.

```
pm2 start ./main --name "handler" -- start
```
End of Server (Handler) instructions

### QRLtoMongoDB-PoS (Blockchain to MongoDB Synchroniser) 

Cd into the QRLtoMongoDB-PoS directory
```
touch .env
```

#### .env fields
| VARIABLE | VALUE |
| ------ | ------ |
| MONGOURI | mongodb://localhost:27017 |
| NODE_URL | http://localhost:8545 |

```
go build main.go
```

Now you have a main executable. We can add it to pm2 

```
pm2 start ./main --name "synchroniser" -- start
```

##### Optional
To save the pm2 processes, run the following command
```
pm2 save
```
Great! That is all. The explorer should now be live. (Don't forget to use tmux or pm2 for Zond too!)

## License

MIT
