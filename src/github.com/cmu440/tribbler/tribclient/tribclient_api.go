// This is the API for a TribClient that we have written for you as
// an example. DO NOT MODIFY!

package tribclient

import "github.com/cmu440/tribbler/rpc/tribrpc"

// TribClient defines the set of methods for one possible Tribbler
// client implementation.
type TribClient interface {
	CreateUser(userID string) (tribrpc.Status, error)
	GetSubscriptions(userID string) ([]string, tribrpc.Status, error)
	AddSubscription(userID, targetUser string) (tribrpc.Status, error)
	RemoveSubscription(userID, targetUser string) (tribrpc.Status, error)
	GetTribbles(userID string) ([]tribrpc.Tribble, tribrpc.Status, error)
	GetTribblesBySubscription(userID string) ([]tribrpc.Tribble, tribrpc.Status, error)
	PostTribble(userID, contents string) (tribrpc.Status, error)
	Close() error
}
