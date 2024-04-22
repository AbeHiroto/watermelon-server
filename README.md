# bribe_server

- main.go
- auth/ (Validation of JWT)
- bribe/
  - actions/ (Handle client's actions)
  - broadcast/ (Broadcast game state to clients)
  - connection/ (Manage game instances and websocket connections)
  - database/ （Manage Session ID with Redis）
- handlers/
- middleware/ (Manage JWT)
- models/
- screens/ 
- utils/ (Initialization of Cron and Logger)