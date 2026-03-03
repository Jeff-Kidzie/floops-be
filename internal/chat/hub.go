package chat

var GlobalHub *Hub

type Hub struct {
	clients    map[int]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage
}

type BroadcastMessage struct {
	ConversationID int
	Data           []byte
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[int]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			convo := h.clients[client.ConversationID]
			if convo == nil {
				convo = make(map[*Client]bool)
				h.clients[client.ConversationID] = convo
			}
			convo[client] = true

		case client := <-h.unregister:
			convo := h.clients[client.ConversationID]
			if convo != nil {
				if _, ok := convo[client]; ok {
					delete(convo, client)
					close(client.Send)
					if len(convo) == 0 {
						delete(h.clients, client.ConversationID)
					}
				}
			}

		case msg := <-h.broadcast:
			convo := h.clients[msg.ConversationID]
			for client := range convo {
				select {
				case client.Send <- msg.Data:
				default:
					close(client.Send)
					delete(convo, client)
				}
			}
		}
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

func (h *Hub) Broadcast(conversationID int, data []byte) {
	h.broadcast <- BroadcastMessage{ConversationID: conversationID, Data: data}
}
