package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"hackathon-backend/db"
	"hackathon-backend/model"
)

// GetConversations はログインユーザーが会話したことのある相手一覧を、最新メッセージ付きで返す
func GetConversations(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var messages []model.Message
	db.DB.Preload("Sender").Preload("Receiver").
		Where("sender_id = ? OR receiver_id = ?", userID, userID).
		Order("created_at DESC").
		Find(&messages)

	type Conversation struct {
		Partner     model.User    `json:"partner"`
		LastMessage model.Message `json:"last_message"`
		UnreadCount int           `json:"unread_count"`
	}

	seen := make(map[uint]bool)
	conversations := make([]Conversation, 0)

	for _, m := range messages {
		partnerID := m.ReceiverID
		partner := m.Receiver
		if m.SenderID != userID {
			partnerID = m.SenderID
			partner = m.Sender
		}
		if seen[partnerID] {
			continue
		}
		seen[partnerID] = true

		var unread int64
		db.DB.Model(&model.Message{}).
			Where("sender_id = ? AND receiver_id = ? AND is_read = ?", partnerID, userID, false).
			Count(&unread)

		conversations = append(conversations, Conversation{
			Partner:     partner,
			LastMessage: m,
			UnreadCount: int(unread),
		})
	}

	return c.JSON(http.StatusOK, conversations)
}

// GetMessages はログインユーザーと指定ユーザーとの間のメッセージ一覧を返す
func GetMessages(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	partnerID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ユーザーIDが不正です")
	}

	var target model.User
	if result := db.DB.First(&target, partnerID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	var messages []model.Message
	db.DB.Preload("Sender").Preload("Receiver").
		Where(
			"(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			userID, partnerID, partnerID, userID,
		).
		Order("created_at ASC").
		Find(&messages)

	// 相手から自分への未読メッセージを既読にする
	db.DB.Model(&model.Message{}).
		Where("sender_id = ? AND receiver_id = ? AND is_read = ?", partnerID, userID, false).
		Update("is_read", true)

	return c.JSON(http.StatusOK, messages)
}

// SendMessage はログインユーザーから指定ユーザーへメッセージを送信する
func SendMessage(c echo.Context) error {
	senderID := c.Get("user_id").(uint)
	receiverID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "ユーザーIDが不正です")
	}

	if senderID == uint(receiverID) {
		return echo.NewHTTPError(http.StatusBadRequest, "自分自身にはメッセージを送れません")
	}

	var target model.User
	if result := db.DB.First(&target, receiverID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&req); err != nil || req.Content == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "メッセージ内容を入力してください")
	}

	message := model.Message{
		SenderID:   senderID,
		ReceiverID: uint(receiverID),
		Content:    req.Content,
	}
	db.DB.Create(&message)
	db.DB.Preload("Sender").Preload("Receiver").First(&message, message.ID)

	return c.JSON(http.StatusCreated, message)
}
