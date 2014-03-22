// This file contains constants and arguments used to perform RPCs between
// a TribClient and TribServer. DO NOT MODIFY!

package tribrpc

import "time"

// Status represents the status of a RPC's reply.
type Status int

const (
	OK               Status = iota + 1 // The RPC was a success.
	NoSuchUser                         // The specified UserID does not exist.
	NoSuchTargetUser                   // The specified TargerUserID does not exist.
	Exists                             // The specified UserID or TargerUserID already exists.
)

// Tribble stores the contents and information identifying a unique
// tribble message.
type Tribble struct {
	UserID   string    // The user who created the tribble.
	Posted   time.Time // The exact time the tribble was posted.
	Contents string    // The text/contents of the tribble message.
}

type CreateUserArgs struct {
	UserID string
}

type CreateUserReply struct {
	Status Status
}

type SubscriptionArgs struct {
	UserID       string // The subscribing user.
	TargetUserID string // The user being subscribed to.
}

type SubscriptionReply struct {
	Status Status
}

type PostTribbleArgs struct {
	UserID   string
	Contents string
}

type PostTribbleReply struct {
	Status Status
}

type GetSubscriptionsArgs struct {
	UserID string
}

type GetSubscriptionsReply struct {
	Status  Status
	UserIDs []string
}

type GetTribblesArgs struct {
	UserID string
}

type GetTribblesReply struct {
	Status   Status
	Tribbles []Tribble
}
