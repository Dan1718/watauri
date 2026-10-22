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
	IsSaved     bool   `json:"isSaved,omitempty"`
	Rank        *int   `json:"rank,omitempty"` // Only populated for group participants.
}

type Message struct {
	ID            string   `json:"id"`
	ClientSendID  string   `json:"clientSendId,omitempty"`
	ChatJID       string   `json:"chatJid"`
	SenderID      string   `json:"senderId"`
	Text          string   `json:"text"`
	Timestamp     string   `json:"timestamp"`
	Status        string   `json:"status"`
	MediaType     string   `json:"mediaType,omitempty"`
	IsFromMe      bool     `json:"isFromMe"`
	MentionedJIDs []string `json:"mentionedJids"`
	Revision      int64    `json:"-"`
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
	OlderCursor  *string   `json:"olderCursor"`
	NewerCursor  *string   `json:"newerCursor"`
	HasOlder     bool      `json:"hasOlder"`
	HasNewer     bool      `json:"hasNewer"`
}

type GroupMetadata struct {
	Description            *string `json:"description,omitempty"`
	OwnerJID               *string `json:"ownerJid,omitempty"`
	CreatedAt              *string `json:"createdAt,omitempty"`
	IsLocked               bool    `json:"isLocked"`
	IsAnnouncement         bool    `json:"isAnnouncement"`
	IsEphemeral            bool    `json:"isEphemeral"`
	DisappearingTimer      *int    `json:"disappearingTimer,omitempty"`
	IsJoinApprovalRequired bool    `json:"isJoinApprovalRequired"`
	ParentGroupJID         *string `json:"parentGroupJid,omitempty"`
	LinkedParentJID        *string `json:"linkedParentJid,omitempty"`
	IsDefaultSubgroup      bool    `json:"isDefaultSubgroup"`
	IsSuspended            bool    `json:"isSuspended"`
	IsDeleted              bool    `json:"isDeleted"`
}

// GroupMetadataPatch only updates fields explicitly present in an incoming
// history-sync, snapshot, or live group-info event.
type GroupMetadataPatch struct {
	Description            *string
	OwnerJID               *string
	OwnerJIDFallback       bool
	CreatedAt              *string
	IsLocked               *bool
	IsAnnouncement         *bool
	IsEphemeral            *bool
	DisappearingTimer      *int
	IsJoinApprovalRequired *bool
	ParentGroupJID         *string
	LinkedParentJID        *string
	IsDefaultSubgroup      *bool
	IsSuspended            *bool
	IsDeleted              *bool
}

type Chat struct {
	ID                   string         `json:"id"`
	Participants         []User         `json:"participants"`
	LastMessage          *Message       `json:"lastMessage,omitempty"`
	UnreadCount          int            `json:"unreadCount"`
	UnreadCountSet       bool           `json:"-"` // Set when UnreadCount is an authoritative sync value.
	IsGroup              bool           `json:"isGroup"`
	IsArchivedSet        bool           `json:"-"`
	IsStarredSet         bool           `json:"-"`
	Name                 *string        `json:"name,omitempty"`
	Avatar               *string        `json:"avatar,omitempty"`
	IsArchived           bool           `json:"isArchived"`
	IsStarred            bool           `json:"isStarred,omitempty"`
	IsCommunity          bool           `json:"isCommunity,omitempty"`
	IsCommunitySet       bool           `json:"-"`
	GroupMetadata        *GroupMetadata `json:"groupMetadata,omitempty"`
	LastMessageID        string         `json:"-"`
	LastMessageText      string         `json:"-"`
	LastMessageTimestamp string         `json:"-"`
	LastMessageSender    string         `json:"-"`
	CanSend              bool           `json:"canSend"`
	IsPinned             bool           `json:"isPinned"`
	IsMuted              bool           `json:"isMuted"`
	IsFavorite           bool           `json:"isFavorite"`
	IsAnnouncement       bool           `json:"isAnnouncement"`
	IsSuspended          bool           `json:"isSuspended"`
}
