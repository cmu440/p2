// This file provides a type-safe wrapper that should be used to register
// the TribServer to receive RPCs from the TribClient. DO NOT MODIFY!

package tribrpc

type RemoteTribServer interface {
	CreateUser(args *CreateUserArgs, reply *CreateUserReply) error
	AddSubscription(args *SubscriptionArgs, reply *SubscriptionReply) error
	RemoveSubscription(args *SubscriptionArgs, reply *SubscriptionReply) error
	GetSubscriptions(args *GetSubscriptionsArgs, reply *GetSubscriptionsReply) error
	PostTribble(args *PostTribbleArgs, reply *PostTribbleReply) error
	GetTribbles(args *GetTribblesArgs, reply *GetTribblesReply) error
	GetTribblesBySubscription(args *GetTribblesArgs, reply *GetTribblesReply) error
}

type TribServer struct {
	// Embed all methods into the struct. See the Effective Go section about
	// embedding for more details: golang.org/doc/effective_go.html#embedding
	RemoteTribServer
}

// Wrap wraps t in a type-safe wrapper struct to ensure that only the desired
// TribServer methods are exported to receive RPCs. You should Wrap your TribServer
// before registering it for RPC in your TribServer's NewTribServer function
// like so:
//
//     tribServer := new(tribServer)
//
//     // Create the server socket that will listen for incoming RPCs.
//     listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
//     if err != nil {
//         return nil, err
//     }
//
//     // Wrap the tribServer before registering it for RPC.
//     err = rpc.RegisterName("TribServer", tribrpc.Wrap(tribServer))
//     if err != nil {
//         return nil, err
//     }
//
//     // Setup the HTTP handler that will server incoming RPCs and
//     // serve requests in a background goroutine.
//     rpc.HandleHTTP()
//     go http.Serve(listener, nil)
//
//     return tribServer, nil
func Wrap(t RemoteTribServer) RemoteTribServer {
	return &TribServer{t}
}
