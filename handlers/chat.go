package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rs/xid"
	"github.com/stakwork/sphinx-tribes/websocket"

	"github.com/go-chi/chi"
	"github.com/stakwork/sphinx-tribes/db"
)

type ChatHandler struct {
	httpClient *http.Client
	db         db.Database
}

type ChatResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type HistoryChatResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

type ChatHistoryResponse struct {
	Messages []db.ChatMessage `json:"messages"`
}

type StakworkChatPayload struct {
	Name           string                 `json:"name"`
	WorkflowID     int                    `json:"workflow_id"`
	WorkflowParams map[string]interface{} `json:"workflow_params"`
}

type ChatWebhookResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	ChatID  string           `json:"chat_id"`
	History []db.ChatMessage `json:"history"`
}

func NewChatHandler(httpClient *http.Client, database db.Database) *ChatHandler {
	return &ChatHandler{
		httpClient: httpClient,
		db:         database,
	}
}

func (ch *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	var request struct {
		WorkspaceID string `json:"workspaceId"`
		Title       string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if request.WorkspaceID == "" || request.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	chat := &db.Chat{
		ID:          xid.New().String(),
		WorkspaceID: request.WorkspaceID,
		Title:       request.Title,
		Status:      "active",
	}

	createdChat, err := ch.db.AddChat(chat)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create chat: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Chat created successfully",
		Data:    createdChat,
	})
}

