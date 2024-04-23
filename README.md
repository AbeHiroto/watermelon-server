# bribe_server
<img src="https://img.shields.io/badge/-Go-76E1FE.svg?logo=go&style=for-the-badge"> <img src="https://img.shields.io/badge/-Gin-333366.svg?logo=gin&style=for-the-badge"> <img src="https://img.shields.io/badge/-Postgresql-2f2f2f.svg?logo=postgresql&style=for-the-badge">

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