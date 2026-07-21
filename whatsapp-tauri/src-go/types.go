package main

func ptr(s string) *string { return &s }

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
	Status string `json:"status"`
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
}