func (ch *ChatHandler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chat_id")
	if chatID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Chat ID is required",
		})
		return
	}

	var request struct {
		WorkspaceID string `json:"workspaceId"`
		Title       string `json:"title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	existingChat, err := ch.db.GetChatByChatID(chatID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Chat not found",
		})
		return
	}

	updatedChat := existingChat
	updatedChat.Title = request.Title

	updatedChat, err = ch.db.UpdateChat(&updatedChat)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update chat: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Chat updated successfully",
		Data:    updatedChat,
	})
}

func (ch *ChatHandler) ArchiveChat(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "chat_id")
	if chatID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Chat ID is required",
		})
		return
	}

	existingChat, err := ch.db.GetChatByChatID(chatID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Chat not found",
		})
		return
	}

	updatedChat := existingChat
	updatedChat.Status = db.ArchiveStatus

	updatedChat, err = ch.db.UpdateChat(&updatedChat)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to archive chat: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Chat archived successfully",
		Data:    updatedChat,
	})
}

func (ch *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {

	var request struct {
		ChatID      string `json:"chat_id"`
		Message     string `json:"message"`
		ContextTags []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"contextTags"`
		SourceWebsocketID string `json:"sourceWebsocketId"`
		WorkspaceUUID     string `json:"workspaceUUID"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if request.WorkspaceUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "workspaceUUID is required",
		})
		return
	}

	context, err := ch.db.GetProductBrief(request.WorkspaceUUID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Error retrieving product brief",
		})
		return
	}

	history, err := ch.db.GetChatMessagesForChatID(request.ChatID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch chat history: %v", err),
		})
		return
	}

	start := 0
	if len(history) > 20 {
		start = len(history) - 20
	}
	recentHistory := history[start:]

	messageHistory := make([]map[string]string, len(recentHistory))
	for i, msg := range recentHistory {
		messageHistory[i] = map[string]string{
			"role":    string(msg.Role),
			"content": msg.Message,
		}
	}

	message := &db.ChatMessage{
		ID:        xid.New().String(),
		ChatID:    request.ChatID,
		Message:   request.Message,
		Role:      "user",
		Timestamp: time.Now(),
		Status:    "sending",
		Source:    "user",
	}

	createdMessage, err := ch.db.AddChatMessage(message)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save message: %v", err),
		})
		return
	}

	stakworkPayload := StakworkChatPayload{
		Name:       "Hive Chat Processor",
		WorkflowID: 38842,
		WorkflowParams: map[string]interface{}{
			"set_var": map[string]interface{}{
				"attributes": map[string]interface{}{
					"vars": map[string]interface{}{
						"chatId":            request.ChatID,
						"messageId":         createdMessage.ID,
						"message":           request.Message,
						"history":           messageHistory,
						"contextTags":       context,
						"sourceWebsocketId": request.SourceWebsocketID,
						"webhook_url":       fmt.Sprintf("%s/hivechat/response", os.Getenv("HOST")),
					},
				},
			},
		},
	}

	projectID, err := ch.sendToStakwork(stakworkPayload)
	if err != nil {
		createdMessage.Status = "error"
		ch.db.UpdateChatMessage(&createdMessage)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to process message: %v", err),
		})
		return
	}

	projectMsg := websocket.TicketMessage{
		BroadcastType:   "direct",
		SourceSessionID: request.SourceWebsocketID,
		Message:         fmt.Sprintf("https://jobs.stakwork.com/admin/projects/%d", projectID),
		Action:          "swrun",
	}

	if err := websocket.WebsocketPool.SendTicketMessage(projectMsg); err != nil {
		log.Printf("Failed to send Stakwork project WebSocket message: %v", err)
	}

	wsMessage := websocket.TicketMessage{
		BroadcastType:   "direct",
		SourceSessionID: request.SourceWebsocketID,
		Message:         "Message sent",
		Action:          "process",
		ChatMessage:     createdMessage,
	}

	if err := websocket.WebsocketPool.SendTicketMessage(wsMessage); err != nil {
		log.Printf("Failed to send websocket message: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Message sent successfully",
		Data:    createdMessage,
	})
}

func (ch *ChatHandler) sendToStakwork(payload StakworkChatPayload) (int64, error) {

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("error marshaling payload: %v", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.stakwork.com/api/v1/projects",
		bytes.NewBuffer(payloadJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("error creating request: %v", err)
	}

	apiKey := os.Getenv("SWWFKEY")
	if apiKey == "" {
		return 0, fmt.Errorf("SWWFKEY environment variable not set")
	}

	req.Header.Set("Authorization", "Token token="+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ch.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("stakwork API error: %s", string(body))
	}

	var stakworkResp StakworkResponse
	if err := json.NewDecoder(resp.Body).Decode(&stakworkResp); err != nil {
		return 0, fmt.Errorf("error decoding response: %v", err)
	}

	return stakworkResp.Data.ProjectID, nil
}

func (ch *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	chatStatus := r.URL.Query().Get("status")

	if workspaceID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "workspace_id query parameter is required",
		})
		return
	}

	chats, err := ch.db.GetChatsForWorkspace(workspaceID, chatStatus)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch chats: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Data:    chats,
	})
}

func (ch *ChatHandler) GetChatHistory(w http.ResponseWriter, r *http.Request) {
	chatID := chi.URLParam(r, "uuid")
	if chatID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Chat ID is required",
		})
		return
	}

	messages, err := ch.db.GetChatMessagesForChatID(chatID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch chat history: %v", err),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HistoryChatResponse{
		Success: true,
		Data:    messages,
	})
}

func (ch *ChatHandler) ProcessChatResponse(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Value struct {
			ChatID            string `json:"chatId"`
			MessageID         string `json:"messageId"`
			Response          string `json:"response"`
			SourceWebsocketID string `json:"sourceWebsocketId"`
		} `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	chatID := request.Value.ChatID
	response := request.Value.Response
	sourceWebsocketID := request.Value.SourceWebsocketID

	if chatID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: "ChatID is required for message creation",
		})
		return
	}

	message := &db.ChatMessage{
		ID:        xid.New().String(),
		ChatID:    chatID,
		Message:   response,
		Role:      "assistant",
		Timestamp: time.Now(),
		Status:    "sent",
		Source:    "agent",
	}

	createdMessage, err := ch.db.AddChatMessage(message)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(ChatResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to save response message: %v", err),
		})
		return
	}

	wsMessage := websocket.TicketMessage{
		BroadcastType:   "direct",
		SourceSessionID: sourceWebsocketID,
		Message:         "Response received",
		Action:          "message",
		ChatMessage:     createdMessage,
	}

	if err := websocket.WebsocketPool.SendTicketMessage(wsMessage); err != nil {
		log.Printf("Failed to send websocket message: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ChatResponse{
		Success: true,
		Message: "Response processed successfully",
		Data:    createdMessage,
	})
}
