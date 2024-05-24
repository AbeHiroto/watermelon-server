# Online Tic-Tac-Toe "BRIBE" Server

## Overview
This demo project is the backend server for a [two-player online Tic-Tac-Toe web app "BRIBE"](https://github.com/AbeHiroto/bribe). I implemented a registration-free matching system using JWT tokens and invitation URLs. **In addition to that, "bribe" system makes this game definitely funnier than conventional tic-tac-toe!**

## Built with natural language
As a complete beginner with no prior programming experience, I started this project after just a basic tutorial in Go. With the help of ChatGPT, I managed to build this server using natural language processing. Although the project has a strong element of being a joke app, I took serious steps to ensure the gameplay process was enjoyable and implemented real-time battle through WebSocket communication. This project allowed me to learn basic development procedures, and I look forward to creating more enjoyable and beneficial projects in the future.

## Features
- **Bribe System:** You can give a bribe or accuse unfair judge!
- **Token-based Authentication:** Securely manage user sessions and game states using JWT.
- **Invitation Link System:** Allows users to join games without needing to register, just by clicking a link.

## Technologies and Libraries
<img src="https://img.shields.io/badge/-Go-76E1FE.svg?logo=go&style=for-the-badge"> <img src="https://img.shields.io/badge/-Gin-333366.svg?logo=gin&style=for-the-badge"> <img src="https://img.shields.io/badge/-Postgresql-2f2f2f.svg?logo=postgresql&style=for-the-badge">

**Important Libraries Used:**
- Logging: `go.uber.org/zap`
- ORM: `gorm.io/gorm`
- Real-time Communication: `github.com/gorilla/websocket`
- Token Management: `github.com/golang-jwt/jwt v3.2.1+incompatible`

## Directories
- main.go
- auth/         (Validation of JWT)
- bribe/
  - actions/    (Handle client's actions)
  - broadcast/  (Broadcast game state to clients)
  - connection/ (Manage game instances and websocket connections)
  - database/  （Manage Session ID with Redis）
- handlers/     (Handlers of screen state and websocket connections)
- middleware/   (Manage JWT)
- models/
- screens/      (Handle HTTP requests )
- utils/        (Initialization of Cron and Logger)

## Future Prospects
The frontend of this web app is currently being developed as a Flutter web application. The goal for this phase was to ensure that all features function correctly.
