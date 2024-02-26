package websocket

// handleDisconnection handles the disconnection of a client
func (client *Client) handleDisconnection() {
	// Handle client disconnection
	// Update player's online state
	UpdatePlayerState(client.PlayerID, false)
	// Cleanup resources
}

// readPump listens for messages from the client
func (client *Client) readPump() {
	defer client.handleDisconnection()
	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			// Handle error (e.g., connection lost)
			break
		}
		handleMessage(client, message)
	}
}
