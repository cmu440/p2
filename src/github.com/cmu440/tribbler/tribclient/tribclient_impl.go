// This is the implementation of a TribClient that we have written for you as
// an example. This code also serves as a good reference for understanding
// how RPC works in Go. DO NOT MODIFY!

package tribclient

import (
	"net"
	"net/rpc"
	"strconv"

	"github.com/cmu440/tribbler/rpc/tribrpc"
)

// The TribClient uses an 'rpc.Client' in order to perform RPCs to the
// TribServer. The TribServer must register to receive RPCs and setup
// an HTTP handler to serve the requests. The client may then perform RPCs
// to the TribServer using the rpc.Client's Call method (see the code below).
type tribClient struct {
	client *rpc.Client
}

func NewTribClient(serverHost string, serverPort int) (TribClient, error) {
	cli, err := rpc.DialHTTP("tcp", net.JoinHostPort(serverHost, strconv.Itoa(serverPort)))
	if err != nil {
		return nil, err
	}
	return &tribClient{client: cli}, nil
}

func (tc *tribClient) CreateUser(userID string) (tribrpc.Status, error) {
	args := &tribrpc.CreateUserArgs{UserID: userID}
	var reply tribrpc.CreateUserReply
	if err := tc.client.Call("TribServer.CreateUser", args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *tribClient) GetSubscriptions(userID string) ([]string, tribrpc.Status, error) {
	args := &tribrpc.GetSubscriptionsArgs{UserID: userID}
	var reply tribrpc.GetSubscriptionsReply
	if err := tc.client.Call("TribServer.GetSubscriptions", args, &reply); err != nil {
		return nil, 0, err
	}
	return reply.UserIDs, reply.Status, nil
}

func (tc *tribClient) AddSubscription(userID, targetUserID string) (tribrpc.Status, error) {
	return tc.doSub("TribServer.AddSubscription", userID, targetUserID)
}

func (tc *tribClient) RemoveSubscription(userID, targetUserID string) (tribrpc.Status, error) {
	return tc.doSub("TribServer.RemoveSubscription", userID, targetUserID)
}

func (tc *tribClient) doSub(funcName, userID, targetUserID string) (tribrpc.Status, error) {
	args := &tribrpc.SubscriptionArgs{UserID: userID, TargetUserID: targetUserID}
	var reply tribrpc.SubscriptionReply
	if err := tc.client.Call(funcName, args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *tribClient) GetTribbles(userID string) ([]tribrpc.Tribble, tribrpc.Status, error) {
	return tc.doTrib("TribServer.GetTribbles", userID)
}

func (tc *tribClient) GetTribblesBySubscription(userID string) ([]tribrpc.Tribble, tribrpc.Status, error) {
	return tc.doTrib("TribServer.GetTribblesBySubscription", userID)
}

func (tc *tribClient) doTrib(funcName, userID string) ([]tribrpc.Tribble, tribrpc.Status, error) {
	args := &tribrpc.GetTribblesArgs{UserID: userID}
	var reply tribrpc.GetTribblesReply
	if err := tc.client.Call(funcName, args, &reply); err != nil {
		return nil, 0, err
	}
	return reply.Tribbles, reply.Status, nil
}

func (tc *tribClient) PostTribble(userID, contents string) (tribrpc.Status, error) {
	args := &tribrpc.PostTribbleArgs{UserID: userID, Contents: contents}
	var reply tribrpc.PostTribbleReply
	if err := tc.client.Call("TribServer.PostTribble", args, &reply); err != nil {
		return 0, err
	}
	return reply.Status, nil
}

func (tc *tribClient) Close() error {
	return tc.client.Close()
}
