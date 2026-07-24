package main

func ptr(s string) *string { return &s }

type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PushName    string `json:"pushName,omitempty"`
	Avatar      string `json:"avatar"`
	Status      string `json:"status"`
	PhoneNumber string `json:"phoneNumber"`
	PhoneJID    string `json:"phoneJid,omitempty"`
	LIDJID      string `json:"lidJid,omitempty"`
	Phone       string `json:"phone,omitempty"`
	IsSaved     bool   `json:"isSaved,omitempty"`
}

type Message struct {
	ID        string `json:"id"`
	ChatJID   string `json:"chatJid"`
	SenderID  string `json:"senderId"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
	MediaType string `json:"mediaType,omitempty"`
	IsFromMe  bool   `json:"isFromMe"`
	Revision  int64  `json:"-"`
}

type Profile struct {
	ID       string `json:"id"`
	PushName string `json:"pushName"`
}

type MessagePage struct {
	Messages     []Message `json:"messages"`
	NextCursor   *string   `json:"nextCursor"`
	LatestCursor *string   `json:"latestCursor"`
	HasMore      bool      `json:"hasMore"`
}

type Chat struct {
	ID                   string   `json:"id"`
	Participants         []User   `json:"participants"`
	LastMessage          *Message `json:"lastMessage,omitempty"`
	UnreadCount          int      `json:"unreadCount"`
	IsGroup              bool     `json:"isGroup"`
	Name                 *string  `json:"name,omitempty"`
	Avatar               *string  `json:"avatar,omitempty"`
	IsArchived           bool     `json:"isArchived"`
	IsStarred            bool     `json:"isStarred,omitempty"`
	IsCommunity          bool     `json:"isCommunity,omitempty"`
	LastMessageID        string   `json:"-"`
	LastMessageText      string   `json:"-"`
	LastMessageTimestamp string   `json:"-"`
	LastMessageSender    string   `json:"-"`
	CanSend              bool     `json:"canSend"`
	IsPinned             bool     `json:"isPinned"`
	IsMuted              bool     `json:"isMuted"`
	IsFavorite           bool     `json:"isFavorite"`
	IsAnnouncement       bool     `json:"isAnnouncement"`
	IsSuspended          bool     `json:"isSuspended"`
}
